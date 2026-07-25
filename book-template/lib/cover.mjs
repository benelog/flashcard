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

// 계층 판 윗면에 그려 넣는 그림. cover.diagram[].motif로 고른다(없으면 빈 면).
// screen: 간소화한 앱 화면(상단 바·카드·버튼), code: if/for 코드 줄, table: DB 테이블 격자.
function motifHtml(motif) {
  if (motif === 'screen')
    return (
      '<span class="m-screen"><span class="m-topbar"></span>' +
      '<span class="m-card"><span class="m-word"></span><span class="m-ans"></span></span>' +
      '<span class="m-btn"></span></span>'
    )
  if (motif === 'code')
    return (
      '<span class="m-code">' +
      '<span class="m-cl"><em class="m-kw">for</em><span class="m-b m-b1"></span></span>' +
      '<span class="m-cl m-ind"><span class="m-b m-b2"></span></span>' +
      '<span class="m-cl m-ind"><em class="m-kw">if</em><span class="m-b m-b3"></span></span>' +
      '<span class="m-cl m-ind2"><span class="m-b m-b4"></span></span>' +
      '</span>'
    )
  if (motif === 'table')
    return (
      '<span class="m-table">' +
      '<span class="m-th"></span>'.repeat(4) +
      '<span class="m-td"></span>'.repeat(12) +
      '</span>'
    )
  return ''
}

