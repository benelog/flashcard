# Flashcard

영어 단어·문장·숙어·개념을 카드로 뒤집으며 외우는 학습 앱.

- **UI**: Go 서버가 렌더링하는 HTML(`html/template`) + [htmx](https://htmx.org) 부분 갱신 + 순수 CSS — 프런트엔드 빌드 도구가 없다
- **백엔드**: Go + Gin — Vercel 서버리스 함수(`api/index.go`) 하나가 페이지·정적 파일·JSON API를 모두 서빙, 로컬은 `cmd/server`
- **DB / 인증**: Supabase (PostgreSQL + Google/GitHub OAuth). OAuth는 서버 사이드 PKCE로 처리하고 세션은 HttpOnly 쿠키 — 브라우저 JS가 토큰을 만질 일이 없다
- **호스팅**: Vercel 무료 티어 하나로 전부 배포. PWA로 웹과 Android 모두 커버
- 남은 자바스크립트는 `internal/web/static/app.js` 하나: 음성(TTS)·클립보드·오프라인 감지·서비스 워커 등록 등 브라우저 전용 API만 감싼다

## 기능

- 양방향 카드(원문 text: 영어·용어 / 뜻 meaning: 설명) + 덱 관리, 태그
- 학습: 방향 선택(원문→뜻 / 뜻→원문) → 카드 뒤집기(CSS만으로 동작) → 스스로 맞음/틀림 판정 → 덱 소화 후 틀린 카드 재도전 라운드
- 간격 반복(SRS, SM-2 변형): 매일 "오늘 복습" 큐 자동 구성
- 규칙 기반 스마트 덱: 오답률 높은 카드·오래 안 본 카드 등을 홈 화면에서 추천
- 학습 통계: 일별 학습량, 정답률, 연속 학습일, 덱별 성취도
- 덱 공유: 공유 링크 + 공유 덱 갤러리에서 미리보고 "내 덱으로 가져오기"(카드 복사, 학습 기록은 새로 시작)
- CSV 가져오기/내보내기 (`text,meaning,type,tags,phonetic,example`, 태그는 `|` 구분; 가져오기는 구 `front,back` 헤더도 인식)
- 무료 사전 API로 발음기호·뜻·예문 자동 채우기 (서버가 조회해 htmx로 폼에 채움)
- 음성(TTS) 버튼: Web Speech API로 영어 읽어주기

## 환경

local, dev, production 세 환경이 있다. 브랜치가 배포 환경을 정한다: `main` 푸시 → dev(Vercel Preview), `release` 푸시(= `./release.sh`의 main → release 병합) → production.

| 환경 | URL | DB / 인증 | 배포 계기 |
|---|---|---|---|
| **local** | http://localhost:8080 | SQLite 파일(`local-db/flashcard.db`), 로그인 없음 | `./run_local.sh` |
| **dev** (개발) | https://flashcard-dev.vercel.app | 개발용 Supabase 프로젝트 | `main` 푸시 |
| **production** (운영) | https://flashcard.benelog.net | 운영용 Supabase 프로젝트 | `release` 푸시 |

dev URL은 main 브랜치의 최신 Preview 배포를 가리키는 고정 별칭이다(배포마다 새로 생기는 고유 URL과 별개).
환경 변수는 Vercel의 Production/Preview 스코프에 따로 등록한다. 상세 배선은 [DEPLOY.md](./DEPLOY.md) 참고.

## 로컬 개발

```bash
./run_local.sh   # SQLite, 환경 변수 없음, 로그인 없음
./run_dev.sh     # 개발 Supabase DB + GitHub/Google 로그인 (.env.dev 필요)
./run_book.sh    # 책(doc/) 이북 뷰어 개발 서버
```

`run_local.sh`는 환경 변수 없이 http://localhost:8080 이 뜬다 (SQLite 로컬 모드).
개발 DB에 붙어 로그인까지 시험하려면 `.env.dev.example`을 `.env.dev`로 복사해 채우고 `run_dev.sh`를 쓴다.
스키마를 개발 DB에 적용하는 `./migrate_dev.sh`도 같은 파일을 읽는다. 상세는 [DEPLOY.md](./DEPLOY.md) 참고.

책 원고를 보려면 `./run_book.sh`로 http://localhost:5173/flashcard/ 를 띄운다(npm 의존성은 처음 한 번 자동 설치).
`./run_book.sh build`는 인용과 링크까지 검증하는 전체 빌드, `./run_book.sh preview`는 그 결과를 그대로 보여 준다.

## 테스트

```bash
go test ./...
```

두 층으로 나뉜다. SRS 알고리즘·스마트 덱 규칙·CSV 매핑·도메인 판정처럼 DB도 HTTP도 모르는 코드는 각 패키지에서 단위 테스트하고,
화면과 JSON API는 `pkg/app`에서 앱을 통째로 띄워 실제 HTTP 요청을 보내 확인한다(임시 SQLite + 고정 사용자라 준비물이 없다).
