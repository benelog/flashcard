// 직접 프린트용 A4 PDF. 집이나 회사의 프린터로 뽑아 읽는 독자를 위한 판이다.
// 빌드된 사이트를 장 순서대로 인쇄 조판(print.mjs)으로 찍고, 표지·차례와 함께 한 권으로 합친다.
// 프린트해서 직접 묶는 책이라 홀짝 구분 없이 좌우 여백을 같게 두고 쪽 번호는 아래 가운데에 찍는다.
// 북마크(PDF 아웃라인)를 넣어 화면에서 넘겨 보기에도 쓸 수 있다. 시스템 Chrome을 사용한다.
import { writeFile } from 'node:fs/promises'
import { join } from 'node:path'
import { PDFDocument, PDFHexString, PDFName, PDFNumber } from 'pdf-lib'
import { pdfCoverHtml, pdfTocHtml } from './cover.mjs'
import { localFonts, pageNumbersHtml, printCss, stampNumbers, startPages, withPrinter } from './print.mjs'
import { flattenChapters } from './toc.mjs'

// A4는 B5 POD 판보다 넓어서 본문을 한 단계 키워도 쪽수가 늘지 않는다. 일반 책 본문 크기(11pt)에
// 펜으로 메모할 행간을 남긴다. 그림은 mm.
const TYPE = {
  body: 11,
  lineHeight: 1.7,
  h1: 20,
  h2: 14,
  h3: 12,
  code: 9,
  inlineCode: 9.5,
  box: 9.5,
  small: 9,
  imageMaxHeight: 180,
  phoneImageWidth: 70,
}
// 사무용·가정용 프린터는 가장자리 3~5mm를 찍지 못한다. 펀치 구멍이나 클립 자리도 남긴다.
const MARGIN = { top: 20, bottom: 22, side: 20 }

export function addOutline(doc, items) {
  const ctx = doc.context
  const rootRef = ctx.nextRef()
  const itemRefs = items.map(() => ctx.nextRef())
  items.forEach((item, i) => {
    const dict = ctx.obj({})
    dict.set(PDFName.of('Title'), PDFHexString.fromText(item.title))
    dict.set(PDFName.of('Parent'), rootRef)
    dict.set(PDFName.of('Dest'), ctx.obj([doc.getPage(item.pageIndex).ref, PDFName.of('Fit')]))
    if (i > 0) dict.set(PDFName.of('Prev'), itemRefs[i - 1])
    if (i < itemRefs.length - 1) dict.set(PDFName.of('Next'), itemRefs[i + 1])
    ctx.assign(itemRefs[i], dict)
  })
  const outlineRoot = ctx.obj({})
  outlineRoot.set(PDFName.of('Type'), PDFName.of('Outlines'))
  outlineRoot.set(PDFName.of('First'), itemRefs[0])
  outlineRoot.set(PDFName.of('Last'), itemRefs[itemRefs.length - 1])
  outlineRoot.set(PDFName.of('Count'), PDFNumber.of(items.length))
  ctx.assign(rootRef, outlineRoot)
  doc.catalog.set(PDFName.of('Outlines'), rootRef)
}

export async function exportPdf(root, book) {
  const chapters = flattenChapters(book)
  const bodyMargin = {
    top: `${MARGIN.top}mm`,
    bottom: `${MARGIN.bottom}mm`,
    left: `${MARGIN.side}mm`,
    right: `${MARGIN.side}mm`,
  }
  const a4 = (margin) => ({ format: 'A4', printBackground: true, margin })
  const none = { top: 0, bottom: 0, left: 0, right: 0 }

  const { chapterDocs, coverBuf, tocBuf, numbersBuf } = await withPrinter(
    root,
    book,
    async (printer) => {
      const chapterDocs = await printer.chapters(chapters, printCss(TYPE), a4(bodyMargin))
      const bodyPages = chapterDocs.reduce((n, d) => n + d.getPageCount(), 0)
      return {
        chapterDocs,
        tocBuf: await printer.html(
          localFonts(pdfTocHtml(chapters, startPages(chapterDocs))),
          a4(bodyMargin),
        ),
        coverBuf: await printer.html(pdfCoverHtml(book), { ...a4(none), pageRanges: '1' }),
        numbersBuf: await printer.html(
          pageNumbersHtml(bodyPages, 210, 297, { bottom: MARGIN.bottom, align: 'center' }),
          a4(none),
        ),
      }
    },
  )

  // ── 병합: 표지 + 차례 + 본문 ────────────────────────────────────
  const merged = await PDFDocument.create()
  merged.setTitle(book.title)
  merged.setAuthor(book.author)
  merged.setSubject(book.subtitle)
  merged.setLanguage(book.lang ?? 'ko-KR')

  async function append(src) {
    const pages = await merged.copyPages(src, src.getPageIndices())
    pages.forEach((p) => merged.addPage(p))
    return pages.length
  }

  const coverPages = await append(await PDFDocument.load(coverBuf))
  const tocPages = await append(await PDFDocument.load(tocBuf))
  const frontPages = coverPages + tocPages

  const chapterStartIndex = [] // 병합본에서 각 장의 0-기준 페이지 인덱스
  for (const doc of chapterDocs) {
    chapterStartIndex.push(merged.getPageCount())
    await append(doc)
  }
  const contentTotal = merged.getPageCount() - frontPages
  await stampNumbers(merged, numbersBuf, frontPages)

  addOutline(merged, [
    { title: '차례', pageIndex: coverPages },
    ...chapters.map((c, i) => ({ title: c.title, pageIndex: chapterStartIndex[i] })),
  ])

  const out = join(root, '.vitepress/dist', book.pdf.fileName)
  await writeFile(out, await merged.save())
  console.log(`PDF 생성 완료: ${out} (표지 ${coverPages} + 차례 ${tocPages} + 본문 ${contentTotal}쪽)`)
}
