// 원고 본문의 치환 토큰을 훑는 공통 골격.
//
// 장 참조({ch-…})와 수치({n-…})는 규칙이 다르지만 훑는 방식은 같다.
// 코드 블록(----, ...., ++++) 안은 건드리지 않고, 오류는 원고 줄 번호로 짚는다.
// lineNumbers는 include 해석으로 늘어난 각 줄이 원고 몇 번째 줄에서 왔는지다.

const VERBATIM_DELIM = /^(----|\.\.\.\.|\+\+\+\+)$/

// resolve가 던지는 오류를 모아 한꺼번에 보고한다. 한 번 돌 때 원고의 문제를
// 다 보여 주는 편이, 고칠 때마다 빌드를 다시 돌리는 것보다 낫다.
export function replaceTokens(source, pattern, resolve, { file, lineNumbers, label }) {
  const errors = []
  let verbatim = false
  const out = source.split('\n').map((line, i) => {
    if (VERBATIM_DELIM.test(line.trimEnd())) {
      verbatim = !verbatim
      return line
    }
    if (verbatim) return line
    return line.replace(pattern, (whole, expr) => {
      try {
        return resolve(expr)
      } catch (e) {
        errors.push(`${file}:${lineNumbers?.[i] ?? i + 1} ${e.message}`)
        return whole
      }
    })
  })
  if (errors.length) {
    throw new Error(`${label}:\n${errors.join('\n')}`)
  }
  return out.join('\n')
}
