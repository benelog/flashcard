// 빌드된 사이트(.vitepress/dist)에서 장마다 본문만 뽑아 EPUB 3 한 권으로 묶는다.
// PDF처럼 로컬 서버와 시스템 Chrome을 쓰되, 자바스크립트를 끄고 SSR 결과 그대로 읽는다
// (이북 뷰어 테마가 DOM을 바꾸기 전의 깨끗한 본문이다).
// 표지는 PDF 표지 HTML을 그대로 찍은 PNG다. 글꼴은 넣지 않고 리더의 글꼴을 따른다.
import { readFile, writeFile } from 'node:fs/promises'
import { createHash } from 'node:crypto'
import { extname, join } from 'node:path'
import { deflateRawSync } from 'node:zlib'
import puppeteer from 'puppeteer-core'
import { pdfCoverHtml } from './cover.mjs'
import { findChrome, serveDist } from './server.mjs'
import { epubFileName, flattenChapters } from './toc.mjs'

// 장 경로(part3/htmx) → EPUB 안의 파일 이름(part3-htmx.xhtml)
const xhtmlName = (route) => route.replaceAll('/', '-') + '.xhtml'

const esc = (s) =>
  String(s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c])

const IMAGE_TYPES = {
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.gif': 'image/gif',
  '.svg': 'image/svg+xml',
  '.webp': 'image/webp',
}

// 브라우저 안에서 실행한다: 본문을 정리해 XHTML 문자열과 절 제목·그림 목록을 돌려준다.
function extractChapter({ base, routes, site }) {
  const root = document.querySelector('.vp-doc > div')
  if (!root) throw new Error('본문(.vp-doc)을 찾지 못했다')

  // 화면 전용 장식: 제목 옆 앵커, 코드 복사 버튼, 언어 라벨
  root.querySelectorAll('.header-anchor, button.copy, span.lang').forEach((el) => el.remove())
  root.querySelectorAll('[tabindex]').forEach((el) => el.removeAttribute('tabindex'))
  // 앵커를 지운 자리에 남은 제목 끝 공백
  root.querySelectorAll('h1, h2, h3, h4').forEach((h) => {
    if (h.lastChild?.nodeType === Node.TEXT_NODE) h.lastChild.textContent = h.lastChild.textContent.trimEnd()
  })

  // shiki는 밝은·어두운 테마 색을 CSS 변수로만 준다. 리더는 변수를 모르니 밝은 색을 직접 칠한다.
  // 코드 상자 배경은 style.css가 밝게 고정한다(밤 모드에서도 글자색과 대비가 유지된다).
  root.querySelectorAll('pre.shiki, pre.shiki span[style]').forEach((el) => {
    const light = el.style.getPropertyValue('--shiki-light')
    el.removeAttribute('style')
    if (light) el.style.color = light
  })
  root.querySelectorAll('[class*="language-"]').forEach((el) => {
    const lang = [...el.classList].find((c) => c.startsWith('language-'))
    el.className = 'code-block'
    if (lang && lang !== 'language-') el.dataset.lang = lang.slice('language-'.length)
  })
  root.querySelectorAll('pre.shiki').forEach((el) => (el.className = 'code'))

  // 각주: 리더가 팝업으로 띄울 수 있게 EPUB 의미 표시를 단다
  root.querySelectorAll('sup.footnote-ref > a').forEach((a) => a.setAttribute('epub:type', 'noteref'))
  root.querySelectorAll('section.footnotes').forEach((s) => s.setAttribute('epub:type', 'footnotes'))
  root.querySelectorAll('li.footnote-item').forEach((li) => li.setAttribute('epub:type', 'footnote'))

  // 그림: 사이트 절대 경로를 EPUB 안의 images/로 옮긴다
  const images = []
  root.querySelectorAll('img').forEach((img) => {
    const src = img.getAttribute('src') ?? ''
    if (!src.startsWith(base)) return
    const path = src.slice(base.length)
    const name = path.replaceAll('/', '-')
    images.push({ path, name })
    img.setAttribute('src', `images/${name}`)
    if (!img.hasAttribute('alt')) img.setAttribute('alt', '')
  })

  // 링크: 다른 장은 EPUB 안의 파일로, 그 밖의 사이트 내부 주소는 공개 사이트 주소로
  root.querySelectorAll('a[href]').forEach((a) => {
    const href = a.getAttribute('href')
    if (!href.startsWith(base)) return
    const [path, hash] = href.slice(base.length).split('#')
    const route = path.replace(/\.html$/, '')
    if (routes.includes(route)) {
      a.setAttribute('href', route.replaceAll('/', '-') + '.xhtml' + (hash ? '#' + hash : ''))
    } else {
      a.setAttribute('href', site + href.slice(base.length))
    }
  })

  // Vue가 남긴 흔적(data-v-*, 빈 주석)
  root.querySelectorAll('*').forEach((el) => {
    for (const { name } of [...el.attributes]) if (name.startsWith('data-v-')) el.removeAttribute(name)
  })
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_COMMENT)
  const comments = []
  while (walker.nextNode()) comments.push(walker.currentNode)
  comments.forEach((c) => c.remove())

  const title = root.querySelector('h1')?.textContent.trim() ?? document.title
  const sections = [...root.querySelectorAll('h2[id]')].map((h) => ({
    id: h.id,
    text: h.textContent.trim(),
  }))
  // 감싼 div 하나를 벗긴 내용만 XHTML로 직렬화한다
  const body = [...root.childNodes].map((n) => new XMLSerializer().serializeToString(n)).join('')
  return { title, sections, images, body }
}

