// 표지의 단일 소스. book.config의 cover 데이터에서
// 홈(랜딩) 페이지 마크다운과 PDF 표지·차례 HTML을 만든다.
// 스타일은 theme/custom.css(홈)와 이 파일 안의 인쇄 CSS(PDF)가 담당한다.
import { FONT_LINKS } from './fonts.mjs'
import { firstRoute } from './toc.mjs'

// cover.actions의 link 약어를 실제 주소로 푼다.
function actionHref(book, link) {
  if (link === 'start') return book.base + firstRoute(book)
  if (link === 'pdf') return `${book.site}${book.pdf.fileName}`
  if (link === 'repo') return book.repo
  return link
}

// 홈(index.md) — 책 표지 랜딩 페이지
export function homeCoverMarkdown(book) {
  const c = book.cover
  const start = book.base + firstRoute(book)
  const diagram = c.diagram?.length
    ? `\n    <div class="fc-book-diagram" aria-hidden="true">\n      ${c.diagram
        .map(
          (l) =>
            `<div class="fc-layer"><span class="fc-plane"><i class="fc-side fc-side-l"></i><i class="fc-side fc-side-r"></i></span><span class="fc-layer-label"><span class="fc-layer-name">${l.name}</span><span class="fc-layer-tech">${l.tech}</span></span></div>`,
        )
        .join('\n      ')}\n    </div>`
    : ''
  const pitch = c.pitch?.length
    ? `\n    <ul class="fc-book-pitch">\n${c.pitch.map((p) => `      <li>${p}</li>`).join('\n')}\n    </ul>`
    : ''
  const actions = (c.actions ?? [])
    .map(
      (a) =>
        `    <a class="fc-btn${a.brand ? ' brand' : ''}" href="${actionHref(book, a.link)}">${a.text}</a>`,
    )
    .join('\n')
  return `---
layout: page
sidebar: false
---

<div class="fc-home">
  <a class="fc-book" href="${start}" aria-label="읽기 시작">
    <h1>${c.titleHtml}</h1>
    <p class="fc-book-subtitle">${c.subtitleHtml}</p>${diagram}${pitch}
    <div class="fc-book-footer"><p class="fc-book-author">${book.author} 지음</p></div>
  </a>
  <p class="fc-home-desc">${c.homeDesc}</p>
  <div class="fc-home-actions">
${actions}
  </div>
</div>
`
}

