# Flashcard 완성본 앱(flashcard-advanced) 지침

Vercel + Supabase에 배포하는 완성본 앱이다. 아래 경로는 모두 이 디렉터리(`flashcard-advanced/`) 기준이다.
저장소 공통 규칙(브랜치 정책, 두 모듈 검증, 책 인용 태그)은 루트 `CLAUDE.md`에 있다.

## 구조

- UI는 Go 서버가 렌더링한다: `internal/web` (html/template + htmx + 순수 CSS, 전부 바이너리에 embed). 화면 하나가 파일 하나다(`home.go`, `decks.go`, `cards.go`, …). 화면들이 공통으로 쓰는 것은 역할별 일곱 파일로 나뉜다: `web.go`(embed 자산·`Web` 타입·요청 헬퍼), `render.go`(템플릿 파싱과 그리기), `routes.go`(라우트 등록과 정적 자원), `funcs.go`(템플릿 함수), `cookies.go`(쿠키·세션 미들웨어), `gotrue.go`(Supabase Auth 클라이언트), `icons.go`(인라인 SVG).
- `web`과 `handlers`는 서로 import하지 않는다. 둘 다 쓰는 요청 미들웨어(프로필 보장 `EnsureProfile`)는 `internal/auth`에 두고, 실패를 오류 화면으로 그릴지 JSON으로 답할지만 각자 끼운다.
- 브라우저 JS는 `internal/web/static/app.js` 하나뿐(TTS·클립보드·오프라인·서비스 워커). 프런트엔드 빌드 도구(npm 등)는 앱에 없다.
- JSON API(`/api/*`)는 `internal/handlers`에 있다. HTML과 API가 같은 Gin 엔진(`pkg/app`)에 물린다.
- **`internal/model`이 앱의 공용어다.** 행 타입(`Card`, `Deck`, …), `ErrNotFound`, 열이 받는 값(카드 종류·학습 방향·모드)과 그 판정, DB를 모르는 순수 함수(`Streak`, 슬러그, `NilIfBlank`), 그리고 저장소 계약인 `Store` 인터페이스가 여기 있다.
  - 저장소 구현은 둘이고 둘 다 `model.Store`를 만족한다: `internal/pgstore`(pgx, 배포)와 `internal/litestore`(SQLite, 로컬). 어느 쪽도 상대를 모른다.
  - **두 구현은 대칭으로 둔다.** 같은 일을 하는 것은 같은 파일·같은 이름에 놓는다(`rules.go`의 `ruleQuery`, `collect`, `scan…`, `requireRowAffected`). 한쪽만 고치면 다른 쪽도 함께 고친다.
  - `internal/smartrules`는 규칙의 모양과 유효성만 안다. 규칙을 SQL로 옮기는 일은 방언이 달라 각 저장소의 `rules.go`가 맡는다.
  - `web`·`handlers`·`cardcsv`·`study`는 `model`만 보므로 pgx를 링크하지 않는다. 새 코드에서 이 방향을 뒤집지 않는다(구현 패키지를 화면이나 핸들러에서 직접 import하지 않는다).
- **`internal/study`는 화면과 API가 공유하는 학습 정책이다.** 어느 카드를 어떤 순서로 낼지(`Pick`), 어떤 추천 타일을 보일지(`Suggestions`)를 여기 한 곳에서 정한다. 잘못된 요청은 sentinel 에러로만 돌려주고, 그것을 404 화면으로 옮길지 400 JSON으로 옮길지는 부르는 쪽이 정한다. 학습 규칙을 바꿀 때 `web`과 `handlers`를 각각 고치지 않도록 이 방향을 지킨다.
- `pkg/app`만 `internal/`이 아니다. Vercel의 Go 빌더가 `api/index.go`를 모듈 바깥에서 컴파일해 `internal/`을 가져올 수 없기 때문이다. 옮기면 로컬은 통과하고 배포만 깨진다.

## 테스트

- 순수 로직(`srs`, `smartrules`, `cardcsv`, `model`)은 각 패키지에서 단위 테스트한다. DB에 붙지 않는 것은 `pgstore`도 마찬가지다(`rules_test.go`는 만들어진 SQL 문자열만 본다).
- **저장소를 갈아 끼울 때 전부 구현할 필요는 없다.** 소비자가 쓰는 부분집합만 소비자 쪽 인터페이스로 정의했거나(`study.Store`, `cardcsv`의 `deckSource`, `auth`의 `profileStore`), `model.Store`를 묻은 구조체로 그 테스트가 쓰는 메서드만 채우면 된다(`pkg/app`의 `brokenStore`: 한 메서드만 실패시켜 500 경로를 확인).
- 화면과 API는 `pkg/app`에서 **앱을 통째로 띄워 실제 HTTP 요청을 보내** 확인한다(임시 SQLite + 고정 사용자). 라우팅·미들웨어·템플릿·저장소가 이어져 있는지까지 한 번에 잡힌다.
- 그래서 `go test -cover`의 `handlers`·`web` 수치는 낮게 나온다. 커버리지는 자기 패키지의 테스트가 실행한 줄만 세기 때문이지, 검사되지 않는다는 뜻이 아니다.

## 스타일(CSS)