function chapterXhtml(book, title, body) {
  // 자식마다 붙은 기본 이름공간 선언은 html 요소에 한 번이면 된다
  const clean = body.replaceAll(' xmlns="http://www.w3.org/1999/xhtml"', '')
  return `<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="${book.lang}" lang="${book.lang}">
<head>
<meta charset="utf-8"/>
<title>${esc(title)}</title>
<link rel="stylesheet" type="text/css" href="style.css"/>
</head>
<body>
<section epub:type="chapter">
${clean}
</section>
</body>
</html>
`
}

function coverXhtml(book) {
  return `<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="${book.lang}" lang="${book.lang}">
<head>
<meta charset="utf-8"/>
<title>${esc(book.title)}</title>
<style>html, body { margin: 0; padding: 0; height: 100%; } body { text-align: center; } img { max-width: 100%; max-height: 100%; }</style>
</head>
<body epub:type="cover">
<img src="images/cover.png" alt="${esc(book.title)} 표지"/>
</body>
</html>
`
}

// 판권 쪽: 제목·지은이·사이트·라이선스. licenseHtml은 홈 표지와 같은 문장이다.
function colophonXhtml(book, modified) {
  const license = book.cover?.licenseHtml
    ? `<p class="license">${book.cover.licenseHtml.replace(/ target="_blank"/g, '')}</p>`
    : ''
  return `<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="${book.lang}" lang="${book.lang}">
<head>
<meta charset="utf-8"/>
<title>판권</title>
<link rel="stylesheet" type="text/css" href="style.css"/>
</head>
<body>
<section class="colophon" epub:type="copyright-page">
<h1 class="book-title">${esc(book.title)}</h1>
${book.subtitle ? `<p class="book-subtitle">${esc(book.subtitle)}</p>` : ''}
<p>지은이 ${esc(book.author)}</p>
<p>웹으로 읽기 <a href="${esc(book.site)}">${esc(book.siteLabel ?? book.site)}</a></p>
<p>이 파일을 만든 날 ${modified.slice(0, 10)}</p>
${license}
</section>
</body>
</html>
`
}

// 차례(EPUB 3 nav): 부 → 장 → 절(h2). 장이 하나뿐인 그룹(서문)은 장을 바로 올린다.
function navXhtml(book, chapters) {
  const byFile = new Map(chapters.map((c) => [c.file, c]))
  const chapterLi = (c, indent) => {
    const href = xhtmlName(c.route)
    const subs = c.sections.length
      ? `\n${indent}  <ol>\n${c.sections
          .map((s) => `${indent}    <li><a href="${href}#${esc(s.id)}">${esc(s.text)}</a></li>`)
          .join('\n')}\n${indent}  </ol>\n${indent}`
      : ''
    return `${indent}<li><a href="${href}">${esc(c.navTitle)}</a>${subs}</li>`
  }
  const items = book.toc
    .map((group) => {
      const cs = group.items.map((i) => byFile.get(i.file))
      if (cs.length === 1) return chapterLi(cs[0], '      ')
      return `      <li><a href="${xhtmlName(cs[0].route)}">${esc(group.text)}</a>
        <ol>
${cs.map((c) => chapterLi(c, '          ')).join('\n')}
        </ol>
      </li>`
    })
    .join('\n')
  return `<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="${book.lang}" lang="${book.lang}">
<head>
<meta charset="utf-8"/>
<title>차례</title>
<link rel="stylesheet" type="text/css" href="style.css"/>
</head>
<body>
<nav epub:type="toc" id="toc">
  <h1>차례</h1>
  <ol>
${items}
  </ol>
</nav>
<nav epub:type="landmarks" hidden="">
  <ol>
    <li><a epub:type="cover" href="cover.xhtml">표지</a></li>
    <li><a epub:type="toc" href="nav.xhtml">차례</a></li>
    <li><a epub:type="bodymatter" href="${xhtmlName(chapters[0].route)}">본문</a></li>
  </ol>
</nav>
</body>
</html>
`
}