// PDF 표지 (A4 한 장, 인쇄용 CSS 내장)
export function pdfCoverHtml(book) {
  const c = book.cover
  const diagram = c.diagram?.length
    ? `\n    <div class="diagram">\n      ${c.diagram
        .map(
          (l) =>
            `<div class="layer"><span class="plane"><i class="side side-l"></i><i class="side side-r"></i></span><span class="label"><span class="name">${l.name}</span><span class="tech">${l.tech}</span></span></div>`,
        )
        .join('\n      ')}\n    </div>`
    : ''
  const pitch = c.pitch?.length
    ? `\n    <ul class="pitch">\n${c.pitch.map((p) => `      <li>${p}</li>`).join('\n')}\n    </ul>`
    : ''
  return `<!doctype html><html lang="ko"><head><meta charset="utf-8">${FONT_LINKS}
  <style>
    @page { size: A4; margin: 0; }
    html, body { margin: 0; padding: 0; }
    .cover {
      position: relative; box-sizing: border-box; width: 210mm; height: 296mm;
      padding: 20mm 19mm 17mm 24mm; overflow: hidden;
      background: #0d1117; color: #f5f5f5; font-family: 'Noto Sans KR', sans-serif;
      display: flex; flex-direction: column;
    }
    .spine { position: absolute; inset: 0 auto 0 0; width: 4mm; background: #2563eb; }
    h1 { font-size: 38pt; line-height: 1.24; font-weight: 700; margin: 3mm 0 0; word-break: keep-all; }
    h1 strong { display: inline-block; padding: 0 3mm 1.5mm; background: #f59e0b; color: #0d1117; font-size: 43pt; line-height: 1; font-weight: 700; }
    .subtitle { margin: 8mm 0 0; font-family: 'Noto Serif KR', serif; font-size: 14pt; font-weight: 600; color: #a3a3a3; line-height: 1.75; word-break: keep-all; }
    /* 세 층을 두께가 있는 판(아이소메트릭 슬래브)으로 쌓는다. 판마다 세 면을 그린다.
       윗면은 사각형을 rotate(45deg)로 세워 scaleY(0.5)로 눕힌 마름모,
       좌우 옆면은 마름모 아래 변에 맞춰 skewY(±26.565deg)로 기울인 직사각형이다
       (마름모 변의 기울기가 정확히 0.5 = tan 26.565°). */
    .diagram { flex: none; margin: 15mm 0 0 2mm; }
    .layer { position: relative; display: flex; align-items: center; gap: 9mm; height: 31mm; }
    .layer + .layer { margin-top: -12mm; }
    .layer:nth-child(1) { z-index: 3; }
    .layer:nth-child(2) { z-index: 2; }
    .layer:nth-child(3) { z-index: 1; }
    .plane { position: relative; flex: none; width: 53mm; height: 100%;
      --s: 36mm;                       /* 윗면 정사각형 한 변 */
      --d2: calc(var(--s) * 0.7071);   /* 마름모 반너비 */
      --q: calc(var(--s) * 0.35355);   /* 마름모 반높이 */
      --t: 4.5mm;                      /* 판 두께 */
    }
    .plane::before {
      content: ''; position: absolute; z-index: 1; left: 50%; top: calc(50% - var(--t) / 2);
      width: var(--s); height: var(--s);
      transform: translate(-50%, -50%) scaleY(0.5) rotate(45deg);
    }
    .side { position: absolute; width: var(--d2); height: var(--t); }
    .side-l { left: calc(50% - var(--d2)); top: calc(50% - var(--t) / 2); transform-origin: 0 0; transform: skewY(26.565deg); }
    .side-r { left: 50%; top: calc(50% - var(--t) / 2 + var(--q)); transform-origin: 0 0; transform: skewY(-26.565deg); }
    .layer:nth-child(1) .plane::before { background: #eff6ff; }
    .layer:nth-child(1) .side-l { background: #bfdbfe; }
    .layer:nth-child(1) .side-r { background: #93c5fd; }
    .layer:nth-child(2) .plane::before { background: #60a5fa; }
    .layer:nth-child(2) .side-l { background: #3b82f6; }
    .layer:nth-child(2) .side-r { background: #2563eb; }
    .layer:nth-child(3) .plane::before { background: #2563eb; }
    .layer:nth-child(3) .side-l { background: #1d4ed8; }
    .layer:nth-child(3) .side-r { background: #1e40af; }
    .label { display: flex; flex-direction: column; gap: 1mm; }
    .layer .name { font-size: 13pt; font-weight: 700; color: #f5f5f5; }
    .layer .tech { font-family: ui-monospace, 'Cascadia Code', 'Source Code Pro', Menlo, Consolas, monospace; font-size: 9.5pt; font-weight: 600; letter-spacing: 0.2mm; color: #93c5fd; }
    .pitch { margin: 10mm 0 0; padding: 0; list-style: none; font-size: 10.5pt; line-height: 1.9; font-weight: 600; color: #a3a3a3; word-break: keep-all; }
    .pitch li::before { content: ''; display: inline-block; width: 2mm; height: 2mm; margin: 0 3mm 0.4mm 0; background: #2563eb; }
    .bottom { display: flex; align-items: flex-end; justify-content: flex-end; margin-top: auto; padding-top: 5mm; border-top: 0.3mm solid #30363d; }
    .author { font-size: 14pt; font-weight: 600; color: #f5f5f5; margin: 0 0 2mm; text-align: right; }
    .site { font-family: ui-monospace, 'Cascadia Code', 'Source Code Pro', Menlo, Consolas, monospace; font-size: 8.5pt; color: #737373; margin: 0; text-align: right; }
  </style></head><body>
  <div class="cover">
    <div class="spine"></div>
    <h1>${c.titleHtml}</h1>
    <p class="subtitle">${book.subtitle}</p>${diagram}${pitch}
    <div class="bottom">
      <div><p class="author">${book.author} 지음</p><p class="site">${book.siteLabel}</p></div>
    </div>
  </div></body></html>`
}

// PDF 차례 (장별 시작 쪽 번호 포함)
export function pdfTocHtml(chapters, startPages) {
  const rows = chapters
    .map((c, i) => {
      const part = c.part ? `<div class="part">${c.part}</div>` : ''
      return `${part}<div class="row"><span class="t">${c.title}</span><span class="dots"></span><span class="p">${startPages[i]}</span></div>`
    })
    .join('\n')
  return `<!doctype html><html lang="ko"><head><meta charset="utf-8">${FONT_LINKS}
  <style>
    html, body { margin: 0; padding: 0; }
    body { font-family: 'Noto Serif KR', serif; color: #1c1c1e; }
    h1 { font-family: 'Noto Sans KR', sans-serif; font-size: 21pt; font-weight: 700; margin: 6mm 0 12mm; }
    .part { font-family: 'Noto Sans KR', sans-serif; font-size: 11.5pt; font-weight: 700; color: #33436e; margin: 9mm 0 2.5mm; }
    .row { display: flex; align-items: baseline; font-size: 11pt; line-height: 2.2; word-break: keep-all; }
    .row .t { padding-right: 3mm; }
    .row .dots { flex: 1; border-bottom: 1px dotted #b3b3b8; transform: translateY(-1.5mm); }
    .row .p { padding-left: 3mm; font-variant-numeric: tabular-nums; color: #444; }
  </style></head><body>
  <h1>차례</h1>
  ${rows}
  </body></html>`
}
