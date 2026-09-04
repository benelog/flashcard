# flashcard-cli

`flashcard-advanced` 서버의 JSON API(`/api/*`)를 부르는 CLI. 명령 구조는
cobra, 화면은 Bubble Tea + lipgloss다. 서버 모듈을 import하지 않고
JSON 계약만 본다(별개의 Go 모듈).

같은 기능을 세 가지 방식으로 쓴다.

## 1. 메뉴 모드 (옵션 없이 실행하면 이 모드)

```sh
go run .          # 또는 go run . menu
```

Turbo C처럼 위쪽 메뉴 바에서 풀다운 메뉴를 펴고 고른다.

- `←`/`→` 메뉴 옮기기, `↑`/`↓` 항목 옮기기(닫혀 있으면 `↓`로 편다)
- `enter` 고르기, `esc` 펼친 메뉴 닫기, `q` 끝내기
- 메뉴는 **프로그램**(서버 확인·GitHub/Google로 로그인·로그아웃·조작 안내·
  끝내기), **덱**(덱 목록·카드 보기·오늘 복습할 카드 수), **학습**(오늘 복습·덱
  골라 학습, 각각 뜻→표현 방향도)
- 덱을 골라야 하는 항목은 덱 목록을 띄워 `↑`/`↓`+`enter`로 고른다.
- 학습을 고르면 메뉴를 닫고 학습 화면으로 갔다가, 끝나면 결과를 안고 메뉴로
  돌아온다.

파이프로 실행하는 등 터미널이 아니면 메뉴 대신 사용법을 보여 준다.

## 2. 명령 모드

```sh
go run . login              # 브라우저로 로그인(--provider google도 된다)
go run . logout             # 저장된 로그인을 지운다
go run . decks              # 덱 목록
go run . cards <덱-slug>    # 카드 목록
go run . due                # 오늘 복습할 카드 수
go run . study              # 오늘 복습(due) 세션을 학습 화면으로
go run . study <덱-slug>    # 한 덱 전체 학습
```

`study`는 `--reverse`(뜻을 보고 표현을 떠올린다)와 `--limit`을 받는다.

## 3. 셸 모드

```sh
go run . shell
```

프롬프트에 명령을 한 줄씩 친다. `help`로 명령 목록, `exit`(또는 `quit`,
`Ctrl-D`)로 나간다. 셸에 들어올 때의 `--server`·`--token`을 이어 쓰고,
한 줄에서 준 플래그는 그 줄에만 적용된다.

```
flashcard> decks
flashcard> study jp-n3 --reverse
flashcard> exit
```

## 학습 화면

`space` 뒤집기 · `o` 맞혔다 · `x` 틀렸다 · `q` 그만.
채점은 서버의 SRS에 그대로 기록된다.

## 로그인

운영 서버는 로그인이 필요하다. `login` 명령이나 메뉴의 **프로그램 ▸ GitHub로
로그인**을 고르면 브라우저가 열리고, GitHub/Google 로그인을 마치면 브라우저가
`http://localhost:45678/callback`으로 돌아와 CLI에 code를 건넨다. CLI는 그
code를 서버의 `/api/auth/exchange`로 보내 토큰을 받는다(Supabase anon key는
서버에만 있다). 흐름은 서버의 웹 로그인과 같은 OAuth PKCE다.

- 토큰은 `~/.config/flashcard/credentials.json`(`os.UserConfigDir()` 기준,
  `FLASHCARD_CONFIG_DIR`로 바꿀 수 있다)에 서버 주소별로 저장되고, 만료 1분
  전부터 리프레시 토큰으로 자동 갱신한다.
- `logout`은 저장된 토큰을 지운다.
- Supabase 대시보드의 **Authentication ▸ URL Configuration ▸ Redirect URLs**에
  `http://localhost:45678/callback`이 등록돼 있어야 한다. 없으면 GoTrue가
  localhost로 돌아오지 않고 Site URL(`https://flashcard.benelog.net/?code=…`)로
  가 버려 CLI가 5분 동안 기다리다 끝난다. 그동안 다시 `login`을 치면 포트가
  잡혀 있다는 오류가 난다.
- 브라우저는 CLI와 같은 컴퓨터에서 열려야 한다(콜백이 localhost다). 원격
  셸에서는 `--token`을 쓴다.

## 서버 지정

- 기본값은 운영 서버 `https://flashcard.benelog.net`.
- 다른 서버는 `--server` 또는 `FLASHCARD_SERVER`. 로컬 서버는
  `--server http://localhost:8080`(`../flashcard-advanced/run_local.sh`, 인증
  없음. 로그인 명령은 "로그인이 없다"고 알린다).
- `--token` 또는 `FLASHCARD_TOKEN`에 Supabase 액세스 토큰을 직접 주면 저장된
  로그인보다 먼저 쓴다.

## 검증

```sh
go build ./... && go vet ./... && go test ./...
```
