# flashcard-cli

`flashcard-advanced` 서버의 JSON API(`/api/*`)를 부르는 CLI. 명령 구조는
cobra, 학습 화면은 Bubble Tea + lipgloss다. 서버 모듈을 import하지 않고
JSON 계약만 본다(별개의 Go 모듈).

## 실행

```sh
go run . decks              # 덱 목록
go run . cards <덱-slug>    # 카드 목록
go run . due                # 오늘 복습할 카드 수
go run . study              # 오늘 복습(due) 세션을 TUI로
go run . study <덱-slug>    # 한 덱 전체 학습
```

학습 화면 조작: `space` 뒤집기 · `o` 맞혔다 · `x` 틀렸다 · `q` 그만.
채점은 서버의 SRS에 그대로 기록된다.

## 서버 지정

- 기본값은 로컬 서버 `http://localhost:8080`(`../flashcard-advanced/run_local.sh`, 인증 없음).
- 다른 서버는 `--server` 또는 `FLASHCARD_SERVER`.
- dev/production처럼 인증 있는 서버는 `--token` 또는 `FLASHCARD_TOKEN`에
  Supabase 액세스 토큰을 넣는다. (로그인이 GitHub/Google OAuth뿐이라 CLI용
  토큰 발급 흐름은 아직 없다. 당장은 브라우저 세션에서 꺼낸 액세스 토큰을 쓴다.)

## 검증

```sh
go build ./... && go vet ./... && go test ./...
```