// 홈(index.md) — 책 표지 랜딩 페이지
export function homeCoverMarkdown(book) {
  const c = book.cover
  const start = book.base + firstRoute(book)
  const diagram = c.diagram?.length
    ? `\n    <div class="fc-book-diagram" aria-hidden="true">\n      ${c.diagram
        .map(
          (l) =>
            `<div class="fc-layer"><span class="fc-plane"><i class="fc-side fc-side-l"></i><i class="fc-side fc-side-r"></i><i class="fc-top">${motifHtml(l.motif)}</i></span><span class="fc-layer-label"><span class="fc-layer-name">${l.name}</span><span class="fc-layer-tech">${l.tech}</span></span></div>`,
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
            `<div class="layer"><span class="plane"><i class="side side-l"></i><i class="side side-r"></i><i class="top">${motifHtml(l.motif)}</i></span><span class="label"><span class="name">${l.name}</span><span class="tech">${l.tech}</span></span></div>`,
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
      background: #ffffff; color: #171717; font-family: 'Noto Sans KR', sans-serif;
      display: flex; flex-direction: column;
    }
    .spine { position: absolute; inset: 0 auto 0 0; width: 4mm; background: #2563eb; }
    h1 { font-size: 38pt; line-height: 1.24; font-weight: 700; margin: 3mm 0 0; word-break: keep-all; }
    h1 strong { display: inline-block; padding: 0 3mm 1.5mm; background: #f59e0b; color: #171717; font-size: 43pt; line-height: 1; font-weight: 700; }
    .subtitle { margin: 7mm 0 0; font-family: 'Noto Serif KR', serif; font-size: 14pt; font-weight: 600; color: #525252; line-height: 1.75; word-break: keep-all; }
    /* 세 층을 두께가 있는 판(아이소메트릭 슬래브)으로 쌓는다. 판마다 세 면을 그린다.
       윗면은 가로로 긴 직사각형을 rotate(45deg)로 세워 scaleY(0.45)로 눕힌 평행사변형,
       좌우 옆면은 그 아래 두 변에 맞춰 skewY(±24.228deg)로 기울인 직사각형이다
       (눕힌 뒤 변의 기울기가 0.45 = tan 24.228°).
       윗면은 실제 요소(.top)라 안에 각 층의 그림(motif)을 담고, 그림도 함께 눕는다. */
    .diagram { flex: none; margin: 8mm 0 0; }
    .layer { position: relative; display: flex; align-items: center; gap: 8mm; height: 44mm; }
    .layer + .layer { margin-top: -6mm; }
    .layer:nth-child(1) { z-index: 3; }
    .layer:nth-child(2) { z-index: 2; }
    .layer:nth-child(3) { z-index: 1; }
    .plane { position: relative; flex: none; width: 90mm; height: 100%;
      filter: drop-shadow(0 3mm 4mm rgba(30, 64, 175, 0.16));
      --sw: 75mm;                                          /* 윗면 직사각형 가로 */
      --sh: 47mm;                                          /* 세로: 가로와 16:10(모니터 비율) */
      --t: 5mm;                                            /* 판 두께 */
      --dw: calc((var(--sw) + var(--sh)) * 0.3536);        /* 윗면 반너비 */
      --dh: calc((var(--sw) + var(--sh)) * 0.1591);        /* 윗면 반높이 */
      --ldrop: calc((var(--sh) - var(--sw)) * 0.1591);     /* 왼쪽 꼭짓점의 중심 대비 높이 */
    }
    .top {
      position: absolute; z-index: 1; left: 50%; top: calc(50% - var(--t) / 2);
      width: var(--sw); height: var(--sh); box-sizing: border-box; padding: 4.5mm 5mm;
      background: #fff; border: 0.5mm solid #d9e5f5;
      display: flex; flex-direction: column; justify-content: center;
      transform: translate(-50%, -50%) scaleY(0.45) rotate(45deg);
    }
    .side { position: absolute; height: var(--t); }
    .side-l { width: calc(var(--sw) * 0.7071); left: calc(50% - var(--dw)); top: calc(50% - var(--t) / 2 + var(--ldrop)); transform-origin: 0 0; transform: skewY(24.228deg); }
    .side-r { width: calc(var(--sh) * 0.7071); left: calc(50% + (var(--sw) - var(--sh)) * 0.3536); top: calc(50% - var(--t) / 2 + var(--dh)); transform-origin: 0 0; transform: skewY(-24.228deg); }
    /* 층이 내려갈수록 옆면 파랑이 한 단씩 짙어진다 */
    .layer:nth-child(1) .side-l { background: #bfdbfe; }
    .layer:nth-child(1) .side-r { background: #93c5fd; }
    .layer:nth-child(2) .side-l { background: #3b82f6; }
    .layer:nth-child(2) .side-r { background: #2563eb; }
    .layer:nth-child(3) .side-l { background: #1d4ed8; }
    .layer:nth-child(3) .side-r { background: #1e40af; }
    /* 윗면 그림: 간소화한 앱 화면 */
    .m-screen { display: flex; flex-direction: column; justify-content: center; gap: 2.2mm; width: 100%; }
    .m-topbar { height: 2.6mm; width: 45%; border-radius: 1mm; background: #e2e8f0; }
    .m-card { display: flex; flex-direction: column; gap: 2mm; padding: 3mm 3.5mm; border: 0.8mm solid #bfdbfe; border-radius: 2.5mm; background: #eff6ff; }
    .m-word { height: 3mm; width: 45%; border-radius: 1mm; background: #171717; }
    .m-ans { height: 2.2mm; width: 70%; border-radius: 1mm; background: #93c5fd; }
    .m-btn { height: 4mm; width: 100%; border-radius: 2mm; background: #2563eb; }
    /* 윗면 그림: if/for 코드 줄 */
    .m-code { display: flex; flex-direction: column; gap: 2.2mm; width: 100%; }
    .m-cl { display: flex; align-items: center; gap: 2mm; }
    .m-ind { padding-left: 6mm; }
    .m-ind2 { padding-left: 12mm; }
    .m-kw { font-family: ui-monospace, 'Cascadia Code', 'Source Code Pro', Menlo, Consolas, monospace; font-style: normal; font-weight: 700; font-size: 8.5pt; line-height: 1; color: #2563eb; }
    .m-b { height: 2.5mm; border-radius: 1mm; background: #cbd5e1; }
    .m-b1 { width: 48%; }
    .m-b2 { width: 62%; }
    .m-b3 { width: 36%; }
    .m-b4 { width: 52%; background: #93c5fd; }
    /* 윗면 그림: DB 테이블 격자 */
    .m-table { display: grid; grid-template-columns: repeat(4, 1fr); gap: 0.8mm; width: 100%; border: 0.8mm solid #bfdbfe; border-radius: 2.5mm; overflow: hidden; background: #bfdbfe; }
    .m-th { height: 4.8mm; background: #2563eb; }
    .m-td { height: 4.8mm; background: #fff; }
    .label { display: flex; flex-direction: column; gap: 1mm; }
    .layer .name { font-size: 13pt; font-weight: 700; color: #171717; }
    .layer .tech { font-family: ui-monospace, 'Cascadia Code', 'Source Code Pro', Menlo, Consolas, monospace; font-size: 9.5pt; font-weight: 600; letter-spacing: 0.2mm; color: #2563eb; }
    .pitch { margin: 9mm 0 0; padding: 0; list-style: none; font-size: 10.5pt; line-height: 1.9; font-weight: 600; color: #525252; word-break: keep-all; }
    .pitch li::before { content: ''; display: inline-block; width: 2mm; height: 2mm; margin: 0 3mm 0.4mm 0; background: #2563eb; }
    .bottom { display: flex; align-items: flex-end; justify-content: flex-end; margin-top: auto; padding-top: 5mm; border-top: 0.3mm solid #e5e5e5; }
    .author { font-size: 14pt; font-weight: 600; color: #171717; margin: 0 0 2mm; text-align: right; }
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
