// 이 책의 단일 설정 소스.
// 사이트(사이드바·nav·OG), PDF(표지·차례·아웃라인), 홈 표지가 모두 여기서 파생된다.
// 장을 더하거나 빼면 이 파일의 toc만 고치면 된다.
export default {
  lang: 'ko-KR',
  title: '이해하며 만드는 나만의 웹 앱',
  subtitle: 'AI에게 지시해 쉬운 프로그래밍 언어로 만들고, 개념을 익혀 오랫동안 안정적으로 운영한다',
  description:
    'AI에게 지시해 쉬운 프로그래밍 언어로 만들고, 개념을 익혀 오랫동안 안정적으로 운영한다. 작은 웹 앱의 실제 코드로 Go, Gin, PostgreSQL과 HTML·htmx를 익히고, Vercel·Supabase 무료 티어로 서버 비용 없이 배포해 기능을 더해 가는 과정을 배운다.',
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
    subtitleHtml: 'AI에게 지시해 쉬운 프로그래밍 언어로 만들고,<br>개념을 익혀 오랫동안 안정적으로 운영한다',
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
        { file: 'how-to-read.adoc', text: '이 책을 읽는 법', pdfPart: '읽는 법' },
        {
          file: 'requirements.adoc',
          text: '무엇을 만드는가: Flashcard의 기능 요구사항',
          pdfTitle: '도입: 무엇을 만드는가',
          pdfPart: '도입',
        },
      ],
    },
    {
      text: '1부 AI와 함께 시작하기',
      items: [
        { file: 'part1/tech-choices.adoc', text: '1장 기술 선택: 요구사항에서 아키텍처까지' },
        { file: 'part1/setup.adoc', text: '2장 개발 도구 설치' },
        { file: 'part1/claude-code.adoc', text: '3장 Claude Code: AI 에이전트와 개발하기' },
        {
          file: 'part1/instructing.adoc',
          text: '4장 에이전트에게 지시하기: Plan 모드로 다듬는 요구사항과 아키텍처',
        },
        { file: 'part1/first-run.adoc', text: '5장 완성된 앱 띄워 보기: 클론에서 실행까지' },
      ],
    },
    {
      text: '2부 세 언어 첫걸음: HTML·CSS, Go, SQL',
      items: [
        { file: 'part2/html-hello.adoc', text: '6장 HTML 첫걸음: 파일 하나로 만드는 웹 문서' },
        { file: 'part2/css-hello.adoc', text: '7장 CSS 첫걸음: 색과 배치를 입히는 규칙' },
        { file: 'part2/go-hello.adoc', text: '8장 Go 첫걸음: Hello, World로 배우는 기본 문법' },
        { file: 'part2/go-types.adoc', text: '9장 Go로 데이터 다루기: 구조체, 슬라이스와 맵' },
        { file: 'part2/database-basics.adoc', text: '10장 데이터베이스 기초: 테이블, SQL, 인덱스' },
      ],
    },
    {
      text: '3부 앱 코드 해부: 데이터에서 화면까지',
      items: [
        { file: 'part3/database.adoc', text: '11장 데이터베이스 설계: 요구사항에서 테이블로' },
        { file: 'part3/go-basics.adoc', text: '12장 Go 기초: 모듈, 변수, 함수' },
        { file: 'part3/go.adoc', text: '13장 Go 코드 읽기: 구조체, 포인터, 에러 처리' },
        { file: 'part3/go-testing.adoc', text: '14장 Go 테스트와 품질 검사 도구' },
        { file: 'part3/gin.adoc', text: '15장 Gin으로 만드는 HTTP API' },
        { file: 'part3/html-css.adoc', text: '16장 HTML과 CSS: 화면을 이루는 문서와 스타일' },
        { file: 'part3/go-templates.adoc', text: '17장 html/template으로 만드는 화면' },
        { file: 'part3/htmx.adoc', text: '18장 htmx: 자바스크립트 없이 만드는 동적 화면' },
        { file: 'part3/local-dev.adoc', text: '19장 로컬 개발 환경: 무설정 실행의 원리와 개발 도구' },
      ],
    },
    {
      text: '4부 세상에 공개하기',
      items: [
        { file: 'part4/git.adoc', text: '20장 Git: 개념과 브랜치 정책' },
        { file: 'part4/quality-gates.adoc', text: '21장 품질 게이트: 훅과 서브에이전트' },
        { file: 'part4/github-actions.adoc', text: '22장 GitHub Actions: 원격 품질 게이트' },
        { file: 'part4/vercel.adoc', text: '23장 Vercel: 한 플랫폼에 모두 배포하기' },
        { file: 'part4/supabase-auth.adoc', text: '24장 Supabase 인증: OAuth 로그인과 쿠키 세션' },
        { file: 'part4/token-verification.adoc', text: '25장 토큰 검증의 원리: JWT와 JWKS' },
        {
          file: 'part4/supabase-db.adoc',
          text: '26장 Supabase 데이터베이스: pgx 연결과 커넥션 풀',
        },
        { file: 'part4/migrations.adoc', text: '27장 마이그레이션: 운영 중에 스키마를 고친다' },
      ],
    },
    {
      text: '5부 안정적으로 오래 운영하기',
      items: [
        { file: 'part5/env-secrets.adoc', text: '28장 환경 변수와 시크릿: 값이 두 벌이 된다' },
        { file: 'part5/pwa.adoc', text: '29장 PWA: 설치되는 앱으로 만들기' },
        { file: 'part5/free-tier.adoc', text: '30장 무료 티어 운영과 한도 관리' },
        { file: 'part5/whats-next.adoc', text: '31장 다음 단계: 여기서 더 공부할 것들' },
      ],
    },
    {
      text: '부록',
      items: [
        { file: 'appendix/deploy.adoc', text: '부록 배포 준비: Supabase·Google·GitHub·Vercel 설정' },
      ],
    },
  ],
}
