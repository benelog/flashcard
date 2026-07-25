// book.config의 toc 하나에서 사이드바·PDF 차례·첫 장 경로를 모두 파생한다.
// 장을 더하거나 빼면 book.config.mjs의 toc만 고치면 된다.

const stripExt = (file) => file.replace(/\.(adoc|md)$/, '')

// PDF 차례·아웃라인용 평탄화 목록: { file, route, title, part }
// part는 그 항목 앞에 부 제목 줄을 넣을 때만 존재한다.
// 기본값은 "그룹의 첫 항목이면 그룹 이름"이고, 항목의 pdfPart로 바꿀 수 있다.
export function flattenChapters(book) {
  const out = []
  for (const group of book.toc) {
    group.items.forEach((item, i) => {
      out.push({
        file: item.file,
        route: stripExt(item.file),
        title: item.pdfTitle ?? item.text,
        part: item.pdfPart ?? (i === 0 ? group.text : undefined),
      })
    })
  }
  return out
}

export function firstRoute(book) {
  return flattenChapters(book)[0].route
}

// 장 참조 토큰({ch-파일명})용 라벨 지도: 파일 이름(확장자 없이) → "15장"·"부록".
// 라벨은 toc 항목 제목의 선두에서 읽는다. 선두가 라벨 꼴이 아닌 항목(서문·도입)은
// 지도에 오르지 않아 토큰으로 가리킬 수 없다.
export function chapterLabels(book) {
  const labels = new Map()
  for (const group of book.toc) {
    for (const item of group.items) {
      const m = item.text.match(/^(\d+장|부록)\s/)
      if (!m) continue
      const key = stripExt(item.file).replace(/^.*\//, '')
      if (labels.has(key)) {
        throw new Error(`장 파일 이름이 겹쳐 {ch-${key}} 참조가 모호하다: ${item.file}`)
      }
      labels.set(key, m[1])
    }
  }
  return labels
}

// VitePress 사이드바
export function sidebar(book) {
  return book.toc.map((group) => ({
    text: group.text,
    items: group.items.map((item) => ({ text: item.text, link: '/' + stripExt(item.file) })),
  }))
}