// EPUB 2 리더를 위한 NCX 차례(부·장 두 단계만)
function tocNcx(book, chapters, uid) {
  // playOrder는 가리키는 파일마다 하나다. 부와 그 첫 장은 같은 파일이라 같은 번호를 쓴다.
  let id = 0
  const point = (label, route, children = '') =>
    `<navPoint id="np${++id}" playOrder="${routes.indexOf(route) + 1}"><navLabel><text>${esc(label)}</text></navLabel><content src="${xhtmlName(route)}"/>${children}</navPoint>`
  const routes = chapters.map((c) => c.route)
  const byFile = new Map(chapters.map((c) => [c.file, c]))
  const points = book.toc
    .map((group) => {
      const cs = group.items.map((i) => byFile.get(i.file))
      if (cs.length === 1) return point(cs[0].navTitle, cs[0].route)
      const children = cs.map((c) => point(c.navTitle, c.route)).join('')
      return point(group.text, cs[0].route, children)
    })
    .join('\n')
  return `<?xml version="1.0" encoding="utf-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1" xml:lang="${book.lang}">
<head>
<meta name="dtb:uid" content="${esc(uid)}"/>
<meta name="dtb:depth" content="2"/>
<meta name="dtb:totalPageCount" content="0"/>
<meta name="dtb:maxPageNumber" content="0"/>
</head>
<docTitle><text>${esc(book.title)}</text></docTitle>
<navMap>
${points}
</navMap>
</ncx>
`
}

function contentOpf(book, { id, modified, chapters, images }) {
  const manifest = [
    '<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>',
    '<item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>',
    '<item id="css" href="style.css" media-type="text/css"/>',
    '<item id="cover-image" href="images/cover.png" media-type="image/png" properties="cover-image"/>',
    '<item id="cover" href="cover.xhtml" media-type="application/xhtml+xml"/>',
    '<item id="colophon" href="colophon.xhtml" media-type="application/xhtml+xml"/>',
    ...chapters.map(
      (c, i) => `<item id="ch${i + 1}" href="${xhtmlName(c.route)}" media-type="application/xhtml+xml"/>`,
    ),
    ...images.map(
      (img, i) => `<item id="img${i + 1}" href="images/${img.name}" media-type="${img.type}"/>`,
    ),
  ]
  const spine = [
    '<itemref idref="cover" linear="yes"/>',
    '<itemref idref="colophon"/>',
    '<itemref idref="nav"/>',
    ...chapters.map((_, i) => `<itemref idref="ch${i + 1}"/>`),
  ]
  return `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bookid" xml:lang="${book.lang}">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
<dc:identifier id="bookid">${esc(id)}</dc:identifier>
<dc:title>${esc(book.title)}</dc:title>
<dc:creator>${esc(book.author)}</dc:creator>
<dc:language>${book.lang}</dc:language>
${book.description ? `<dc:description>${esc(book.description)}</dc:description>` : ''}
<dc:source>${esc(book.site)}</dc:source>
<meta property="dcterms:modified">${modified}</meta>
<meta name="cover" content="cover-image"/>
</metadata>
<manifest>
${manifest.join('\n')}
</manifest>
<spine toc="ncx">
${spine.join('\n')}
</spine>
</package>
`
}

const CONTAINER_XML = `<?xml version="1.0" encoding="utf-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
<rootfiles>
<rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
</rootfiles>
</container>
`