- **자손 선택자 대신 클래스를 쓴다.** `.bottomnav a`처럼 태그에 기대면 템플릿에서 그 태그를 바꾸거나 래퍼를 하나 더 감쌀 때 스타일이 풀리는데, 컴파일 오류도 테스트 실패도 나지 않아 알아채기 어렵다. 요소마다 이름표를 달아 `.navlink`처럼 평평하게 고른다.
- 예외는 **상태 선택자**다. `:checked ~`(카드 뒤집기), `:checked +`(알약 라디오), `details.reveal[open] > summary`, `[data-done] .copy-*`는 자바스크립트 없이 상태를 자식·형제에 전달하는 메커니즘 자체라 클래스로 대체할 수 없다. 이런 선택자를 새로 둘 때는 왜 평탄화하지 않는지 주석에 남긴다.
- 템플릿에 인라인 `style` 속성을 두지 않는다. 서버가 계산한 값을 넣는 경우(`style="width:{{.ProgressPct}}%"`)만 예외다.

## 환경

DB는 환경마다 완전히 분리되어 있다.

| 환경 | 배포 | DB | 로그인 |
|---|---|---|---|
| local | `./run_local.sh` | SQLite (`local-db/flashcard.db`) | 없음(고정 사용자) |
| dev | main 푸시 → Vercel **Preview** | 개발용 Supabase 프로젝트 | GitHub/Google |
| production | release 병합 → Vercel **Production** | 운영 Supabase 프로젝트 | GitHub/Google |

- `./run_dev.sh`: local에서 **dev DB**에 붙어 서버를 띄운다(GitHub/Google 로그인 포함). 값은 `.env.dev`에서 읽는다(`.env.dev.example` 참고).
- 개발 환경 값의 단일 출처는 `.env.dev` 하나다. 이 파일을 읽는 곳은 `run_dev.sh`와 `migrate_dev.sh` 둘뿐이고, 값을 셸에 상주시키는 도구(direnv 등)는 쓰지 않는다. 일회성 명령에 값이 필요하면 그 셸에서 `set -a; source .env.dev; set +a`로 읽어 온다. **운영 값은 로컬에 두지 않는다**(운영 반영은 release 병합 시 GitHub Actions가 한다).
- 환경 구분은 `internal/config`가 `DATABASE_URL` 유무로 한다(있으면 postgres+supabase, 없으면 sqlite+local). 그래서 `run_local.sh`는 `env -u DATABASE_URL`로 실행한다. 어떤 이유로든 dev 값이 셸에 올라와 있어도 로컬 모드가 SQLite로 뜨게 하기 위함이다.

## 스키마 관리

- Postgres 스키마의 단일 소스는 `internal/db/migrations/*.up.sql`(golang-migrate, 바이너리에 embed). 새 변경은 항상 새 번호의 up/down 쌍을 추가한다. 기존 파일은 고치지 않는다.
- 적용은 `cmd/migrate`가 한다. `.github/workflows/migrate.yml`이 **스키마 SQL이 바뀐 푸시에만** 자동 실행한다: main → dev DB, release → 운영 DB.
- local에서 dev DB에 미리 적용해 보려면 `./migrate_dev.sh`. 운영 DB에는 수동으로 적용하지 않는다.
- 마이그레이션과 Vercel 배포는 서로를 기다리지 않는다. 칼럼 삭제·이름 변경은 배포 순서에 상관없이 안전하도록 세 단계(추가 → 코드 전환 → 제거)로 나눈다.
- SQLite(`internal/litestore/schema.sql`)는 위 마이그레이션을 손으로 옮긴 포팅본이다. Postgres 마이그레이션을 추가하면 **같은 커밋에서 이 파일도 함께 고쳐야** 두 환경이 어긋나지 않는다.
  - 잊으면 `internal/litestore/schema_test.go`가 잡는다(새 테이블·새 뷰·새 열, 그리고 파일 첫머리의 `Ported through:` 표식을 대조한다). 옮긴 뒤 그 표식도 최신 마이그레이션 이름으로 고친다.

## PWA와 캐시

- `internal/web/static/sw.js`의 캐시 저장소는 두 개다. `flashcard-pages-*`는 사용자별로 렌더링된 HTML(서버가 `no-store`를 붙이지만 Cache Storage에는 통하지 않는다), `flashcard-static-*`는 사용자와 무관한 자원이다. 개인 화면은 `PAGES`에만 담고, 로그아웃(`/login?signed_out=1` → `app.js`)에서 그 상자만 지운다.
- `/api/` 응답과 GET이 아닌 요청은 캐시하지 않는다(network only).
- 캐시 전략을 바꾸면 두 캐시 이름의 버전(`-v3`)을 함께 올린다. 올리지 않으면 옛 정책으로 채워진 항목이 사용자 기기에 남는다.
- 정적 자원은 파일명에 해시가 없다. 템플릿에서 반드시 `{{asset "/static/…"}}`로 참조해 `?v=<내용 해시>`가 붙게 한다(`internal/web/routes.go`의 `assetVersion`). 새 자산 파일을 추가하면 해시 대상 목록에도 넣는다.

## 검증

- 이 디렉터리에서 `go build ./... && go vet ./... && go test ./...` (gofmt는 훅이 자동 적용)
- 책이 인용하는 코드를 건드렸으면 `cd ../book && npm run build`까지 돌린다(인용이 깨졌는지 여기서 잡힌다).
