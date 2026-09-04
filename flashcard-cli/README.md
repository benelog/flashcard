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
- 메뉴는 **프로그램**(서버 확인·조작 안내·끝내기), **덱**(덱 목록·카드 보기·오늘
  복습할 카드 수), **학습**(오늘 복습·덱 골라 학습, 각각 뜻→표현 방향도)
- 덱을 골라야 하는 항목은 덱 목록을 띄워 `↑`/`↓`+`enter`로 고른다.
- 학습을 고르면 메뉴를 닫고 학습 화면으로 갔다가, 끝나면 결과를 안고 메뉴로
  돌아온다.

파이프로 실행하는 등 터미널이 아니면 메뉴 대신 사용법을 보여 준다.

## 2. 명령 모드

```sh
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

## 서버 지정

- 기본값은 운영 서버 `https://flashcard.benelog.net`.
- 다른 서버는 `--server` 또는 `FLASHCARD_SERVER`. 로컬 서버는
  `--server http://localhost:8080`(`../flashcard-advanced/run_local.sh`, 인증 없음).
- 운영 서버처럼 인증 있는 서버는 `--token` 또는 `FLASHCARD_TOKEN`에
  Supabase 액세스 토큰을 넣는다. (로그인이 GitHub/Google OAuth뿐이라 CLI용
  토큰 발급 흐름은 아직 없다. 당장은 브라우저 세션에서 꺼낸 액세스 토큰을 쓴다.)

## 검증

```sh
go build ./... && go vet ./... && go test ./...
```
