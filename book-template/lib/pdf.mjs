// 빌드된 사이트(.vitepress/dist)를 로컬로 서빙하고, 표지·차례와 함께
// 장 순서대로 인쇄해 한 권의 PDF로 합친다.
// 북마크(PDF 아웃라인)와 연속 쪽 번호를 넣는다. 시스템 Chrome을 사용한다.
import { writeFile } from 'node:fs/promises'
import { join } from 'node:path'
import puppeteer from 'puppeteer-core'
import { PDFDocument, PDFHexString, PDFName, PDFNumber, StandardFonts, rgb } from 'pdf-lib'
import { pdfCoverHtml, pdfTocHtml } from './cover.mjs'
import { findChrome, serveDist } from './server.mjs'
import { flattenChapters } from './toc.mjs'

// 북마크(PDF 아웃라인)를 한 단계로 단다. items: [{ title, pageIndex }]
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
  const dist = join(root, '.vitepress/dist')
  const chapters = flattenChapters(book)
  const { port, close } = await serveDist(dist, book.base)

  const CONTENT_MARGIN = { top: '18mm', bottom: '18mm', left: '15mm', right: '15mm' }
  const browser = await puppeteer.launch({
    executablePath: findChrome(),
    args: ['--no-sandbox', '--font-render-hinting=none'],
  })

  let coverBuf, tocBuf
  const chapterDocs = []
  try {
    const page = await browser.newPage()

    // 본문 장들
    for (const { route } of chapters) {
      await page.goto(`http://127.0.0.1:${port}${book.base}${route}`, {
        waitUntil: 'networkidle0',
        timeout: 90_000,
      })
      await page.evaluateHandle('document.fonts.ready')
      const buf = await page.pdf({ format: 'A4', printBackground: true, margin: CONTENT_MARGIN })
      chapterDocs.push(await PDFDocument.load(buf))
      console.log(`printed: ${route}`)
    }

    // 장별 시작 쪽 번호(본문 기준 1부터)를 계산해 차례를 만든다
    const startPages = []
    let cursor = 1
    for (const doc of chapterDocs) {
      startPages.push(cursor)
      cursor += doc.getPageCount()
    }

    await page.setContent(pdfTocHtml(chapters, startPages), { waitUntil: 'load', timeout: 60_000 })
    await page.evaluate(() => document.fonts.ready)
    tocBuf = await page.pdf({ format: 'A4', printBackground: true, margin: CONTENT_MARGIN })

    await page.setContent(pdfCoverHtml(book), { waitUntil: 'load', timeout: 60_000 })
    await page.evaluate(() => document.fonts.ready)
    coverBuf = await page.pdf({ format: 'A4', printBackground: true, margin: 0, pageRanges: '1' })
  } finally {
    await browser.close()
    close()
  }

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

  // 본문에만 연속 쪽 번호를 찍는다 (표지·차례 제외)
  const font = await merged.embedFont(StandardFonts.Helvetica)
  const contentTotal = merged.getPageCount() - frontPages
  merged.getPages().forEach((p, i) => {
    if (i < frontPages) return
    const label = `${i - frontPages + 1} / ${contentTotal}`
    const width = font.widthOfTextAtSize(label, 9)
    p.drawText(label, {
      x: (p.getSize().width - width) / 2,
      y: 24,
      size: 9,
      font,
      color: rgb(0.45, 0.45, 0.45),
    })
  })

  addOutline(merged, [
    { title: '차례', pageIndex: coverPages },
    ...chapters.map((c, i) => ({ title: c.title, pageIndex: chapterStartIndex[i] })),
  ])

  const out = join(dist, book.pdf.fileName)
  await writeFile(out, await merged.save())
  console.log(`PDF 생성 완료: ${out} (표지 ${coverPages} + 차례 ${tocPages} + 본문 ${contentTotal}쪽)`)
}
