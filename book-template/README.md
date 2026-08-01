# book-template

AsciiDoc 원고를 이북 뷰어 웹사이트와 PDF 한 권으로 배포하는 책 빌드 엔진이다.
원고(.adoc)를 downdoc으로 마크다운으로 변환해 VitePress로 빌드하고, 그 위에 종이책 느낌의 뷰어(페이지 넘김·책장 넘김 애니메이션·북마크·형광펜·읽기 진행 바)를 얹는다.

책 저장소는 원고와 설정만 가지며, 이 패키지를 npm 의존성으로 쓴다.
첫 사용처는 [flashcard 책](../book/)이다.

## 구성

```
bin/book.mjs        CLI: dev | build | preview | pdf | og
lib/config.mjs      defineBookConfig(book): book.config → VitePress 설정
lib/generate.mjs    원고(.adoc) → .generated/*.md + 홈(index.md) 생성
lib/include.mjs     include:: 지시자 해석(저장소 코드를 베끼지 않고 인용)
lib/adoc.mjs        downdoc 변환 파이프라인(보호 → 변환 → 복원 → 검증)
lib/cover.mjs       표지 단일 소스(홈 랜딩·PDF 표지·PDF 차례)
lib/pdf.mjs         빌드 결과를 장 순서대로 인쇄해 한 권으로 병합(아웃라인·쪽 번호)
lib/og.mjs          홈 표지에서 Open Graph 이미지(1200×630) 생성
theme/              VitePress 테마(이북 뷰어 Layout.vue, custom.css, ebook.js)
tools/              md2adoc.mjs(마크다운 원고 일회성 이행), verify-roundtrip.mjs(이행 검증)
templates/          새 저장소용 참고 파일(GitHub Actions 워크플로, postcss 방화벽)
```

## 새 책 만들기

1. 책 디렉터리를 만들고 이 패키지를 의존성으로 추가한다.

   ```jsonc
   // package.json
   {
     "type": "module",
     "scripts": { "dev": "book dev", "build": "book build", "pdf": "book pdf", "og": "book og" },
     "devDependencies": { "book-template": "file:../book-template" }
   }
   ```

   `.npmrc`에 `install-links=false`를 넣어 심링크 방식으로 고정한다.
   이 패키지는 자체 lockfile로 의존성을 갖고 있으므로 양쪽 모두 `npm install`을 한 번씩 한다.

2. `book.config.mjs`를 작성한다. 책의 모든 메타데이터와 목차의 단일 소스다.

   ```js
   export default {
     lang: 'ko-KR',
     title: '책 제목',
     subtitle: '부제',
     description: '검색·SNS 미리보기에 쓰일 설명',
     site: 'https://user.github.io/repo/',   // 끝에 /
     base: '/repo/',                          // 끝에 /
     repo: 'https://github.com/user/repo',
     author: '지은이',
     siteLabel: 'user.github.io/repo',        // PDF 표지 하단 표기
     pdf: { fileName: 'my-book.pdf' },
     storage: { prefix: 'mybook' },           // 북마크·형광펜 localStorage 키 접두사
     cover: {
       kicker: '시리즈 라벨',
       volume: '01',
       titleHtml: '<strong>강조</strong>가 든<br>제목',   // <strong>이 형광 강조가 된다
       subtitleHtml: '줄바꿈이 든<br>부제',
       diagram: [{ name: '층 이름', tech: '기술' }],       // 생략 가능
       pitch: ['특징 한 줄', '특징 한 줄'],                 // 생략 가능
       homeDesc: '홈 하단 설명 문단',
       actions: [
         { text: '읽기 시작', link: 'start', brand: true }, // start|pdf|repo|URL
         { text: 'PDF 다운로드', link: 'pdf' },
       ],
     },
     toc: [
       {
         text: '1부 제목',
         items: [
           { file: 'part1/chapter.adoc', text: '1장 제목' },
           // pdfTitle·pdfPart로 PDF 차례의 제목·부 라벨을 따로 정할 수 있다
         ],
       },
     ],
   }
   ```

   사이드바·nav·OG 메타·홈 표지·PDF 표지·차례·아웃라인이 전부 여기서 파생된다.
   toc의 그룹 첫 항목 앞에는 PDF 차례에 부 제목이 들어간다.

3. `.vitepress/`에 심 두 개를 둔다.

   ```ts
   // .vitepress/config.ts
   import { defineBookConfig } from 'book-template/config'
   import book from '../book.config.mjs'
   export default defineBookConfig(book)
   ```

   ```ts
   // .vitepress/theme/index.ts
   export { default } from 'book-template/theme'
   ```

4. 원고(.adoc)를 쓰고 `npm run dev`로 확인한다. `.generated/`(변환 산출물)는 gitignore에 넣는다.
   정적 파일(이미지 등)은 `public/`에 두면 사이트 루트로 복사된다.
5. 배포는 `templates/book.yml`을 참고해 GitHub Actions를 구성한다.
   PDF·OG 생성은 시스템 Chrome을 쓴다(`PUPPETEER_EXECUTABLE_PATH`로 지정 가능).

## 원고 규약 (downdoc 부분집합)

이 엔진은 asciidoctor가 아니라 downdoc으로 변환하므로, AsciiDoc 전체가 아니라 아래 부분집합을 전제한다.
규약 위반은 `book build`가 파일:줄 번호와 함께 에러로 알려 준다.

