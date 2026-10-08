// 주문형 인쇄(POD)에 올릴 본문 PDF. 화면용 PDF(pdf.mjs)와 같은 빌드 결과를 인쇄하되,
// 종이책 조판(작은 글자·좁은 행간·흑백 코드)을 덧입히고 홀짝 쪽 여백을 거울처럼 나눈다.
// 표지는 POD 업체에 따로 올리므로 넣지 않는다. 구성: 표제지 · 판권 · 차례 · 본문.
//
// 홀짝 여백: Chrome은 장마다 따로 인쇄하므로 쪽의 홀짝을 알 수 없다. 그래서 좌우 여백을
// (안쪽+바깥쪽)/2로 같게 인쇄한 뒤, 병합할 때 홀수 쪽(오른쪽 면)은 오른쪽으로, 짝수 쪽은
// 왼쪽으로 (안쪽−바깥쪽)/2만큼 옮겨 안쪽 여백을 넓힌다.
//
// 글꼴: Chrome은 가변 글꼴과 CFF 기반 OpenType(Google Fonts의 Noto KR, 시스템의 Noto CJK)을
// Type 3로 넣는다. 인쇄소가 꺼리는 형식이고 파일도 커진다. 그래서 웹 글꼴 요청을 막고
// 정적 TrueType인 나눔명조·나눔고딕과 네이버 D2Coding(모두 OFL)을 내려받아 .pod/fonts/에 두고 쓴다.
// 코드에는 합자 없는 D2Coding을 쓴다(합자판은 != 같은 기호를 한 글자로 그려 따라 치기 어렵다).
// 쪽 번호도 pdf-lib 기본 글꼴(임베딩되지 않는다) 대신 Chrome으로 그려 겹친다.
import { mkdir, writeFile } from 'node:fs/promises'
import { existsSync } from 'node:fs'
import { join } from 'node:path'
import puppeteer from 'puppeteer-core'
import { PDFDocument } from 'pdf-lib'
import { pdfTocHtml } from './cover.mjs'
import { addOutline } from './pdf.mjs'
import { findChrome, serveDist } from './server.mjs'
import { flattenChapters } from './toc.mjs'

