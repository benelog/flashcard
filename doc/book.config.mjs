// 이 책의 단일 설정 소스.
// 사이트(사이드바·nav·OG), PDF(표지·차례·아웃라인), 홈 표지가 모두 여기서 파생된다.
// 장을 더하거나 빼면 이 파일의 toc만 고치면 된다.
export default {
  lang: 'ko-KR',
  title: '이해하며 만드는 나만의 웹 앱',
  subtitle: 'AI에게 지시해 쉬운 프로그래밍 언어로 만들고, 개념을 익혀 오래 운영한다',
  description:
    'AI에게 지시해 쉬운 프로그래밍 언어로 만들고, 개념을 익혀 오래 운영한다. 작은 웹 앱의 실제 코드로 Go, Gin, PostgreSQL과 HTML·htmx를 익히고 Vercel·Supabase 무료 티어로 서버 비용 없이 운영하는 과정을 배운다.',
  site: 'https://benelog.github.io/flashcard/',
  base: '/flashcard/',
  repo: 'https://github.com/benelog/flashcard',
  author: '정상혁',
  siteLabel: 'benelog.github.io/flashcard',
  pdf: { fileName: 'flashcard-book.pdf' },
  // 북마크·형광펜 localStorage 키 접두사. ef-는 앱 이름 변경 전의 잔재를 이행한다.
  storage: { prefix: 'fc', legacyPrefixes: ['ef'] },
  cover: {
    kicker: 'AI와 함께 만드는 실전 개발서',
    volume: '01',
    titleHtml: '<strong>이해</strong>하며<br>만드는<br>나만의 웹 앱',
    subtitleHtml: 'AI에게 지시해 쉬운 프로그래밍 언어로 만들고,<br>개념을 익혀 오래 운영한다',
    diagram: [
      { name: '화면', tech: 'HTML · CSS' },
      { name: '서버', tech: 'Go' },
      { name: '데이터', tech: 'SQL' },
    ],
    pitch: [
      '코드는 AI 에이전트가 쓰고, 판단은 사람이 한다',
      '이해할 언어는 최소화(HTML/CSS, Go, SQL)',
      '무료 티어에 유리하게 수십 MB 메모리만 사용하는 서버',
    ],
    homeDesc:
      '암기 카드 앱의 실제 코드로 Go, Gin, PostgreSQL과 HTML·htmx를 익히고, Claude Code와 함께 개발해 Vercel·Supabase 무료 티어로 서버 비용 없이 배포·운영하는 과정을 배운다.',
    actions: [
      { text: '읽기 시작', link: 'start', brand: true },
      { text: 'PDF 다운로드', link: 'pdf' },
      { text: 'GitHub 저장소', link: 'repo' },
    ],
  },
  toc: [
    {
      text: '도입',
      items: [
        { file: 'preface.adoc', text: '저자 서문', pdfPart: '서문' },
        {
          file: 'intro.adoc',
          text: '무엇을 만드는가: Flashcard의 기능 요구사항',
          pdfTitle: '도입: 무엇을 만드는가',
          pdfPart: '도입',
        },
      ],
    },
    {
      text: '1부 내 컴퓨터에서 웹 앱 완성하기',
      items: [
        { file: 'part1/tech-choices.adoc', text: '1장 기술 선택: 요구사항에서 아키텍처까지' },
        { file: 'part1/claude-code.adoc', text: '2장 Claude Code: AI 에이전트와 개발하기' },
        {
          file: 'part1/instructing.adoc',
          text: '3장 에이전트에게 지시하기: Plan 모드로 다듬는 요구사항과 아키텍처',
        },
        { file: 'part1/database-basics.adoc', text: '4장 데이터베이스 기초: 테이블, SQL, 인덱스' },
        { file: 'part1/database.adoc', text: '5장 데이터베이스 설계: 요구사항에서 테이블로' },
        { file: 'part1/go-hello.adoc', text: '6장 Go 첫걸음: Hello, World로 배우는 기본 문법' },
        { file: 'part1/go-types.adoc', text: '7장 Go로 데이터 다루기: 구조체, 슬라이스와 맵, 제네릭' },
        { file: 'part1/go-basics.adoc', text: '8장 Go 기초: 모듈, 변수, 함수' },
        { file: 'part1/go.adoc', text: '9장 Go 코드 읽기: 구조체, 포인터, 에러 처리' },
        { file: 'part1/go-testing.adoc', text: '10장 Go 테스트와 품질 게이트: 도구, 훅, 서브에이전트' },
        { file: 'part1/gin.adoc', text: '11장 Gin으로 만드는 HTTP API' },
        { file: 'part1/html-hello.adoc', text: '12장 HTML 첫걸음: 파일 하나로 만드는 웹 문서' },
        { file: 'part1/css-hello.adoc', text: '13장 CSS 첫걸음: 색과 배치를 입히는 규칙' },
        { file: 'part1/html-css.adoc', text: '14장 HTML과 CSS: 화면을 이루는 문서와 스타일' },
        { file: 'part1/go-templates.adoc', text: '15장 html/template으로 만드는 화면' },
        { file: 'part1/htmx.adoc', text: '16장 htmx: 자바스크립트 없이 만드는 동적 화면' },
        { file: 'part1/local-dev.adoc', text: '17장 로컬 개발 환경: 내 컴퓨터에서 앱 완성하기' },
      ],
    },
    {
      text: '2부 세상에 공개하고 오래 운영하기',
      items: [
        { file: 'part2/git.adoc', text: '18장 Git: 개념과 브랜치 정책' },
        { file: 'part2/github-actions.adoc', text: '19장 GitHub Actions: 원격 품질 게이트' },
        { file: 'part2/vercel.adoc', text: '20장 Vercel: 한 플랫폼에 모두 배포하기' },
        { file: 'part2/supabase-auth.adoc', text: '21장 Supabase 인증: OAuth와 JWKS 검증' },
        {
          file: 'part2/supabase-db.adoc',
          text: '22장 Supabase 데이터베이스: pgx 연결과 개발·운영 DB 분리',
        },
        { file: 'part2/pwa.adoc', text: '23장 PWA: 설치되는 앱으로 만들기' },
        { file: 'part2/free-tier.adoc', text: '24장 무료 티어 운영과 한도 관리' },
        { file: 'part2/whats-next.adoc', text: '25장 다음 단계: 여기서 더 공부할 것들' },
      ],
    },
    {
      text: '부록',
      items: [
        { file: 'appendix/setup.adoc', text: '부록 A 개발 도구 설치' },
        { file: 'appendix/deploy.adoc', text: '부록 B 배포 준비: Supabase·Google·GitHub·Vercel 설정' },
      ],
    },
  ],
}
