// 주문형 인쇄(POD)에 올릴 본문 PDF. 인쇄 조판(print.mjs)을 B5 종이책 크기로 덧입히고
// 홀짝 쪽 여백을 거울처럼 나눈다. 표지는 POD 업체에 따로 올리므로 넣지 않는다.
// 구성: 표제지 · 판권 · 차례 · 본문. 결과는 .pod/에 둔다(배포하지 않는다).
//
// 교보 바로출판 원고 규정(필수체크 안내, 2026.10)을 따른다.
// - 재단 여유: 책 크기에 상하좌우 3mm씩 더한 쪽 크기로 만들고, 글과 그림은 책 크기 안에 둔다.
// - PDF 1쪽이 책의 오른쪽 면이다. 홀수 쪽은 모두 오른쪽 면이 된다.
// - 흑백 인쇄를 고르면 원고 PDF도 그레이스케일이어야 한다(색이 남으면 흐릿하게 찍힐 수 있다).
//   병합한 뒤 Ghostscript(gs)로 PDF 전체를 회색조로 바꾼다.
// - 판권 페이지가 반드시 있어야 한다. 항목은 book.config의 pod.colophon에 둔다.
//
// 홀짝 여백: Chrome은 장마다 따로 인쇄하므로 쪽의 홀짝을 알 수 없다. 그래서 좌우 여백을
// (안쪽+바깥쪽)/2로 같게 인쇄한 뒤, 병합할 때 홀수 쪽(오른쪽 면)은 오른쪽으로, 짝수 쪽은
// 왼쪽으로 (안쪽−바깥쪽)/2만큼 옮겨 안쪽 여백을 넓힌다.
import { execFile } from 'node:child_process'
import { mkdir, rm, writeFile } from 'node:fs/promises'
import { join } from 'node:path'
import { promisify } from 'node:util'
import { PDFDocument } from 'pdf-lib'
import { pdfTocHtml } from './cover.mjs'
import { addOutline } from './pdf.mjs'
import {
  MM,
  PAGE_CSS,
  SANS,
  localFonts,
  pageNumbersHtml,
  printCss,
  stampNumbers,
  startPages,
  withPrinter,
} from './print.mjs'
import { flattenChapters } from './toc.mjs'

// 종이책 본문 크기(pt). 그림은 mm.
const TYPE = {
  body: 10,
  lineHeight: 1.7,
  h1: 18,
  h2: 13,
  h3: 11,
  code: 8,
  inlineCode: 9,
  box: 9,
  small: 8.5,
  imageMaxHeight: 150,
  phoneImageWidth: 62,
}

// 기본값은 교보 바로출판 POD의 B5(46배판) 188×254mm다. book.config의 pod로 바꿀 수 있다.
const DEFAULTS = {
  fileName: 'book-pod.pdf',
  width: 188,
  height: 254,
  margin: { top: 20, bottom: 22, inner: 22, outer: 16 },
  bleed: 3, // 재단 여유(mm), 상하좌우 각각
  grayscale: true,
}

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

// 판권: 표제지 뒷면(왼쪽 면) 아래쪽에 서지 정보를 모은다.
// pod.colophon.groups는 [항목, 값] 묶음의 목록이고 묶음 사이를 띄운다. notes는 그 아래 문단들이다.
// 없으면 지은이와 라이선스만 적는다. ISBN을 받으면 pod.isbn에 넣어 마지막 묶음에 붙인다.
function copyrightPageHtml(book, pod, w, h) {
  const colophon = pod.colophon ?? {
    groups: [[['지은이', book.author]]],
    notes: book.cover?.licenseHtml ? [book.cover.licenseHtml] : [],
  }
  const groups = colophon.groups.map((g, i) =>
    i === colophon.groups.length - 1 && pod.isbn ? [...g, ['ISBN', pod.isbn]] : g,
  )
  const tables = groups
    .map((g) => `<table>${g.map(([k, v]) => `<tr><td>${k}</td><td>${v}</td></tr>`).join('')}</table>`)
    .join('')
  const notes = (colophon.notes ?? []).map((n) => `<p class="note">${n}</p>`).join('')
  return `<!doctype html><html lang="ko"><head><meta charset="utf-8">
  <style>${PAGE_CSS(w, h)}
    .sheet { padding: 0 22mm 30mm 16mm; display: flex; flex-direction: column; justify-content: flex-end; }
    .t { font-family: ${SANS}; font-size: 11pt; font-weight: 700; margin: 0 0 4mm; }
    table { border-collapse: collapse; font-size: 8.5pt; line-height: 1.8; margin: 0 0 4mm; }
    td:first-child { width: 16mm; padding-right: 4mm; color: #444; white-space: nowrap; }
    .note { font-size: 8pt; line-height: 1.6; margin: 0 0 1.5mm; word-break: keep-all; }
    .note a { color: inherit; text-decoration: none; }
  </style></head><body>
  <div class="sheet"><p class="t">${book.title}</p>${tables}${notes}</div>
  </body></html>`
}

