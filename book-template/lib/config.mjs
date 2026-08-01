// book.config.mjs 하나를 VitePress 설정으로 펼친다.
// 소비자의 .vitepress/config.ts는 이 함수를 부르는 몇 줄짜리 심(shim)이면 된다.
import { defineConfig } from 'vitepress'
import { FONT_URL, coverFontFaceCss } from './fonts.mjs'
import { firstRoute, sidebar } from './toc.mjs'

export function defineBookConfig(book) {
  return defineConfig({
    lang: book.lang ?? 'ko-KR',
    title: book.title,
    description: book.description,
    base: book.base,
    // 원고(.adoc)에서 생성한 md가 여기 모인다. `book build`/`book dev`가 채운다.
    srcDir: './.generated',
    cleanUrls: true,
    markdown: {
      config(md) {
        // 인라인 코드의 {{ }}가 Vue 보간으로 해석되지 않게 v-pre를 붙인다.
        // (Go 템플릿 문법 `{{.Title}}` 같은 표기가 본문에 자주 나온다)
        md.renderer.rules.code_inline = (tokens, idx, _options, _env, self) => {
          const token = tokens[idx]
          return `<code v-pre${self.renderAttrs(token)}>${md.utils.escapeHtml(token.content)}</code>`
        }
      },
    },
    head: [
      // SNS 공유 미리보기(Open Graph). og-image.png는 `book og`가 표지에서 만든다.
      ['meta', { property: 'og:type', content: 'website' }],
      ['meta', { property: 'og:title', content: book.title }],
      ['meta', { property: 'og:description', content: book.description }],
      ['meta', { property: 'og:image', content: `${book.site}og-image.png` }],
      ['meta', { property: 'og:image:width', content: '1200' }],
      ['meta', { property: 'og:image:height', content: '630' }],
      ['meta', { property: 'og:locale', content: (book.lang ?? 'ko-KR').replace('-', '_') }],
      ['meta', { name: 'twitter:card', content: 'summary_large_image' }],
      ['link', { rel: 'preconnect', href: 'https://fonts.googleapis.com' }],
      ['link', { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' }],
      ['link', { rel: 'stylesheet', href: FONT_URL }],
      // 표지 글꼴(D2Coding). 파일은 이 책의 public/fonts/에 있다.
      ['style', {}, coverFontFaceCss(book.base)],
    ],
    vite: {
      // 테마(.vue)가 이 패키지 안에 있으므로 SSR 빌드에서 외부 모듈로 두지 않고 함께 컴파일한다.
      ssr: { noExternal: ['book-template'] },
      optimizeDeps: { exclude: ['book-template'] },
    },
    themeConfig: {
      nav: [
        { text: '홈', link: '/' },
        { text: '읽기 시작', link: '/' + firstRoute(book) },
        { text: 'PDF 다운로드', link: `${book.site}${book.pdf.fileName}` },
      ],
      sidebar: sidebar(book),
      outline: false,
      docFooter: { prev: '이전 장', next: '다음 장' },
      lastUpdated: { text: '마지막 수정' },
      darkModeSwitchLabel: '다크 모드',
      sidebarMenuLabel: '목차',
      returnToTopLabel: '맨 위로',
      search: {
        provider: 'local',
        options: {
          translations: {
            button: { buttonText: '검색', buttonAriaLabel: '검색' },
            modal: {
              noResultsText: '결과가 없습니다',
              resetButtonTitle: '초기화',
              footer: { selectText: '선택', navigateText: '이동', closeText: '닫기' },
            },
          },
        },
      },
      ...(book.repo ? { socialLinks: [{ icon: 'github', link: book.repo }] } : {}),
      // 이북 뷰어(Layout.vue)가 북마크·형광펜을 저장할 localStorage 키 접두사
      ebook: {
        storagePrefix: book.storage?.prefix ?? 'book',
        legacyPrefixes: book.storage?.legacyPrefixes ?? [],
      },
    },
    // 탈출구: 위 기본값을 책이 직접 덮어쓸 때 (최상위 키 얕은 병합)
    ...(book.vitepress ?? {}),
  })
}
