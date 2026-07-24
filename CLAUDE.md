# Flashcard 프로젝트 지침

## 브랜치 정책 (GitLab flow 단순화)

- `main`: 개발 브랜치. 모든 작업은 main에서 한다. main 푸시는 Vercel Preview 배포(개발 환경)로 이어진다.
- `release`: 운영 브랜치. 직접 커밋하지 않는다. 운영 배포는 main → release 병합으로만 한다.
- 두 브랜치 모두 push 전에 아래 검증이 통과해야 한다(CI도 같은 검사를 돌린다).

## 구조

- UI는 Go 서버가 렌더링한다: `internal/web` (html/template + htmx + 순수 CSS, 전부 바이너리에 embed).
- 브라우저 JS는 `internal/web/static/app.js` 하나뿐(TTS·클립보드·오프라인·서비스 워커). 프런트엔드 빌드 도구(npm 등)는 앱에 없다. `doc/`(책 원고, AsciiDoc)와 `book-template/`(책 빌드 엔진, 재사용 가능한 npm 패키지)만 자체 package.json을 가진다.
- JSON API(`/api/*`)는 `internal/handlers`에 그대로 있다. HTML과 API가 같은 Gin 엔진(`pkg/app`)에 물린다.

## 책이 인용하는 코드

`doc/`의 원고는 이 저장소의 코드를 베끼지 않고 `include::`로 인용한다(규약은 `doc/CLAUDE.md`).
그래서 코드에 `// tag::이름[]` … `// end::이름[]` 주석이 붙어 있는 곳이 있다. 지우거나 옮길 때 주의한다.

- 코드를 고쳐도 원고는 자동으로 따라온다. 대신 **태그를 지우거나 이름을 바꾸면** `cd doc && npm run build`가 실패한다. 인용 위치를 옮겼으면 마커도 함께 옮긴다.
- 함수·필드 이름을 바꾸면 원고 본문의 설명 문장은 자동으로 바뀌지 않는다. `grep`으로 옛 이름을 훑어 함께 고친다.
- 마커는 구조체 필드 사이에 넣지 않는다. gofmt가 주석을 경계로 정렬 그룹을 끊어 필드 정렬이 깨진다.

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
- 마이그레이션과 Vercel 배포는 서로를 기다리지 않는다. 컬럼 삭제·이름 변경은 배포 순서에 상관없이 안전하도록 세 단계(추가 → 코드 전환 → 제거)로 나눈다.
- SQLite(`internal/litestore/schema.sql`)는 위 마이그레이션을 손으로 옮긴 포팅본이다. Postgres 마이그레이션을 추가하면 **같은 커밋에서 이 파일도 함께 고쳐야** 두 환경이 어긋나지 않는다.

## PWA와 캐시

- `internal/web/static/sw.js`의 캐시 저장소는 두 개다. `flashcard-pages-*`는 사용자별로 렌더링된 HTML(서버가 `no-store`를 붙이지만 Cache Storage에는 통하지 않는다), `flashcard-static-*`는 사용자와 무관한 자원이다. 개인 화면은 `PAGES`에만 담고, 로그아웃(`/login?signed_out=1` → `app.js`)에서 그 상자만 지운다.
- `/api/` 응답과 GET이 아닌 요청은 캐시하지 않는다(network only).
- 캐시 전략을 바꾸면 두 캐시 이름의 버전(`-v3`)을 함께 올린다. 올리지 않으면 옛 정책으로 채워진 항목이 사용자 기기에 남는다.
- 정적 자원은 파일명에 해시가 없다. 템플릿에서 반드시 `{{asset "/static/…"}}`로 참조해 `?v=<내용 해시>`가 붙게 한다(`internal/web/web.go`의 `assetVersion`). 새 자산 파일을 추가하면 해시 대상 목록에도 넣는다.

## 검증

- `go build ./... && go vet ./... && go test ./...` (gofmt는 훅이 자동 적용)
- 책이 인용하는 코드를 건드렸으면 `cd doc && npm run build`까지 돌린다(인용이 깨졌는지 여기서 잡힌다).