// 리더가 저마다 글꼴·여백·밤 모드를 덮어쓰므로 색과 크기는 최소한으로만 정한다.
const STYLE_CSS = `@charset "utf-8";
body { line-height: 1.75; word-break: keep-all; overflow-wrap: break-word; }
h1, h2, h3, h4 { line-height: 1.4; page-break-after: avoid; break-after: avoid; }
h1 { font-size: 1.6em; margin: 0 0 1.4em; }
h2 { font-size: 1.3em; margin: 2em 0 0.8em; }
h3 { font-size: 1.1em; margin: 1.6em 0 0.6em; }
p { margin: 0 0 0.9em; }
a { text-decoration: none; }
code { font-family: "D2Coding", ui-monospace, Menlo, Consolas, monospace; font-size: 0.9em; }
:not(pre) > code { padding: 0 0.2em; background: rgba(127, 127, 127, 0.12); border-radius: 3px; }
.code-block { margin: 1em 0; }
pre.code {
  margin: 0; padding: 0.7em 0.8em; font-size: 0.8em; line-height: 1.5;
  white-space: pre-wrap; overflow-wrap: anywhere; word-break: normal;
  border: 1px solid rgba(127, 127, 127, 0.35); border-radius: 4px;
  color: #24292e; background-color: #f6f8fa;
}
pre.code code { font-size: 1em; padding: 0; background: none; }
.custom-block {
  margin: 1.2em 0; padding: 0.6em 0.9em;
  border: 1px solid rgba(127, 127, 127, 0.4); border-radius: 4px;
  page-break-inside: avoid; break-inside: avoid;
}
.custom-block-title { font-weight: bold; margin-bottom: 0.4em; }
.custom-block p:last-child { margin-bottom: 0; }
table { border-collapse: collapse; margin: 1em 0; font-size: 0.9em; }
th, td { border: 1px solid rgba(127, 127, 127, 0.45); padding: 0.3em 0.5em; vertical-align: top; }
th { background: rgba(127, 127, 127, 0.12); }
blockquote { margin: 1em 0; padding-left: 1em; border-left: 3px solid rgba(127, 127, 127, 0.45); }
img { max-width: 100%; height: auto; }
.fc-shots { margin: 1.5em 0 0.4em; text-align: center; page-break-inside: avoid; break-inside: avoid; }
.fc-shots p { margin: 0; text-align: center; }
.fc-caption { font-size: 0.85em; text-align: center; margin: 0.4em 0 1.5em; }
.footnotes-sep { margin-top: 2.5em; }
.footnotes { font-size: 0.85em; }
nav#toc ol { list-style: none; padding-left: 1.2em; }
nav#toc > ol { padding-left: 0; }
.colophon { margin-top: 3em; }
.colophon .book-title { margin-bottom: 0.3em; }
.colophon .book-subtitle { margin-bottom: 2em; }
.colophon .license { margin-top: 2em; font-size: 0.85em; }
`

// ── 최소 ZIP 작성기 ────────────────────────────────────────────────
// EPUB은 mimetype을 압축 없이 맨 앞에 두어야 한다. 나머지는 deflate로 줄인다.
const CRC_TABLE = Array.from({ length: 256 }, (_, n) => {
  let c = n
  for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1
  return c >>> 0
})
function crc32(buf) {
  let c = 0xffffffff
  for (const b of buf) c = CRC_TABLE[(c ^ b) & 0xff] ^ (c >>> 8)
  return (c ^ 0xffffffff) >>> 0
}

function zip(entries) {
  const locals = []
  const centrals = []
  let offset = 0
  for (const { name, data, store } of entries) {
    const nameBuf = Buffer.from(name, 'utf8')
    const raw = Buffer.isBuffer(data) ? data : Buffer.from(data, 'utf8')
    const body = store ? raw : deflateRawSync(raw, { level: 9 })
    const method = store ? 0 : 8
    const crc = crc32(raw)
    const local = Buffer.alloc(30)
    local.writeUInt32LE(0x04034b50, 0)
    local.writeUInt16LE(20, 4)
    local.writeUInt16LE(0x0800, 6) // 파일 이름이 UTF-8
    local.writeUInt16LE(method, 8)
    local.writeUInt32LE(0, 10) // 수정 시각(쓰지 않음)
    local.writeUInt32LE(crc, 14)
    local.writeUInt32LE(body.length, 18)
    local.writeUInt32LE(raw.length, 22)
    local.writeUInt16LE(nameBuf.length, 26)
    local.writeUInt16LE(0, 28)
    locals.push(local, nameBuf, body)

    const central = Buffer.alloc(46)
    central.writeUInt32LE(0x02014b50, 0)
    central.writeUInt16LE(20, 4)
    central.writeUInt16LE(20, 6)
    central.writeUInt16LE(0x0800, 8)
    central.writeUInt16LE(method, 10)
    central.writeUInt32LE(0, 12)
    central.writeUInt32LE(crc, 16)
    central.writeUInt32LE(body.length, 20)
    central.writeUInt32LE(raw.length, 24)
    central.writeUInt16LE(nameBuf.length, 28)
    central.writeUInt32LE(offset, 42)
    centrals.push(central, nameBuf)
    offset += 30 + nameBuf.length + body.length
  }
  const centralSize = centrals.reduce((n, b) => n + b.length, 0)
  const end = Buffer.alloc(22)
  end.writeUInt32LE(0x06054b50, 0)
  end.writeUInt16LE(entries.length, 8)
  end.writeUInt16LE(entries.length, 10)
  end.writeUInt32LE(centralSize, 12)
  end.writeUInt32LE(offset, 16)
  return Buffer.concat([...locals, ...centrals, end])
}