const MM = 72 / 25.4 // 1mm를 PDF 포인트로
// 나눔 글꼴에 없는 한자·원 숫자(①)는 D2Coding이 받는다. 시스템 Noto CJK로 넘어가면 Type 3가 된다.
const SERIF = "'POD Serif', 'POD Mono', 'DejaVu Serif', serif"
const SANS = "'POD Sans', 'POD Mono', 'DejaVu Sans', sans-serif"
const MONO = "'POD Mono', 'DejaVu Sans Mono', monospace"
const FONT_MOUNT = '/__pod-fonts/'
const GOOGLE_FONTS = 'https://raw.githubusercontent.com/google/fonts/main/ofl/'
const D2CODING = 'https://raw.githubusercontent.com/naver/d2codingfont/VER1.4.0/fonts/ttf/'
const FONTS = [
  ['POD Serif', 400, `${GOOGLE_FONTS}nanummyeongjo/NanumMyeongjo-Regular.ttf`],
  ['POD Serif', 700, `${GOOGLE_FONTS}nanummyeongjo/NanumMyeongjo-Bold.ttf`],
  ['POD Sans', 400, `${GOOGLE_FONTS}nanumgothic/NanumGothic-Regular.ttf`],
  ['POD Sans', 700, `${GOOGLE_FONTS}nanumgothic/NanumGothic-Bold.ttf`],
  ['POD Mono', 400, `${D2CODING}D2Coding-Regular.ttf`],
  ['POD Mono', 700, `${D2CODING}D2Coding-Bold.ttf`],
]
const fontFile = (url) => url.replace(/^.*\//, '')

// 글꼴 파일이 캐시에 없으면 배포 저장소(google/fonts, naver/d2codingfont)에서 받는다
async function ensureFonts(dir) {
  await mkdir(dir, { recursive: true })
  for (const [, , url] of FONTS) {
    const file = join(dir, fontFile(url))
    if (existsSync(file)) continue
    const res = await fetch(url)
    if (!res.ok) throw new Error(`글꼴을 받지 못했다: ${url} (${res.status})`)
    await writeFile(file, Buffer.from(await res.arrayBuffer()))
    console.log(`글꼴 내려받음: ${fontFile(url)}`)
  }
}

// 화면용 PDF와 함께 쓰는 쪽 HTML(차례)의 웹 글꼴을 POD 글꼴로 바꾼다
const localFonts = (html) =>
  html
    .replace('<style>', `<style>\n${FONT_FACES}`)
    .replaceAll("'Noto Serif KR', serif", SERIF)
    .replaceAll("'Noto Sans KR', sans-serif", SANS)

const FONT_FACES = FONTS.map(
  ([family, weight, url]) =>
    `@font-face { font-family: '${family}'; font-weight: ${weight}; src: url('${FONT_MOUNT}${fontFile(url)}') format('truetype'); }`,
).join('\n')

// 기본값은 교보 바로출판 POD의 B5(46배판) 188×254mm다. book.config의 pod로 바꿀 수 있다.
const DEFAULTS = {
  fileName: 'book-pod.pdf',
  width: 188,
  height: 254,
  margin: { top: 20, bottom: 22, inner: 22, outer: 16 },
}

// 본문 쪽에 덧입히는 인쇄 조판. 웹 화면용 크기(17px·행간 2.05)를 종이책 크기로 줄이고,
// 흑백 인쇄에서 뭉개지는 코드 색을 굵기와 회색으로 바꾼다.
function podCss() {
  return `
${FONT_FACES}
:root {
  --fc-font-serif: ${SERIF} !important;
  --vp-font-family-base: ${SANS} !important;
  --vp-font-family-mono: ${MONO} !important;
}
html { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
.vp-doc { color: #000 !important; }
.vp-doc p, .vp-doc li { font-size: 10pt !important; line-height: 1.7 !important; }
.vp-doc p { margin: 0 0 0.75em !important; }
.vp-doc li + li { margin-top: 0.15em !important; }
.vp-doc ul, .vp-doc ol { margin: 0.4em 0 0.8em !important; }
.vp-doc h1 {
  font-size: 18pt !important; line-height: 1.4 !important;
  margin: 0 0 9mm !important; padding: 0 0 3mm !important; border-bottom: 0.6pt solid #000 !important;
}
.vp-doc h2 {
  font-size: 13pt !important; line-height: 1.45 !important;
  margin: 7mm 0 3mm !important; padding-top: 0 !important; border-top: none !important;
}
.vp-doc h3 { font-size: 11pt !important; line-height: 1.45 !important; margin: 5mm 0 2mm !important; }
.vp-doc h4 { font-size: 10pt !important; margin: 4mm 0 1.5mm !important; }
.vp-doc a { color: inherit !important; text-decoration: none !important; }
.vp-doc :not(pre) > code {
  font-family: ${MONO} !important; font-size: 9pt !important;
  color: #000 !important; background: #ececec !important; padding: 0 0.25em !important;
}

/* 코드 블록: 연한 회색 바탕, 흑백 강조(키워드 굵게, 주석 회색 기울임) */
.vp-doc div[class*='language-'] {
  margin: 2.5mm 0 4mm !important; border-radius: 0 !important;
  background: #f2f2f2 !important; border: none !important;
}
.vp-doc div[class*='language-'] pre {
  padding: 2.5mm 3mm !important; margin: 0 !important; background: transparent !important;
}
.vp-doc div[class*='language-'] code {
  font-family: ${MONO} !important; font-size: 8pt !important;
  line-height: 1.45 !important; word-break: break-all;
}
.vp-doc pre.shiki span { color: #000 !important; }
.vp-doc pre.shiki span[style*='--shiki-light:#D73A49'] { font-weight: 700; }
.vp-doc pre.shiki span[style*='--shiki-light:#6A737D'] { color: #555 !important; font-style: italic; }
.vp-doc pre.shiki span[style*='--shiki-light:#032F62'] { color: #222 !important; }

/* 용어 상자 */
.vp-doc .custom-block {
  margin: 3mm 0 4mm !important; padding: 2.5mm 3.5mm !important;
  border: 0.5pt solid #888 !important; border-left: 2pt solid #000 !important; background: none !important;
}
.vp-doc .custom-block p { font-size: 9pt !important; line-height: 1.6 !important; margin: 0 0 0.4em !important; }
.vp-doc .custom-block .custom-block-title { font-size: 9pt !important; margin-bottom: 1mm !important; }

/* 표 */
.vp-doc table { display: table !important; width: 100% !important; margin: 3mm 0 4mm !important; border-collapse: collapse !important; }
.vp-doc th, .vp-doc td {
  font-size: 8.5pt !important; line-height: 1.5 !important; padding: 1.2mm 2mm !important;
  border: 0.5pt solid #999 !important; background: none !important; color: #000 !important;
}
.vp-doc tr { background: none !important; break-inside: avoid; }
.vp-doc th { font-weight: 700 !important; background: #ececec !important; }

/* 그림과 캡션 */
.vp-doc .fc-shots { margin: 4mm 0 1mm !important; }
.vp-doc .fc-shots img { max-height: 150mm; border: 0.5pt solid #999 !important; border-radius: 0 !important; }
.vp-doc .fc-shots.single img { max-width: 62mm !important; }
.vp-doc .fc-caption { font-size: 8.5pt !important; line-height: 1.5 !important; color: #333 !important; margin: 1.5mm 0 5mm !important; }

/* 인용과 각주 */
.vp-doc blockquote { margin: 3mm 0 !important; padding-left: 4mm !important; border-left: 1.5pt solid #999 !important; }
.vp-doc blockquote p { font-size: 9.5pt !important; color: #222 !important; }
.vp-doc .footnotes-sep { margin: 6mm 0 2mm !important; }
.vp-doc .footnotes p, .vp-doc .footnotes li { font-size: 8.5pt !important; line-height: 1.5 !important; }
`
}

const PAGE_CSS = (w, h) => `
  ${FONT_FACES}
  @page { size: ${w}mm ${h}mm; margin: 0; }
  html, body { margin: 0; padding: 0; }
  body { font-family: ${SERIF}; color: #000; }
  .sheet { box-sizing: border-box; width: ${w}mm; height: ${h}mm; break-after: page; position: relative; }
`

// 표제지: 책 제목·부제·지은이만 담은 첫 오른쪽 면
function titlePageHtml(book, w, h) {
  return `<!doctype html><html lang="ko"><head><meta charset="utf-8">
  <style>${PAGE_CSS(w, h)}
    .sheet { padding: 60mm 22mm 0 30mm; }
    h1 { font-family: ${SANS}; font-size: 26pt; font-weight: 700; line-height: 1.35; margin: 0; word-break: keep-all; }
    .sub { font-size: 12pt; margin: 6mm 0 0; color: #333; }
    .author { position: absolute; left: 30mm; bottom: 40mm; font-size: 12pt; font-family: ${SANS}; font-weight: 500; }
  </style></head><body>
  <div class="sheet"><h1>${book.title}</h1><p class="sub">${book.subtitle ?? ''}</p><p class="author">${book.author} 지음</p></div>
  </body></html>`
}

// 판권: 표제지 뒷면(왼쪽 면) 아래쪽에 서지 정보를 모은다. ISBN은 POD 업체가 발급한 뒤 채운다.
function copyrightPageHtml(book, pod, w, h) {
  const today = new Date()
  const date = `${today.getFullYear()}년 ${today.getMonth() + 1}월 ${today.getDate()}일`
  const rows = [
    ['지은이', book.author],
    ['펴낸 날', date],
    ['웹에서 읽기', book.siteLabel ?? book.site],
    ...(pod.isbn ? [['ISBN', pod.isbn]] : []),
  ]
  const license = book.cover?.licenseHtml ? `<p class="license">${book.cover.licenseHtml}</p>` : ''
  return `<!doctype html><html lang="ko"><head><meta charset="utf-8">
  <style>${PAGE_CSS(w, h)}
    .sheet { padding: 0 22mm 0 16mm; display: flex; flex-direction: column; justify-content: flex-end; padding-bottom: 30mm; }
    .t { font-family: ${SANS}; font-size: 11pt; font-weight: 700; margin: 0 0 4mm; }
    table { border-collapse: collapse; font-size: 8.5pt; line-height: 1.8; }
    td:first-child { padding-right: 5mm; color: #444; white-space: nowrap; }
    .license { font-size: 7.5pt; line-height: 1.6; color: #333; margin: 5mm 0 0; word-break: keep-all; }
    .license a { color: inherit; text-decoration: none; }
  </style></head><body>
  <div class="sheet"><p class="t">${book.title}</p>
  <table>${rows.map(([k, v]) => `<tr><td>${k}</td><td>${v}</td></tr>`).join('')}</table>${license}</div>
  </body></html>`
}

// 본문 쪽 번호만 찍힌 투명한 쪽들. 본문 1쪽이 오른쪽 면이므로 홀수는 오른쪽, 짝수는 왼쪽 모서리.
function pageNumbersHtml(count, w, h, m) {
  const sheets = Array.from({ length: count }, (_, i) => {
    const n = i + 1
    const side = n % 2 === 1 ? `right: ${m.outer}mm` : `left: ${m.outer}mm`
    return `<div class="sheet"><span style="${side}">${n}</span></div>`
  }).join('')
  return `<!doctype html><html lang="ko"><head><meta charset="utf-8">
  <style>${PAGE_CSS(w, h)}
    .sheet span { position: absolute; bottom: ${m.bottom / 2 - 1.5}mm; font-family: ${SANS}; font-size: 8pt; }
  </style></head><body>${sheets}</body></html>`
}

export function podFileName(book) {
  return book.pod?.fileName ?? book.pdf.fileName.replace(/\.pdf$/, '-pod.pdf')
}

export async function exportPod(root, book) {
  const pod = {
    ...DEFAULTS,
    ...book.pod,
    margin: { ...DEFAULTS.margin, ...book.pod?.margin },
  }
  const { width: w, height: h, margin: m } = pod
  const side = (m.inner + m.outer) / 2
  const pdfOpts = (margin) => ({
    width: `${w}mm`,
    height: `${h}mm`,
    printBackground: true,
    margin,
  })
  const bodyMargin = { top: `${m.top}mm`, bottom: `${m.bottom}mm`, left: `${side}mm`, right: `${side}mm` }

  const dist = join(root, '.vitepress/dist')
  const outDir = join(root, '.pod')
  const fontDir = join(outDir, 'fonts')
  await ensureFonts(fontDir)
  const chapters = flattenChapters(book)
  const { port, close } = await serveDist(dist, book.base, { [FONT_MOUNT]: fontDir })
  const browser = await puppeteer.launch({
    executablePath: findChrome(),
    args: ['--no-sandbox', '--font-render-hinting=none'],
  })

  const chapterDocs = []
  let titleBuf, copyrightBuf, tocBuf, numbersBuf
  try {
    const page = await browser.newPage()
    await page.setRequestInterception(true)
    page.on('request', (req) =>
      /fonts\.(googleapis|gstatic)\.com/.test(req.url()) ? req.abort() : req.continue(),
    )
    for (const { route } of chapters) {
      await page.goto(`http://127.0.0.1:${port}${book.base}${route}`, {
        waitUntil: 'networkidle0',
        timeout: 90_000,
      })
      await page.addStyleTag({ content: podCss() })
      await page.evaluate(() => Promise.allSettled([...document.fonts].map((f) => f.load())))
      chapterDocs.push(await PDFDocument.load(await page.pdf(pdfOpts(bodyMargin))))
      console.log(`printed: ${route}`)
    }

    const startPages = []
    let cursor = 1
    for (const doc of chapterDocs) {
      startPages.push(cursor)
      cursor += doc.getPageCount()
    }

    const render = async (html, margin) => {
      await page.setContent(html, { waitUntil: 'load', timeout: 60_000 })
      await page.evaluate(() => Promise.allSettled([...document.fonts].map((f) => f.load())))
      return page.pdf(pdfOpts(margin))
    }
    const none = { top: 0, bottom: 0, left: 0, right: 0 }
    titleBuf = await render(titlePageHtml(book, w, h), none)
    copyrightBuf = await render(copyrightPageHtml(book, pod, w, h), none)
    tocBuf = await render(localFonts(pdfTocHtml(chapters, startPages)), bodyMargin)
    numbersBuf = await render(pageNumbersHtml(cursor - 1, w, h, m), none)
  } finally {
    await browser.close()
    close()
  }

  // ── 병합: 쪽마다 홀짝에 맞춰 옮겨 붙인다 ──────────────────────
  const out = await PDFDocument.create()
  out.setTitle(book.title)
  out.setAuthor(book.author)
  out.setSubject(book.subtitle)
  out.setLanguage(book.lang ?? 'ko-KR')
  const pageW = w * MM
  const pageH = h * MM
  const shift = ((m.inner - m.outer) / 2) * MM

  const blank = () => out.addPage([pageW, pageH])
  // 이미 거울 여백을 직접 잡은 쪽(표제지·판권)은 옮기지 않는다
  async function place(srcBytes, { mirror = true } = {}) {
    const src = srcBytes instanceof PDFDocument ? srcBytes : await PDFDocument.load(srcBytes)
    const embedded = await out.embedPdf(src, src.getPageIndices())
    const first = out.getPageCount()
    for (const e of embedded) {
      const recto = out.getPageCount() % 2 === 0 // 0부터 세므로 짝수 인덱스가 오른쪽 면
      const p = out.addPage([pageW, pageH])
      p.drawPage(e, { x: mirror ? (recto ? shift : -shift) : 0, y: 0 })
    }
    return first
  }

  await place(titleBuf, { mirror: false }) // 1쪽(오른쪽): 표제지
  await place(copyrightBuf, { mirror: false }) // 2쪽(왼쪽): 판권
  const tocIndex = await place(tocBuf) // 3쪽부터 차례
  if (out.getPageCount() % 2 === 1) blank() // 본문 1쪽은 오른쪽 면에서 시작한다
  const bodyStart = out.getPageCount()
  const chapterStartIndex = []
  for (const doc of chapterDocs) chapterStartIndex.push(await place(doc))
  const bodyPages = out.getPageCount() - bodyStart

  // 쪽 번호: 본문 쪽마다 번호만 찍힌 쪽을 그대로 겹친다
  const numbers = await PDFDocument.load(numbersBuf)
  const numberPages = await out.embedPdf(numbers, numbers.getPageIndices())
  numberPages.forEach((np, k) => out.getPage(bodyStart + k).drawPage(np, { x: 0, y: 0 }))

  // 제본은 낱장(2쪽) 단위라 전체 쪽수를 짝수로 맞춘다
  if (out.getPageCount() % 2 === 1) blank()

  addOutline(out, [
    { title: '차례', pageIndex: tocIndex },
    ...chapters.map((c, i) => ({ title: c.title, pageIndex: chapterStartIndex[i] })),
  ])

  const file = join(outDir, podFileName(book))
  await writeFile(file, await out.save())
  console.log(
    `POD PDF 생성 완료: ${file} (${w}×${h}mm, 전체 ${out.getPageCount()}쪽 = 앞부분 ${bodyStart} + 본문 ${bodyPages} + 끝 빈 쪽 ${out.getPageCount() - bodyStart - bodyPages})`,
  )
}
