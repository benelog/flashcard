# Flashcard

영어 단어·문장·숙어·개념을 카드로 뒤집으며 외우는 학습 앱과, 그 앱으로 웹 개발을 설명하는 책 『이해하며 만드는 나만의 웹 앱』의 저장소.

## 저장소 구성

| 디렉터리 | 내용 |
|---|---|
| `book/` | 책 원고(AsciiDoc)와 빌드 설정 |
| `book-template/` | 책 빌드 엔진(재사용 가능한 npm 패키지) |
| `flashcard-advanced/` | Vercel + Supabase에 배포하는 완성본 앱(책 3~5부가 인용) |
| `flashcard-basic/` | 책 2부가 인용하는 최소 플래시카드 앱(Gin + htmx + SQLite, 테이블 둘) |
| `language-basic/` | 2부 문법 입문 장의 연습용 예제(HTML·CSS·Go·SQL) |

완성본 앱의 소개·실행·배포는 [flashcard-advanced/README.md](./flashcard-advanced/README.md)에,
최소 앱은 [flashcard-basic/README.md](./flashcard-basic/README.md)에,
연습용 예제는 [language-basic/README.md](./language-basic/README.md)에 있다.

## 라이선스

디렉터리에 따라 다르다.

| 대상 | 라이선스 |
|---|---|
| `book/`의 책 원고 | [CC BY-NC-SA 4.0](https://creativecommons.org/licenses/by-nc-sa/4.0/deed.ko) ([전문](./book/LICENSE)) |
| 그 밖의 모든 코드(`book-template/`, `flashcard-advanced/`, `flashcard-basic/`, `language-basic/`, 루트 스크립트) | [MIT](./LICENSE) |

책 원고는 저작자를 표시하면 자유롭게 공유·변형할 수 있지만, 원본과 그 파생물 모두 상업적으로 이용할 수 없고 파생물은 같은 조건으로 배포해야 한다.
코드는 상업적 이용을 포함해 자유롭게 쓸 수 있다.

© 2026 Sanghyuk Jung

## 책 보기

배포본은 https://benelog.github.io/flashcard/ 에서 읽을 수 있다.

로컬에서 원고를 보려면 `./run_book.sh`로 http://localhost:5173/flashcard/ 를 띄운다(npm 의존성은 처음 한 번 자동 설치).
`./run_book.sh build`는 인용과 링크까지 검증하는 전체 빌드, `./run_book.sh preview`는 그 결과를 그대로 보여 준다.
