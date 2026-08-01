# Flashcard 저장소 지침

이 저장소는 책 『이해하며 만드는 나만의 웹 앱』의 원고와, 책이 인용하는 세 갈래의 코드가 함께 산다.

## 저장소 구성

- `book/`: 책 원고(AsciiDoc)와 빌드 설정. 집필 규약은 `book/CLAUDE.md`.
- `book-template/`: 책 빌드 엔진(재사용 가능한 npm 패키지, 추후 별도 저장소로 분리 예정).
- `flashcard-advanced/`: Vercel + Supabase에 배포하는 완성본 앱(Go 모듈 `github.com/benelog/flashcard`). 책 1부(첫 실행)와 3~5부·부록이 이 코드를 인용한다. 앱 지침은 `flashcard-advanced/CLAUDE.md`.
- `flashcard-basic/`: 책 2부가 인용하는 최소 플래시카드 앱. Gin + html/template + SQLite, 테이블은 `decks`·`cards` 둘뿐이다. 자바스크립트가 한 줄도 없다(htmx는 완성본에만 있다). 완성본과 별개의 Go 모듈이다.
- `language-basic/`: 2부 문법 입문 장(HTML·CSS·Go·SQL)의 연습용 예제 파일.

원고는 코드를 베끼지 않고 `include::`로 인용한다. 그래서 코드에 `// tag::이름[]` … `// end::이름[]` 주석이 붙어 있는 곳이 있다. 지우거나 옮길 때 주의한다(자세한 규약은 `book/CLAUDE.md`). 태그를 지우거나 이름을 바꾸면 `cd book && npm run build`가 실패한다.

## 브랜치 정책 (GitLab flow 단순화)

- `main`: 개발 브랜치. 모든 작업은 main에서 한다. main 푸시는 Vercel Preview 배포(개발 환경)로 이어진다.
- `release`: 운영 브랜치. 직접 커밋하지 않는다. 운영 배포는 main → release 병합으로만 한다(`flashcard-advanced/release.sh`).
- 두 브랜치 모두 push 전에 아래 검증이 통과해야 한다(CI도 같은 검사를 돌린다).

## 검증

Go 모듈이 둘이므로 각 디렉터리에서 따로 돌린다.

- `cd flashcard-advanced && go build ./... && go vet ./... && go test ./...`
- `cd flashcard-basic && go build ./... && go vet ./... && go test ./...`
- gofmt는 훅이 자동 적용한다(`.claude/hooks/go-check.sh`).
- 책이 인용하는 코드를 건드렸으면 `cd book && npm run build`까지 돌린다(인용이 깨졌는지 여기서 잡힌다).
