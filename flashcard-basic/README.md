# flashcard-basic

책 2부가 인용하는 최소 플래시카드 앱이다.
완성본(`../flashcard-advanced/`)에서 배포·인증·간격 반복을 걷어 내고, 웹 앱의 뼈대만 남겼다.

- 테이블은 `decks`와 `cards` 둘뿐이다(`schema.sql`).
- Gin 라우팅과 핸들러(`handlers.go`), `database/sql` + SQLite 저장소(`store.go`), html/template 화면(`templates/`), htmx 부분 갱신(카드 추가·삭제, 덱 삭제)으로 이루어진다. 덱 이름 바꾸기는 일반 폼과 리다이렉트로 처리한다.
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
