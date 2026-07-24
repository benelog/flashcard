// AsciiDoc 원고 한 편을 VitePress용 마크다운으로 변환한다.
//
//   원문 → [전처리: 규약 검증 + 보호] → downdoc → [후처리: 복원] → sanity check
//
// downdoc의 인라인 치환은 두 가지 함정이 있어 전처리에서 피해 간다.
// 첫째, 강조(*굵게*)가 한글 조사와 붙으면(예: *상자*로) 변환되지 않으므로
// 강조를 제어 문자 센티널로 바꿔 downdoc을 통과시킨 뒤 **굵게**로 복원한다.
// 둘째, 코드 스팬 안의 별표(`count(*)`)가 같은 줄의 다른 별표와 강조 쌍으로
// 묶이므로 \*로 이스케이프했다가 복원한다.
// admonition 블록은 downdoc이 <dl> HTML로 내보내므로 VitePress 컨테이너로 되돌린다.
import downdoc from 'downdoc'

// AsciiDoc admonition → VitePress 컨테이너 종류
const ADMONITION_MAP = {
  NOTE: 'info',
  TIP: 'tip',
  IMPORTANT: 'warning',
  WARNING: 'warning',
  CAUTION: 'danger',
}
const ADMONITION_LABELS = Object.keys(ADMONITION_MAP).join('|')
// downdoc 출력: <dl><dt><strong>{이모지} NOTE: 제목</strong></dt><dd>
const DT_RE = new RegExp(`^<dl><dt><strong>\\S+ (${ADMONITION_LABELS})(?:: (.+))?</strong></dt><dd>$`)