// Ghostscript로 PDF 전체(글자·바탕·그림)를 회색조로 바꾼다. 그림은 해상도를 낮추지 않는다.
async function toGrayscale(src, dest) {
  try {
    await promisify(execFile)('gs', [
      '-q',
      '-dNOPAUSE',
      '-dBATCH',
      '-dSAFER',
      '-sDEVICE=pdfwrite',
      '-sColorConversionStrategy=Gray',
      '-dProcessColorModel=/DeviceGray',
      '-dAutoRotatePages=/None',
      '-dDownsampleColorImages=false',
      '-dDownsampleGrayImages=false',
      '-dDownsampleMonoImages=false',
      `-sOutputFile=${dest}`,
      src,
    ])
  } catch (e) {
    if (e.code === 'ENOENT') {
      throw new Error('그레이스케일 변환에 Ghostscript(gs)가 필요하다. 설치하거나 pod.grayscale을 false로 둔다.')
    }
    throw e
  }
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

  const chapters = flattenChapters(book)
  const none = { top: 0, bottom: 0, left: 0, right: 0 }
  const { chapterDocs, titleBuf, copyrightBuf, tocBuf, numbersBuf } = await withPrinter(
    root,
    book,
    async (printer) => {
      const chapterDocs = await printer.chapters(chapters, printCss(TYPE), pdfOpts(bodyMargin))
      const starts = startPages(chapterDocs)
      const bodyPages = chapterDocs.reduce((n, d) => n + d.getPageCount(), 0)
      return {
        chapterDocs,
        titleBuf: await printer.html(titlePageHtml(book, w, h), pdfOpts(none)),
        copyrightBuf: await printer.html(copyrightPageHtml(book, pod, w, h), pdfOpts(none)),
        tocBuf: await printer.html(localFonts(pdfTocHtml(chapters, starts)), pdfOpts(bodyMargin)),
        numbersBuf: await printer.html(
          pageNumbersHtml(bodyPages, w, h, { bottom: m.bottom, outer: m.outer, align: 'outer' }),
          pdfOpts(none),
        ),
      }
    },
  )

  // ── 병합: 쪽마다 홀짝에 맞춰 옮겨 붙인다 ──────────────────────
  const out = await PDFDocument.create()
  out.setTitle(book.title)
  out.setAuthor(book.author)
  out.setSubject(book.subtitle)
  out.setLanguage(book.lang ?? 'ko-KR')
  // 쪽은 책 크기에 재단 여유를 더한 크기다. 인쇄한 쪽(책 크기)은 여유만큼 안쪽에 놓는다.
  const bleed = pod.bleed * MM
  const pageW = w * MM + 2 * bleed
  const pageH = h * MM + 2 * bleed
  const shift = ((m.inner - m.outer) / 2) * MM

  const addPage = () => {
    const p = out.addPage([pageW, pageH])
    p.setBleedBox(0, 0, pageW, pageH)
    p.setTrimBox(bleed, bleed, w * MM, h * MM)
    return p
  }
  const blank = () => addPage()
  // 이미 거울 여백을 직접 잡은 쪽(표제지·판권)은 옮기지 않는다
  async function place(srcBytes, { mirror = true } = {}) {
    const src = srcBytes instanceof PDFDocument ? srcBytes : await PDFDocument.load(srcBytes)
    const embedded = await out.embedPdf(src, src.getPageIndices())
    const first = out.getPageCount()
    for (const e of embedded) {
      const recto = out.getPageCount() % 2 === 0 // 0부터 세므로 짝수 인덱스가 오른쪽 면
      addPage().drawPage(e, { x: bleed + (mirror ? (recto ? shift : -shift) : 0), y: bleed })
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

  await stampNumbers(out, numbersBuf, bodyStart, bleed)

  // 제본은 낱장(2쪽) 단위라 전체 쪽수를 짝수로 맞춘다
  if (out.getPageCount() % 2 === 1) blank()

  addOutline(out, [
    { title: '차례', pageIndex: tocIndex },
    ...chapters.map((c, i) => ({ title: c.title, pageIndex: chapterStartIndex[i] })),
  ])

  const outDir = join(root, '.pod')
  await mkdir(outDir, { recursive: true })
  const file = join(outDir, podFileName(book))
  if (pod.grayscale) {
    const color = join(outDir, 'color.pdf')
    await writeFile(color, await out.save())
    await toGrayscale(color, file)
    await rm(color)
  } else {
    await writeFile(file, await out.save())
  }
  console.log(
    `POD PDF 생성 완료: ${file} (책 ${w}×${h}mm + 재단 여유 ${pod.bleed}mm, ${pod.grayscale ? '그레이스케일' : '컬러'}, 전체 ${out.getPageCount()}쪽 = 앞부분 ${bodyStart} + 본문 ${bodyPages} + 끝 빈 쪽 ${out.getPageCount() - bodyStart - bodyPages})`,
  )
}
