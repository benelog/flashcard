// 종이에 찍는 PDF(직접 프린트용 A4: pdf.mjs, POD 원고: pod.mjs)가 함께 쓰는 인쇄 조판.
// 빌드된 사이트를 Chrome으로 인쇄하되, 웹 화면용 크기(17px·행간 2.05)를 종이책 크기로 줄이고
// 흑백에서 뭉개지는 코드 색을 굵기와 회색으로 바꾸는 CSS를 덧입힌다.
//
// 글꼴: Chrome은 가변 글꼴과 CFF 기반 OpenType(Google Fonts의 Noto KR, 시스템의 Noto CJK)을
// Type 3로 넣는다. 인쇄소가 꺼리는 형식이고 파일도 커진다. 그래서 웹 글꼴 요청을 막고
// 정적 TrueType인 나눔명조·나눔고딕과 네이버 D2Coding(모두 OFL)을 내려받아 .fonts-cache/에 두고 쓴다.
// 코드에는 합자 없는 D2Coding을 쓴다(합자판은 != 같은 기호를 한 글자로 그려 따라 치기 어렵다).
// 쪽 번호도 pdf-lib 기본 글꼴(임베딩되지 않는다) 대신 Chrome으로 그려 겹친다.
import { mkdir, writeFile } from 'node:fs/promises'
import { existsSync } from 'node:fs'
import { join } from 'node:path'
import puppeteer from 'puppeteer-core'
import { PDFDocument } from 'pdf-lib'
import { findChrome, serveDist } from './server.mjs'

export const MM = 72 / 25.4 // 1mm를 PDF 포인트로

