# language-basic

책 2부의 문법 입문 장이 쓰는 연습용 예제다.
앱이 아니라 언어 하나를 익히는 파일들이라, 플래시카드 앱 코드(`../flashcard-basic/`, `../flashcard-advanced/`)와 분리해 둔다.

- `html/`: HTML·CSS 장의 예제와 화면 캡처 원본(`book/scripts/capture-examples.mjs`가 찍는다).
- `go/`: Go 장의 완결 실행형 예제. `go/hello`, `go/grade` 각 디렉터리에서 `go run .`으로 실행한다.
- `sql/`: 데이터베이스 기초 장의 연습 문장 모음(`sqlite3 practice.db < practice.sql`).

원고의 코드 블록 중 완결된 예제는 여기 파일을 `include::`로 인용하고, 짧은 조각은 원고에 직접 적혀 있다(집필 규약은 `book/CLAUDE.md`).