- 파일 첫 줄은 `= 장 제목` 하나. 절은 `==`, 소절은 `===`.
- 굵게는 `*별표 한 겹*`. 이중 별표는 금지.
- 목록은 `*`(비순서)와 `.`(순서). 표는 `|===`, 헤더 한 줄 뒤 빈 줄.
- 코드 블록은 `[source,언어]` + `----`.
- 용어 상자 같은 강조 상자는 블록 제목 + admonition으로 쓴다. NOTE→info, TIP→tip, WARNING/IMPORTANT→warning, CAUTION→danger 컨테이너로 변환된다.

  ```
  .상자 제목
  [NOTE]
  ====
  본문.
  ====
  ```

- 외부 링크는 `https://주소[표시 문구]` 매크로(맨몸 URL은 링크가 되지 않는다).
- 원본 HTML이 필요한 자리(그림 갤러리 등)는 `++++` 패스스루 블록으로 감싼다. 내용이 그대로 VitePress 마크다운에 들어가므로 그 안에서는 마크다운 이미지 문법도 쓸 수 있다.
- 백틱이 든 인라인 코드는 마크다운식 이중 백틱으로 감싼다.
- 인라인 코드의 중괄호(`{{.Title}}`)는 일반 백틱에 그대로. 따옴표·앰퍼샌드·코드 속 별표는 파이프라인이 자동 보호한다.
- 금지: `+…+` 패스스루, 문서 속성 정의(`:이름:`), downdoc 내장 속성 참조(`{sp}` 등), 제어 문자.

### 장 참조 토큰 ({ch-…})

본문에서 다른 장을 번호로 가리키면 장을 재배열할 때마다 원고 전체를 훑어야 한다.
대신 `{ch-파일명}` 토큰을 쓰면 빌드가 toc 항목 제목의 선두 라벨("15장", "부록")을 읽어 치환한다.
장을 재배열할 때 고칠 곳이 `book.config.mjs`의 toc 하나로 줄고, 본문 번호가 toc와 어긋날 일도 없다.

- `{ch-gin}` → "15장" (파일 이름은 확장자 없이, 디렉터리 무관)
- `{ch-go-hello·go-types}` → "8·9장" (이웃 장 묶음)
- `{ch-go-basics~go-testing}` → "12~14장" (범위)
- `{ch-deploy}` → "부록" (부록이 둘 이상이면 toc에 "부록 A"처럼 적어 구분한다)

toc에 없는 파일을 가리키면 빌드가 파일:줄 번호와 함께 에러를 낸다.
같은 원리로 원고 제목 줄에는 번호를 쓰지 않는다(`= Gin으로 만드는 HTTP API`).
빌드가 toc 라벨을 붙여 "15장 …"으로 만든다.
코드 블록 안은 치환하지 않으므로, 코드 주석에 적은 장 번호는 손으로 맞춰야 한다.

## 코드 인용 (include)

기술서의 코드 예제는 저장소의 실제 코드를 베껴 오는 대신 인용한다.
downdoc은 include를 버리므로(README의 "include directives are dropped") `lib/include.mjs`가 변환 앞단에서 직접 펼친다.

```
[source,go]
----
include::../../internal/srs/srs.go[tag=grade]
----
```

- 경로는 원고 파일 기준 상대 경로. `include::`는 코드 블록(`----`) 안에서만 쓴다.
- 인용할 코드에는 그 언어의 주석으로 마커를 단다. `// tag::grade[]` … `// end::grade[]` (셸·YAML은 `#`, HTML은 `<!-- -->`).
- 마커 줄과 발췌 앞뒤 빈 줄은 결과에서 빠진다. `[]`만 쓰면 파일 전체를 인용한다.
- 옵션은 `tag=이름`과 `indent=0`(공통 들여쓰기 제거) 둘뿐이다. `lines=`는 코드가 바뀌면 어긋나므로 지원하지 않는다.
- 인용한 파일이 없거나 태그를 찾지 못하면 `book build`가 **원고의 파일:줄**과 함께 실패한다. 코드만 고치고 원고를 잊는 사고를 여기서 잡는다.
- `book dev`는 인용된 코드 파일도 감시해, 코드를 고치면 책을 다시 만든다.

## 마크다운 원고 이행

기존 VitePress 마크다운 책은 `tools/md2adoc.mjs`로 일괄 변환하고, `tools/verify-roundtrip.mjs`로 왕복 결과(adoc→md)가 원본과 일치하는지 검증한다.
자세한 사용법은 각 파일 머리 주석에 있다.

## 알아 둘 것

- CSS 클래스와 형광펜 하이라이트 이름의 `fc-` 접두사는 테마 네임스페이스다(책 이름과 무관하며 바꿀 필요 없다). localStorage 키 접두사만 `storage.prefix`로 책마다 달리한다.
- 상위 디렉터리에 PostCSS/Tailwind 설정이 있는 저장소 안에 책을 둘 때는 `templates/postcss.config.mjs`(빈 설정)를 책 디렉터리에 복사해 설정 상향 탐색을 막는다.
- VitePress 설정을 직접 만져야 하면 `book.config.mjs`에 `vitepress: { … }` 필드를 두면 최상위 키가 얕게 병합된다.
