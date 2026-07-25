// 장 참조 토큰({ch-…})을 book.config의 toc에서 파생한 장 번호로 치환한다.
//
// 본문에 장 번호를 직접 쓰면 장을 재배열할 때마다 원고 전체를 훑어야 한다.
// 대신 {ch-파일명} 토큰을 쓰면 빌드가 toc 순서에서 번호를 계산해 치환하므로,
// 재배열 때 고칠 곳이 toc 하나로 준다. 번호가 toc와 어긋날 일도 없다.
//
//   {ch-gin}                  → "15장"     (toc 항목 제목의 선두 라벨)
//   {ch-go-hello·go-types}    → "8·9장"    (장 묶음)
//   {ch-go-basics~go-testing} → "12~14장"  (장 범위)
//
// 코드 블록(----, ...., ++++) 안은 치환하지 않는다. 코드 안 생략 주석의
// 장 번호는 여전히 손으로 맞춰야 한다.

const VERBATIM_DELIM = /^(----|\.\.\.\.|\+\+\+\+)$/
const TOKEN = /\{ch-([a-z0-9-]+(?:[·~][a-z0-9-]+)*)\}/g

// 라벨("15장", "부록")에서 번호를 꺼낸다. 번호가 아니면 null.
const numberOf = (label) => {
  const m = label.match(/^(\d+)장$/)
  return m ? m[1] : null
}

function resolveToken(expr, labels) {
  const joiner = expr.includes('~') ? '~' : '·'
  const keys = expr.split(/[·~]/)
  const missing = keys.filter((k) => !labels.has(k))
  if (missing.length) {
    throw new Error(`toc에 없는 장 참조다: ${missing.join(', ')}`)
  }
  if (keys.length === 1) return labels.get(keys[0])
  if (joiner === '~' && keys.length !== 2) {
    throw new Error(`범위 참조(~)는 시작과 끝 두 장만 쓴다: {ch-${expr}}`)
  }
  const nums = keys.map((k) => {
    const n = numberOf(labels.get(k))
    if (n === null) {
      throw new Error(`번호가 없는 장(${labels.get(k)})은 묶음·범위 참조에 쓸 수 없다: {ch-${expr}}`)
    }
    return n
  })
  return nums.join(joiner) + '장'
}

// 원고 본문의 {ch-…} 토큰을 치환한다. lineNumbers는 include 해석으로 늘어난
// 각 줄이 원고 몇 번째 줄에서 왔는지다(오류를 원고 줄 번호로 짚기 위함).
export function applyChapterRefs(source, labels, file, lineNumbers) {
  const errors = []
  let verbatim = false
  const out = source.split('\n').map((line, i) => {
    if (VERBATIM_DELIM.test(line.trimEnd())) {
      verbatim = !verbatim
      return line
    }
    if (verbatim) return line
    return line.replace(TOKEN, (whole, expr) => {
      try {
        return resolveToken(expr, labels)
      } catch (e) {
        errors.push(`${file}:${lineNumbers?.[i] ?? i + 1} ${e.message}`)
        return whole
      }
    })
  })
  if (errors.length) {
    throw new Error(`장 참조 토큰 오류:\n${errors.join('\n')}`)
  }
  return out.join('\n')
}

// 제목 줄에 toc의 장 라벨을 붙인다: "= Gin으로 …" → "= 15장 Gin으로 …".
// 원고 제목에는 번호를 쓰지 않고, 번호는 toc에서만 나온다.
// 이미 라벨이 붙어 있으면 그대로 둔다(이행기 원고 허용).
export function numberTitle(source, label) {
  if (!label) return source
  const nl = source.indexOf('\n')
  const first = nl === -1 ? source : source.slice(0, nl)
  if (!first.startsWith('= ') || first.startsWith(`= ${label} `)) return source
  const titled = `= ${label} ${first.slice(2)}`
  return nl === -1 ? titled : titled + source.slice(nl)
}