// 코드 스팬: 이중 백틱(안에 백틱 허용)을 단일 백틱보다 먼저 잡는다
const CODE_SPAN = /(``[^`]+(?:`[^`]+)*``|`[^`]*`)/

// 센티널(강조 여닫이·따옴표·앰퍼샌드)과 자리표시(코드 스팬 마스킹)용 제어 문자.
// downdoc은 따옴표가 백틱과 붙으면 굽은 따옴표(<q>, ')로 바꾸고, 같은 줄에
// 코드 스팬이 여럿이면 &lt; 같은 엔티티를 디코드하므로 세 문자를 아예 숨긴다.
const B_OPEN = '\x01'
const B_CLOSE = '\x02'
const MASK_OPEN = '\x03'
const MASK_CLOSE = '\x04'
const Q_DOUBLE = '\x05'
const Q_SINGLE = '\x06'
const AMP = '\x07'

// downdoc이 monospace 안에서도 치환하는 내장 속성 이름들
const BUILTIN_ATTRS = [
  'empty',
  'idprefix',
  'idseparator',
  'markdown-line-break',
  'markdown-strikethrough',
  'nbsp',
  'quotes',
  'sp',
  'vbar',
  'zwsp',
]

const VERBATIM_DELIM = /^(----|\.\.\.\.|\+\+\+\+)$/

// 한 줄(비-verbatim)의 규약 위반을 모으고, 보호를 적용한 줄을 돌려준다
function protectLine(line, at, errors) {
  if (/[\x00-\x07]/.test(line)) {
    errors.push(`${at} 제어 문자는 쓸 수 없다`)
    return line
  }
  // 1) 코드 스팬을 자리표시로 가린다
  const spans = []
  const masked = line.replace(new RegExp(CODE_SPAN.source, 'g'), (m) => {
    spans.push(m)
    return `${MASK_OPEN}${spans.length - 1}${MASK_CLOSE}`
  })
  for (const s of spans) {
    if (s.includes('\\*')) {
      errors.push(`${at} 코드 스팬 안의 \\*는 지원하지 않는다 (별표는 그대로 쓴다): ${s}`)
    }
  }
  if (/`\+[^`]*\+`/.test(line)) {
    errors.push(`${at} \`+…+\` 패스스루 대신 일반 백틱을 쓴다`)
  }
  for (const name of BUILTIN_ATTRS) {
    if (line.includes(`{${name}}`)) {
      errors.push(`${at} downdoc 내장 속성 {${name}}은 본문에 쓸 수 없다`)
    }
  }
  // 2) 강조(*…*)를 센티널로 바꾼다 (코드 스팬이 가려진 상태라 안의 별표와 안 얽힌다).
  //    마크다운처럼 여는 별표 뒤와 닫는 별표 앞은 공백이 아니어야 한다
  //    (data-* 같은 프로즈의 홑 별표는 강조가 아니다).
  const bolded = masked.replace(/\*(?!\s)([^*\n]+?)(?<!\s)\*/g, `${B_OPEN}$1${B_CLOSE}`)
  // 남은 별표 중 강조 쌍처럼 보이는 모양은 downdoc이 임의로 묶을 수 있으므로 거절한다
  if (/\*(?=\S)[^*\n]*?(?<=\S)\*/.test(bolded)) {
    errors.push(`${at} 짝이 모호한 별표가 있다 (강조는 *굵게* 한 겹, 코드 속 별표는 백틱 안에)`)
  }
  // 3) 코드 스팬을 되살리며 안의 별표를 \*로 이스케이프하고,
  //    따옴표와 앰퍼샌드는 줄 전체(프로즈·코드 스팬 모두)에서 숨긴다
  return bolded
    .replace(new RegExp(`${MASK_OPEN}(\\d+)${MASK_CLOSE}`, 'g'), (_, i) =>
      spans[Number(i)].replaceAll('*', '\\*'),
    )
    .replaceAll('"', Q_DOUBLE)
    .replaceAll("'", Q_SINGLE)
    .replaceAll('&', AMP)
}

// 전처리: 규약 검증 + 강조·코드 스팬 보호
//
// lineNumbers는 각 줄이 원고 몇 번째 줄에서 왔는지다. include::로 펼친 원고는
// 줄 수가 늘어나므로, 이것이 있어야 오류를 원고의 줄 번호로 짚어 줄 수 있다.
function protect(source, file, lineNumbers) {
  const errors = []
  const out = []
  let verbatim = false
  source.split('\n').forEach((line, i) => {
    const at = `${file}:${lineNumbers?.[i] ?? i + 1}`
    if (VERBATIM_DELIM.test(line.trimEnd())) {
      verbatim = !verbatim
      out.push(line)
      return
    }
    if (verbatim) {
      out.push(line)
      return
    }
    if (/^:[a-zA-Z][a-zA-Z0-9_-]*(!)?:( |$)/.test(line)) {
      errors.push(`${at} 문서 속성 정의는 금지다: ${line.trim()}`)
    }
    // 목록 마커(* , . )는 강조·별표 처리에서 떼어 놓는다
    const marker = line.match(/^(\*+ |\.+ )/)?.[1] ?? ''
    out.push(marker + protectLine(line.slice(marker.length), at, errors))
  })
  if (errors.length) {
    throw new Error(`AsciiDoc 집필 규약 위반:\n${errors.join('\n')}`)
  }
  return out.join('\n')
}

// 후처리: admonition의 dl 래퍼를 컨테이너로, 센티널·이스케이프를 원래 표기로
function restore(md) {
  const out = []
  let fence = false
  for (let line of md.split('\n')) {
    if (/^```/.test(line)) {
      fence = !fence
      out.push(line)
      continue
    }
    if (fence) {
      out.push(line)
      continue
    }
    const m = line.match(DT_RE)
    if (m) {
      const kind = ADMONITION_MAP[m[1]]
      line = m[2] ? `::: ${kind} ${m[2]}` : `::: ${kind}`
    } else if (line === '</dd></dl>') {
      line = ':::'
    }
    out.push(
      line
        .replaceAll(B_OPEN, '**')
        .replaceAll(B_CLOSE, '**')
        .replaceAll('\\*', '*')
        .replaceAll(Q_DOUBLE, '"')
        .replaceAll(Q_SINGLE, "'")
        .replaceAll(AMP, '&'),
    )
  }
  return out.join('\n')
}

function sanityCheck(source, md, file) {
  const problems = []
  if (/<dl><dt>|<\/dd><\/dl>/.test(md)) {
    problems.push('admonition 래퍼(<dl>)가 후처리에서 변환되지 않고 남았다')
  }
  if (/[\x01-\x07]/.test(md)) {
    problems.push('보호용 제어 문자가 복원되지 않고 남았다')
  }
  // 의도치 않은 속성 치환 징후: 원문보다 여는 중괄호가 줄었다 (주석 줄 제외)
  const count = (s) => (s.match(/\{/g) ?? []).length
  const commentBraces = count(source.match(/^\/\/.*$/gm)?.join('\n') ?? '')
  if (count(md) < count(source) - commentBraces) {
    problems.push(`여는 중괄호가 원문보다 줄었다 (속성 참조로 치환된 듯하다)`)
  }
  if (problems.length) {
    throw new Error(`${file} 변환 결과 검증 실패:\n${problems.map((p) => `- ${p}`).join('\n')}`)
  }
}

export function convertAdoc(source, file = '(unknown)', { lineNumbers } = {}) {
  const md = restore(downdoc(protect(source, file, lineNumbers)))
  sanityCheck(source, md, file)
  return md.endsWith('\n') ? md : md + '\n'
}
