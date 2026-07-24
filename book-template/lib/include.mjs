// 원고가 저장소의 실제 코드를 복사하지 않고 인용하게 하는 include:: 지시자.
//
//   [source,go]
//   ----
//   include::../../internal/srs/srs.go[tag=grade]
//   ----
//
// downdoc은 include를 그냥 버리므로(README의 "include directives are dropped"),
// 변환 파이프라인 앞에서 이 모듈이 직접 펼친다. 인용한 파일이나 태그가 사라지면
// 빌드가 파일:줄과 함께 실패하므로, 코드를 고친 뒤 원고만 옛 상태로 남는 일이
// 없다.
//
// 인용 대상 코드에는 마커 주석을 단다. 주석 기호는 그 언어의 것을 쓰면 된다.
//
//   // tag::grade[]
//   func Grade(...) { ... }
//   // end::grade[]
//
// 펼친 결과에서 마커 줄 자체는 빠지고, 발췌 앞뒤의 빈 줄도 걷어 낸다.
import { existsSync, readFileSync } from 'node:fs'
import { dirname, relative, resolve } from 'node:path'

const INCLUDE_RE = /^include::([^[\]]+)\[([^\]]*)\]$/
// 코드 블록 구분자. include는 이 안에서만 쓴다.
const VERBATIM_DELIM = /^(----|\.\.\.\.|\+\+\+\+)$/
const MARKER_RE = /(?:tag|end)::[\w.-]+\[\]/
const marker = (kind, tag) => new RegExp(`\\b${kind}::${tag.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\[\\]`)

// parseOptions reads the bracket attributes: tag=이름, indent=0.
function parseOptions(raw, report) {
  const options = {}
  for (const part of raw.split(',')) {
    const item = part.trim()
    if (!item) continue
    const [key, value] = item.split('=').map((s) => s.trim())
    if (key === 'tag' && value) options.tag = value
    else if (key === 'indent' && value === '0') options.dedent = true
    else if (key === 'lines') report(`lines=는 쓰지 않는다 (줄 번호는 코드가 바뀌면 어긋난다). tag=를 쓴다`)
    else report(`알 수 없는 include 옵션이다: ${item}`)
  }
  return options
}

// takeTag returns the lines between the tag's markers, or an error string.
function takeTag(lines, tag) {
  const open = lines.findIndex((l) => marker('tag', tag).test(l))
  if (open < 0) return { error: `태그 ${tag}를 찾지 못했다` }
  const rest = lines.slice(open + 1)
  const close = rest.findIndex((l) => marker('end', tag).test(l))
  if (close < 0) return { error: `태그 ${tag}를 닫는 end::${tag}[]가 없다` }
  return { lines: rest.slice(0, close) }
}

// dedent drops the deepest indentation every non-blank line shares.
function dedent(lines) {
  const widths = lines.filter((l) => l.trim()).map((l) => l.match(/^[ \t]*/)[0].length)
  const common = widths.length ? Math.min(...widths) : 0
  return common ? lines.map((l) => l.slice(common)) : lines
}

function trimBlankEdges(lines) {
  let start = 0
  let end = lines.length
  while (start < end && !lines[start].trim()) start++
  while (end > start && !lines[end - 1].trim()) end--
  return lines.slice(start, end)
}

// resolveIncludes expands every include:: line in one manuscript.
//
// Returns the expanded text, the source line number each output line came from
// (so later stages can still report 파일:줄 of the manuscript), and the set of
// included files (book dev watches them).
export function resolveIncludes(source, srcPath, root = dirname(srcPath)) {
  const shown = relative(root, srcPath) || srcPath
  const errors = []
  const deps = new Set()
  const out = []
  const lineNumbers = []
  let verbatim = false

  source.split('\n').forEach((line, i) => {
    const at = `${shown}:${i + 1}`
    const keep = () => {
      out.push(line)
      lineNumbers.push(i + 1)
    }
    if (VERBATIM_DELIM.test(line.trimEnd())) {
      verbatim = !verbatim
      keep()
      return
    }
    const directive = line.trim().match(INCLUDE_RE)
    if (!directive) {
      if (!verbatim && line.trimStart().startsWith('include::')) {
        errors.push(`${at} include:: 문법이 잘못됐다: ${line.trim()}`)
      }
      keep()
      return
    }
    if (!verbatim) {
      errors.push(`${at} include::는 코드 블록(----) 안에서만 쓴다`)
      return
    }

    const report = (message) => errors.push(`${at} ${message}`)
    const options = parseOptions(directive[2], report)
    const target = resolve(dirname(srcPath), directive[1].trim())
    if (!existsSync(target)) {
      errors.push(`${at} 인용할 파일이 없다: ${relative(root, target)}`)
      return
    }
    deps.add(target)

    let body = readFileSync(target, 'utf8').split('\n')
    if (options.tag) {
      const taken = takeTag(body, options.tag)
      if (taken.error) {
        errors.push(`${at} ${taken.error}: ${relative(root, target)}`)
        return
      }
      body = taken.lines
    }
    body = trimBlankEdges(body.filter((l) => !MARKER_RE.test(l)))
    if (options.dedent) body = dedent(body)
    if (!body.length) {
      errors.push(`${at} 인용한 범위가 비어 있다: ${relative(root, target)}`)
      return
    }
    for (const included of body) {
      out.push(included)
      lineNumbers.push(i + 1) // 인용문의 오류는 include 줄에서 보고한다
    }
  })

  if (errors.length) {
    throw new Error(`코드 인용(include) 실패:\n${errors.join('\n')}`)
  }
  return { text: out.join('\n'), lineNumbers, deps: [...deps] }
}
