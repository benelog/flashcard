// 이 책의 단일 설정 소스.
// 사이트(사이드바·nav·OG), PDF(표지·차례·아웃라인), 홈 표지가 모두 여기서 파생된다.
// 장을 더하거나 빼면 이 파일의 toc만 고치면 된다.
import { readFileSync } from 'node:fs'

// 원고가 인용하는 저장소 수치. 본문은 {n-이름} 토큰으로 쓰고 빌드가 치환한다.
// 값은 `npm run metrics`가 저장소를 직접 세어 갱신한다(scripts/metrics.def.mjs).
const metrics = JSON.parse(readFileSync(new URL('./metrics.json', import.meta.url), 'utf8'))

export default {
  metrics,
  lang: 'ko-KR',
  title: '이해하며 만드는 나만의 웹 앱',
  subtitle: '쉬운 프로그래밍 언어로 시작하는 AI 코딩',
  description:
    '쉬운 프로그래밍 언어로 시작하는 AI 코딩. AI에게 지시해 만들고, 개념을 익혀 오랫동안 안정적으로 운영한다. 작은 웹 앱의 실제 코드로 Go, Gin, PostgreSQL과 HTML·htmx를 익히고, Vercel·Supabase 무료 티어로 서버 비용 없이 배포해 기능을 더해 가는 과정을 배운다.',
  site: 'https://benelog.github.io/flashcard/',
  base: '/flashcard/',
  repo: 'https://github.com/benelog/flashcard',
  author: '정상혁',
  siteLabel: 'benelog.github.io/flashcard',
  pdf: { fileName: 'flashcard-book.pdf' },
  // 북마크·형광펜 localStorage 키 접두사. ef-는 앱 이름 변경 전의 잔재를 이행한다.
  storage: { prefix: 'fc', legacyPrefixes: ['ef'] },
  cover: {
    titleHtml: '<strong>이해</strong>하며 만드는<br>나만의 웹 앱',
    subtitleHtml: '쉬운 프로그래밍 언어로 시작하는 AI 코딩',
    diagram: [
      { name: '화면', tech: 'HTML · CSS', motif: 'screen' },
      { name: '로직', tech: 'Go', motif: 'code' },
      { name: '데이터', tech: 'SQL', motif: 'table' },
    ],
    pitch: [
      '코딩이 처음이어도 개념까지 이해하며 만든다',
      '방향은 사람이 정하고, 코드는 AI 에이전트가 작성한다',
      '무료로 쓰는 클라우드에 올려 월 0원으로 운영한다',
    ],
    homeDesc:
      '코딩을 해 본 적이 없어도 시작할 수 있습니다. AI 코딩 에이전트인 Claude Code에게 지시해 암기 카드 앱을 만들고, 화면(HTML·CSS), 로직(Go), 데이터(SQL) 세 층의 실제 코드를 한 층씩 이해해 갑니다. 완성한 앱은 무료로 쓸 수 있는 클라우드 서비스인 Vercel과 Supabase에 올려, 서버 비용 없이 세상에 공개하고 운영하는 법까지 익힙니다.',
    actions: [
      { text: '읽기 시작', link: 'start', brand: true },
      { text: 'PDF 다운로드', link: 'pdf' },
      { text: 'GitHub 저장소', link: 'repo' },
    ],
    licenseHtml:
      '© 2026 정상혁. 원고는 <a href="https://creativecommons.org/licenses/by-nc-sa/4.0/deed.ko" target="_blank" rel="license noopener">CC BY-NC-SA 4.0</a>, 예제 코드는 <a href="https://github.com/benelog/flashcard/blob/main/LICENSE" target="_blank" rel="license noopener">MIT</a> 라이선스로 공개합니다. 출처를 밝히면 원고를 자유롭게 공유하고 고칠 수 있지만, 원고와 그 파생물을 상업적으로 이용할 수는 없습니다. 종이책 출판 등 상업적 이용은 <a href="https://github.com/benelog/flashcard/issues" target="_blank" rel="noopener">저자에게 문의</a>해 주세요.',
  },
  toc: [
    {
      text: '서문',
      items: [{ file: 'preface.adoc', text: '저자 서문', pdfPart: '서문' }],
    },
    {
      text: '1부 AI와 함께 시작하기',
      items: [
        {
          file: 'part1/requirements.adoc',
          text: '1장 무엇을 만드는가: 기능과 비기능 요구사항',
        },
        { file: 'part1/tech-choices.adoc', text: '2장 기술 선택: 요구사항에서 도출하는 네 계층' },
        { file: 'part1/claude-code.adoc', text: '3장 Claude Code: AI 에이전트와 개발하기' },
        {
          file: 'part1/instructing.adoc',
          text: '4장 에이전트에게 지시하기: Plan 모드로 다듬는 요구사항과 아키텍처',
        },
        {
          file: 'part1/first-run.adoc',
          text: '5장 개발 도구 설치와 첫 실행: 깔고, 받고, 띄운다',
        },
      ],
    },
    {
      text: '2부 웹 앱 코딩 기초: 문법에서 첫 앱까지',
      items: [
        { file: 'part2/html-hello.adoc', text: '6장 HTML 첫걸음: 파일 하나로 만드는 웹 문서' },
        { file: 'part2/css-hello.adoc', text: '7장 CSS 첫걸음: 색과 배치를 입히는 규칙' },
        { file: 'part2/go-hello.adoc', text: '8장 Go 첫걸음: Hello, World로 배우는 기본 문법' },
        { file: 'part2/go-types.adoc', text: '9장 Go로 데이터 다루기: 구조체에서 인터페이스까지' },
        { file: 'part2/database-basics.adoc', text: '10장 데이터베이스 기초: 테이블, SQL, 인덱스' },
        {
          file: 'part2/go-sqlite.adoc',
          text: '11장 database/sql 첫걸음: Go로 SQLite에 저장하고 꺼낸다',
        },
        { file: 'part2/gin-hello.adoc', text: '12장 Gin 첫걸음: 저장한 데이터를 웹 화면으로' },
        {
          file: 'part2/template-hello.adoc',
          text: '13장 html/template 첫걸음: 데이터를 화면에 채우고 조각으로 나눈다',
        },
      ],
    },
    {
      text: '3부 완성본 앱 해부: 데이터에서 화면까지',
      items: [
        {
          file: 'part3/design-principles.adoc',
          text: '14장 설계 철학: 이 앱을 지탱하는 다섯 가지 원칙',
        },
        {
          file: 'part3/code-map.adoc',
          text: '15장 코드는 어디에 있는가: 패키지 지도와 의존 방향',
        },
        { file: 'part3/database-design.adoc', text: '16장 데이터베이스 설계: 요구사항에서 테이블로' },
        { file: 'part3/domain-model.adoc', text: '17장 이 앱만의 설계 둘: 간격 반복 상태와 덱 주소' },
        {
          file: 'part3/core-logic.adoc',
          text: '18장 핵심 로직 읽기: 간격 반복 알고리즘과 이 앱이 푸는 문제들',
        },
        { file: 'part3/unit-tests.adoc', text: '19장 함수 하나씩 검증하기: 단위 테스트와 품질 검사 도구' },
        { file: 'part3/gin.adoc', text: '20장 Gin 깊이 보기: 완성본의 라우팅·바인딩·미들웨어' },
        { file: 'part3/app-assembly.adoc', text: '21장 앱을 조립하기: 계층 분리와 두 진입점' },
        {
          file: 'part3/html-css.adoc',
          text: '22장 화면의 구조: 템플릿 삼층과 스타일 시스템',
        },
        {
          file: 'part3/html-template.adoc',
          text: '23장 html/template 깊이 보기: 레이아웃 복제와 자동 이스케이프',
        },
        { file: 'part3/htmx.adoc', text: '24장 htmx: 부분 갱신과 무상태 학습 화면' },
        {
          file: 'part3/app-tests.adoc',
          text: '25장 앱을 통째로 띄우는 테스트: 화면과 API를 한 번에 검증하기',
        },
        { file: 'part3/local-dev.adoc', text: '26장 로컬 개발 환경: 무설정 실행의 원리와 개발 도구' },
      ],
    },
    {
      text: '4부 인터넷에 공개하기: 버전 관리에서 인증과 운영 데이터베이스까지',
      items: [
        { file: 'part4/git.adoc', text: '27장 Git: 개념과 브랜치 정책' },
        { file: 'part4/quality-gates.adoc', text: '28장 품질 게이트: Claude Code 훅과 서브에이전트' },
        { file: 'part4/github-actions.adoc', text: '29장 GitHub Actions: 원격 품질 게이트' },
        { file: 'part4/vercel.adoc', text: '30장 Vercel: 왜 이 플랫폼이고, 무엇이 배포되는가' },
        {
          file: 'part4/vercel-config.adoc',
          text: '31장 Vercel 설정과 제약: vercel.json, 콜드스타트, 배포 절차',
        },
        { file: 'part4/supabase-auth.adoc', text: '32장 Supabase 인증: OAuth 로그인과 쿠키 세션' },
        { file: 'part4/token-verification.adoc', text: '33장 토큰 검증의 원리: JWT와 JWKS' },
        {
          file: 'part4/supabase-db.adoc',
          text: '34장 Supabase 데이터베이스: 커넥션 풀러와 개발·운영 분리',
        },
        { file: 'part4/migrations.adoc', text: '35장 마이그레이션: 운영 중에 스키마를 고친다' },
        { file: 'part4/env-secrets.adoc', text: '36장 환경 변수와 시크릿: 개발과 운영, 두 벌이 된 값' },
      ],
    },
    {
      text: '5부 안정적으로 오래 운영하기',
      items: [
        { file: 'part5/pwa.adoc', text: '37장 PWA: 설치되는 앱으로 만들기' },
        { file: 'part5/caching.adoc', text: '38장 캐시와 서비스 워커: 무엇을 저장하고 언제 버리는가' },
        { file: 'part5/free-tier.adoc', text: '39장 무료 티어 운영: 무엇이 앱을 가장 먼저 멈추는가' },
        { file: 'part5/whats-next.adoc', text: '40장 다음 단계: 여기서 더 공부할 것들' },
      ],
    },
    {
      text: '부록',
      items: [
        {
          file: 'appendix/alternatives.adoc',
          text: '부록 A 검토한 대안 기술: 언제 다른 선택이 더 나은가',
        },
        {
          file: 'appendix/deploy.adoc',
          text: '부록 B 배포 준비: Supabase·Google·GitHub·Vercel 설정',
        },
      ],
    },
  ],
}
