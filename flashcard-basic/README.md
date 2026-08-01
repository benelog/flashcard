# flashcard-basic

책 2부가 인용하는 최소 플래시카드 앱이다.
완성본(`../flashcard-advanced/`)에서 배포·인증·간격 반복을 걷어 내고, 웹 앱의 뼈대만 남겼다.

- 테이블은 `decks`와 `cards` 둘뿐이다(`schema.sql`).
- Gin 라우팅과 핸들러(`handlers.go`), `database/sql` + SQLite 저장소(`store.go`), html/template 화면(`templates/`)으로 이루어진다. 자바스크립트는 한 줄도 쓰지 않는다. 화면 전환은 링크와 폼 제출, 그리고 리다이렉트로만 처리하고(PRG), 카드 뒤집기는 브라우저 내장 `details`/`summary`가 맡는다.
- 로그인이 없고 데이터는 현재 디렉터리의 `flashcard.db` 파일 하나에 저장된다.

실행:

```bash
go run .
```

http://localhost:8080 에서 열린다.

테스트:

```bash
go test ./...
```