export async function exportEpub(root, book) {
  const dist = join(root, '.vitepress/dist')
  const flat = flattenChapters(book)
  const routes = flat.map((c) => c.route)
  const navTitles = new Map(book.toc.flatMap((g) => g.items.map((i) => [i.file, i.text])))
  const { port, close } = await serveDist(dist, book.base)

  const browser = await puppeteer.launch({ executablePath: findChrome(), args: ['--no-sandbox'] })
  const chapters = []
  let coverPng
  try {
    const page = await browser.newPage()
    await page.setJavaScriptEnabled(false)
    for (const c of flat) {
      await page.goto(`http://127.0.0.1:${port}${book.base}${c.route}`, {
        waitUntil: 'domcontentloaded',
        timeout: 90_000,
      })
      const out = await page.evaluate(extractChapter, { base: book.base, routes, site: book.site })
      chapters.push({ ...c, ...out, navTitle: navTitles.get(c.file) ?? out.title })
      console.log(`extracted: ${c.route}`)
    }

    // 표지: PDF 표지(A4)를 2배 해상도 PNG로 찍는다. 표지 글꼴이 base 기준 절대 경로라
    // 로컬 서버의 홈을 먼저 열어 둔다.
    await page.setJavaScriptEnabled(true)
    await page.goto(`http://127.0.0.1:${port}${book.base}`, { waitUntil: 'domcontentloaded' })
    await page.setViewport({ width: 794, height: 1123, deviceScaleFactor: 2 })
    await page.setContent(pdfCoverHtml(book), { waitUntil: 'networkidle0', timeout: 60_000 })
    await page.evaluate(() => document.fonts.ready)
    coverPng = await page.screenshot({ type: 'png', clip: { x: 0, y: 0, width: 794, height: 1123 } })
  } finally {
    await browser.close()
    close()
  }

  // 여러 장이 같은 그림을 쓸 수 있으니 이름으로 한 번만 넣는다
  const images = new Map()
  for (const c of chapters) {
    for (const img of c.images) {
      if (images.has(img.name)) continue
      const type = IMAGE_TYPES[extname(img.name).toLowerCase()]
      if (!type) throw new Error(`EPUB에 넣을 수 없는 그림 형식: ${img.path} (${c.route})`)
      images.set(img.name, { ...img, type, data: await readFile(join(dist, img.path)) })
    }
  }

  // 식별자는 사이트 주소에서 만든 고정 UUID다. 판이 바뀌어도 리더가 같은 책으로 알아본다.
  const h = createHash('sha1').update(book.site).digest('hex')
  const id = `urn:uuid:${h.slice(0, 8)}-${h.slice(8, 12)}-5${h.slice(13, 16)}-${((parseInt(h[16], 16) & 3) | 8).toString(16)}${h.slice(17, 20)}-${h.slice(20, 32)}`
  const modified = new Date().toISOString().replace(/\.\d+Z$/, 'Z')

  const entries = [
    { name: 'mimetype', data: 'application/epub+zip', store: true },
    { name: 'META-INF/container.xml', data: CONTAINER_XML },
    {
      name: 'OEBPS/content.opf',
      data: contentOpf(book, { id, modified, chapters, images: [...images.values()] }),
    },
    { name: 'OEBPS/nav.xhtml', data: navXhtml(book, chapters) },
    { name: 'OEBPS/toc.ncx', data: tocNcx(book, chapters, id) },
    { name: 'OEBPS/style.css', data: STYLE_CSS },
    { name: 'OEBPS/cover.xhtml', data: coverXhtml(book) },
    { name: 'OEBPS/colophon.xhtml', data: colophonXhtml(book, modified) },
    { name: 'OEBPS/images/cover.png', data: coverPng, store: true },
    ...chapters.map((c) => ({
      name: `OEBPS/${xhtmlName(c.route)}`,
      data: chapterXhtml(book, c.title, c.body),
    })),
    // PNG·JPEG는 이미 압축돼 있어 deflate로 줄지 않는다
    ...[...images.values()].map((img) => ({
      name: `OEBPS/images/${img.name}`,
      data: img.data,
      store: img.type !== 'image/svg+xml',
    })),
  ]

  const out = join(dist, epubFileName(book))
  const buf = zip(entries)
  await writeFile(out, buf)
  console.log(
    `EPUB 생성 완료: ${out} (${chapters.length}개 장, 그림 ${images.size}개, ${(buf.length / 1024 / 1024).toFixed(1)}MB)`,
  )
}