// 나눔 글꼴에 없는 한자·원 숫자(①)는 D2Coding이 받는다. 시스템 Noto CJK로 넘어가면 Type 3가 된다.
export const SERIF = "'Print Serif', 'Print Mono', 'DejaVu Serif', serif"
export const SANS = "'Print Sans', 'Print Mono', 'DejaVu Sans', sans-serif"
export const MONO = "'Print Mono', 'DejaVu Sans Mono', monospace"
const FONT_MOUNT = '/__print-fonts/'
const GOOGLE_FONTS = 'https://raw.githubusercontent.com/google/fonts/main/ofl/'
const D2CODING = 'https://raw.githubusercontent.com/naver/d2codingfont/VER1.4.0/fonts/ttf/'
const FONTS = [
  ['Print Serif', 400, `${GOOGLE_FONTS}nanummyeongjo/NanumMyeongjo-Regular.ttf`],
  ['Print Serif', 700, `${GOOGLE_FONTS}nanummyeongjo/NanumMyeongjo-Bold.ttf`],
  ['Print Sans', 400, `${GOOGLE_FONTS}nanumgothic/NanumGothic-Regular.ttf`],
  ['Print Sans', 700, `${GOOGLE_FONTS}nanumgothic/NanumGothic-Bold.ttf`],
  ['Print Mono', 400, `${D2CODING}D2Coding-Regular.ttf`],
  ['Print Mono', 700, `${D2CODING}D2Coding-Bold.ttf`],
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

export const FONT_FACES = FONTS.map(
  ([family, weight, url]) =>
    `@font-face { font-family: '${family}'; font-weight: ${weight}; src: url('${FONT_MOUNT}${fontFile(url)}') format('truetype'); }`,
).join('\n')

// 화면용 HTML 조각(차례 등)이 부르는 웹 글꼴을 인쇄 글꼴로 바꾼다
export const localFonts = (html) =>
  html
    .replace('<style>', `<style>\n${FONT_FACES}`)
    .replaceAll("'Noto Serif KR', serif", SERIF)
    .replaceAll("'Noto Sans KR', sans-serif", SANS)

// 본문 쪽에 덧입히는 인쇄 조판. t는 글자 크기(pt)와 그림 크기(mm)다.
export function printCss(t) {
  return `
${FONT_FACES}
:root {
  --fc-font-serif: ${SERIF} !important;
  --vp-font-family-base: ${SANS} !important;
  --vp-font-family-mono: ${MONO} !important;
}
html { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
.vp-doc { color: #000 !important; }
.vp-doc p, .vp-doc li { font-size: ${t.body}pt !important; line-height: ${t.lineHeight} !important; }
.vp-doc p { margin: 0 0 0.75em !important; }
.vp-doc li + li { margin-top: 0.15em !important; }
.vp-doc ul, .vp-doc ol { margin: 0.4em 0 0.8em !important; }
.vp-doc h1 {
  font-size: ${t.h1}pt !important; line-height: 1.4 !important;
  margin: 0 0 9mm !important; padding: 0 0 3mm !important; border-bottom: 0.6pt solid #000 !important;
}
.vp-doc h2 {
  font-size: ${t.h2}pt !important; line-height: 1.45 !important;
  margin: 7mm 0 3mm !important; padding-top: 0 !important; border-top: none !important;
}
.vp-doc h3 { font-size: ${t.h3}pt !important; line-height: 1.45 !important; margin: 5mm 0 2mm !important; }
.vp-doc h4 { font-size: ${t.body}pt !important; margin: 4mm 0 1.5mm !important; }
.vp-doc a { color: inherit !important; text-decoration: none !important; }
.vp-doc :not(pre) > code {
  font-family: ${MONO} !important; font-size: ${t.inlineCode}pt !important;
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
  font-family: ${MONO} !important; font-size: ${t.code}pt !important;
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
.vp-doc .custom-block p { font-size: ${t.box}pt !important; line-height: 1.6 !important; margin: 0 0 0.4em !important; }
.vp-doc .custom-block .custom-block-title { font-size: ${t.box}pt !important; margin-bottom: 1mm !important; }

/* 표 */
.vp-doc table { display: table !important; width: 100% !important; margin: 3mm 0 4mm !important; border-collapse: collapse !important; }
.vp-doc th, .vp-doc td {
  font-size: ${t.small}pt !important; line-height: 1.5 !important; padding: 1.2mm 2mm !important;
  border: 0.5pt solid #999 !important; background: none !important; color: #000 !important;
}
.vp-doc tr { background: none !important; break-inside: avoid; }
.vp-doc th { font-weight: 700 !important; background: #ececec !important; }

/* 그림과 캡션 */
.vp-doc .fc-shots { margin: 4mm 0 1mm !important; }
.vp-doc .fc-shots img { max-height: ${t.imageMaxHeight}mm; border: 0.5pt solid #999 !important; border-radius: 0 !important; }
.vp-doc .fc-shots.single img { max-width: ${t.phoneImageWidth}mm !important; }
.vp-doc .fc-caption { font-size: ${t.small}pt !important; line-height: 1.5 !important; color: #333 !important; margin: 1.5mm 0 5mm !important; }

/* 인용과 각주 */
.vp-doc blockquote { margin: 3mm 0 !important; padding-left: 4mm !important; border-left: 1.5pt solid #999 !important; }
.vp-doc blockquote p { font-size: ${t.body - 0.5}pt !important; color: #222 !important; }
.vp-doc .footnotes-sep { margin: 6mm 0 2mm !important; }
.vp-doc .footnotes p, .vp-doc .footnotes li { font-size: ${t.small}pt !important; line-height: 1.5 !important; }
`
}

// 쪽 한 장짜리 HTML(표제지·판권·쪽 번호)의 공통 바탕
export const PAGE_CSS = (w, h) => `
  ${FONT_FACES}
  @page { size: ${w}mm ${h}mm; margin: 0; }
  html, body { margin: 0; padding: 0; }
  body { font-family: ${SERIF}; color: #000; }
  .sheet { box-sizing: border-box; width: ${w}mm; height: ${h}mm; break-after: page; position: relative; }
`

// 본문 쪽 번호만 찍힌 쪽들. 병합할 때 본문 위에 겹친다.
// align: 'outer'(본문 1쪽이 오른쪽 면이라 홀수는 오른쪽, 짝수는 왼쪽 모서리) 또는 'center'
export function pageNumbersHtml(count, w, h, { bottom, outer, align }) {
  const sheets = Array.from({ length: count }, (_, i) => {
    const n = i + 1
    const pos =
      align === 'center'
        ? 'left: 0; right: 0; text-align: center'
        : n % 2 === 1
          ? `right: ${outer}mm`
          : `left: ${outer}mm`
    return `<div class="sheet"><span style="${pos}">${n}</span></div>`
  }).join('')
  return `<!doctype html><html lang="ko"><head><meta charset="utf-8">
  <style>${PAGE_CSS(w, h)}
    .sheet span { position: absolute; bottom: ${bottom / 2 - 1.5}mm; font-family: ${SANS}; font-size: 8pt; }
  </style></head><body>${sheets}</body></html>`
}

// 빌드 결과를 서빙하고 Chrome 한 페이지를 연다. 웹 글꼴은 막고 인쇄 글꼴을 붙여 준다.
// fn(printer)이 끝나면 브라우저와 서버를 닫는다.
//   printer.chapters(chapters, css, pdfOpts) → 장마다 인쇄한 PDFDocument 목록
//   printer.html(html, pdfOpts)              → HTML 한 벌을 인쇄한 PDF 바이트
export async function withPrinter(root, book, fn) {
  const fontDir = join(root, '.fonts-cache')
  await ensureFonts(fontDir)
  const { port, close } = await serveDist(join(root, '.vitepress/dist'), book.base, {
    [FONT_MOUNT]: fontDir,
  })
  const browser = await puppeteer.launch({
    executablePath: findChrome(),
    args: ['--no-sandbox', '--font-render-hinting=none'],
  })
  const loadFonts = () => Promise.allSettled([...document.fonts].map((f) => f.load()))
  try {
    const page = await browser.newPage()
    await page.setRequestInterception(true)
    page.on('request', (req) =>
      /fonts\.(googleapis|gstatic)\.com/.test(req.url()) ? req.abort() : req.continue(),
    )
    // setContent는 마지막으로 연 주소를 기준으로 삼는다. 글꼴·표지 그림이 로컬 서버에서 오도록 먼저 연다.
    await page.goto(`http://127.0.0.1:${port}${book.base}`, { waitUntil: 'domcontentloaded' })
    return await fn({
      async chapters(chapters, css, pdfOpts) {
        const docs = []
        for (const { route } of chapters) {
          await page.goto(`http://127.0.0.1:${port}${book.base}${route}`, {
            waitUntil: 'networkidle0',
            timeout: 90_000,
          })
          await page.addStyleTag({ content: css })
          await page.evaluate(loadFonts)
          docs.push(await PDFDocument.load(await page.pdf(pdfOpts)))
          console.log(`printed: ${route}`)
        }
        return docs
      },
      async html(html, pdfOpts) {
        await page.setContent(html, { waitUntil: 'load', timeout: 60_000 })
        await page.evaluate(loadFonts)
        return page.pdf(pdfOpts)
      },
    })
  } finally {
    await browser.close()
    close()
  }
}

// 장별 PDF의 쪽수로 본문 기준 시작 쪽(1부터)을 구한다. 차례에 쓴다.
export function startPages(docs) {
  const starts = []
  let cursor = 1
  for (const doc of docs) {
    starts.push(cursor)
    cursor += doc.getPageCount()
  }
  return starts
}

// 쪽 번호 PDF를 본문 쪽들 위에 겹친다. offset은 재단 여유(pt)만큼 안쪽으로 옮길 때 쓴다.
export async function stampNumbers(out, numbersBuf, bodyStart, offset = 0) {
  const numbers = await PDFDocument.load(numbersBuf)
  const pages = await out.embedPdf(numbers, numbers.getPageIndices())
  pages.forEach((np, k) => out.getPage(bodyStart + k).drawPage(np, { x: offset, y: offset }))
}
