// 원고가 인용하는 "저장소를 센 수"의 정의.
//
// 원고 본문에 이 수를 직접 적으면 코드가 자랄 때마다 원고가 조용히 틀린 말을
// 하게 된다. 빌드도 테스트도 이것을 잡아 주지 않는다. 그래서 본문에는
// {n-이름} 토큰만 두고, 값은 metrics.json 한 곳에 모으며, 그 값은
// `npm run metrics`가 저장소를 직접 세어 갱신한다.
//
// 새 수치를 더할 때 지켜야 할 것이 둘 있다.
//
// 1. measure()는 원고 문장이 말하는 범위와 정확히 같아야 한다.
//    "테스트 파일을 뺀 Go 코드"와 "internal의 Go 코드"는 다른 수다.
//    desc에 그 범위를 적어 두면 나중에 문장과 대조할 수 있다.
// 2. 토큰은 수만 치환하고 단위(줄·개·장)는 원고에 남긴다.
//    단위가 뒤따르는 조사를 정하므로, 값이 바뀌어도 조사가 깨지지 않는다.
//
// korean: true를 주면 `이름-ko`(명사형: 열셋)와 `이름-ko-attr`(관형사형: 열세)이
// 함께 생긴다. 원고가 한글 수사로 쓰는 자리를 위한 것이다.

import { execFileSync } from 'node:child_process'
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join } from 'node:path'

// 저장소 루트에서 본 완성본 앱. 이 파일은 book/scripts/에 있다.
const APP = new URL('../../flashcard-advanced/', import.meta.url).pathname

const at = (...p) => join(APP, ...p)

// 디렉터리를 뺀 파일만 센다(static/의 icons/ 같은 하위 디렉터리 제외).
const filesIn = (dir) =>
  readdirSync(at(dir)).filter((n) => statSync(join(at(dir), n)).isFile())

// 테스트를 뺀 .go 파일을 모은다.
function goFiles(sub = '.') {
  const out = []
  const walk = (dir) => {
    for (const n of readdirSync(dir, { withFileTypes: true })) {
      const p = join(dir, n.name)
      if (n.isDirectory()) {
        if (n.name === 'vendor' || n.name === 'node_modules') continue
        walk(p)
      } else if (n.name.endsWith('.go') && !n.name.endsWith('_test.go')) {
        out.push(p)
      }
    }
  }
  walk(at(sub))
  return out
}

const linesOf = (files) =>
  files.reduce((n, f) => n + readFileSync(f, 'utf8').split('\n').length - 1, 0)

// 화면 템플릿(레이아웃·페이지·조각) 전체에서 정규식에 걸리는 수를 센다.
function countInTemplates(re) {
  let n = 0
  const walk = (dir) => {
    for (const e of readdirSync(dir, { withFileTypes: true })) {
      const p = join(dir, e.name)
      if (e.isDirectory()) walk(p)
      else if (e.name.endsWith('.html')) n += (readFileSync(p, 'utf8').match(re) ?? []).length
    }
  }
  walk(at('internal/web/templates'))
  return n
}

const packages = () =>
  execFileSync('go', ['list', './...'], { cwd: APP, encoding: 'utf8' })
    .trim()
    .split('\n')

// app.css의 규칙 수와 선택자 종류별 내역. 넷이 서로 비교되는 자리에 쓰이므로
// 한 곳에서 같은 파싱으로 낸다(따로 세면 합이 안 맞을 수 있다).
function cssRuleMetrics() {
  const parse = () => {
    const css = readFileSync(at('internal/web/static/app.css'), 'utf8').replace(/\/\*[\s\S]*?\*\//g, '')
    return [...css.matchAll(/([^{}]+)\{/g)]
      .map((m) => m[1].trim())
      .filter((s) => s && !s.startsWith('@'))
      .map((s) => s.split(',')[0].trim())
  }
  const of = (label, desc, pick) => [
    label,
    { desc, korean: false, measure: () => parse().filter(pick).length },
  ]
  return Object.fromEntries([
    of('css-rules', 'app.css의 규칙(선택자 블록) 수, at-rule 제외', () => true),
    of('css-rules-class', 'app.css에서 첫 선택자가 클래스(.)인 규칙 수', (s) => s.startsWith('.')),
    of('css-rules-tag', 'app.css에서 첫 선택자가 태그나 *인 규칙 수', (s) => /^[a-z*]/.test(s)),
    of('css-rules-id', 'app.css에서 첫 선택자가 id(#)인 규칙 수', (s) => s.startsWith('#')),
  ])
}

export const definitions = {
  'page-count': {
    desc: '완성본 앱의 화면 템플릿 수 (internal/web/templates/pages/*.html)',
    korean: true,
    measure: () => filesIn('internal/web/templates/pages').length,
  },
  'store-methods': {
    desc: '저장소 계약 model.Store 인터페이스의 메서드 수',
    korean: true,
    measure: () => {
      const src = readFileSync(at('internal/model/store.go'), 'utf8')
      const body = src.split('type Store interface {')[1]
      if (!body) throw new Error('model.Store 인터페이스를 찾지 못했다')
      const decl = body.slice(0, body.indexOf('\n}'))
      return decl.split('\n').filter((l) => /^\t[A-Z][A-Za-z0-9]*\(/.test(l)).length
    },
  },
  'packages': {
    desc: 'Go 패키지 수 (go list ./..., cmd/ 진입점 포함)',
    measure: () => packages().length,
  },
  'packages-no-cmd': {
    desc: 'Go 패키지 수에서 cmd/ 아래 main 패키지를 뺀 수',
    korean: true,
    measure: () => packages().filter((p) => !p.includes('/cmd/')).length,
  },
  'go-files': {
    desc: '테스트를 뺀 Go 파일 수 (앱 전체)',
    measure: () => goFiles().length,
  },
  'go-lines': {
    desc: '테스트를 뺀 Go 코드 줄 수 (앱 전체)',
    measure: () => linesOf(goFiles()),
  },
  'go-lines-internal': {
    desc: '테스트를 뺀 Go 코드 줄 수 (internal/ 아래만)',
    measure: () => linesOf(goFiles('internal')),
  },
  'sw-lines': {
    desc: '서비스 워커 internal/web/static/sw.js의 줄 수',
    measure: () => readFileSync(at('internal/web/static/sw.js'), 'utf8').split('\n').length - 1,
  },
  'static-files': {
    desc: 'internal/web/static/ 바로 아래의 파일 수 (icons/ 같은 하위 디렉터리 제외)',
    korean: true,
    measure: () => filesIn('internal/web/static').length,
  },
  'tables': {
    desc: '완성본 스키마의 테이블 수 (뷰 제외)',
    korean: true,
    measure: () => {
      const sql = readFileSync(at('internal/litestore/schema.sql'), 'utf8')
      return (sql.match(/create table (if not exists )?\w+/gi) ?? []).length
    },
  },
  // 화면 이동은 <a>가 맡고 htmx는 부분 갱신에만 쓴다는 서술의 근거다.
  // 둘의 대비가 논지이므로 같은 범위(화면 템플릿 전체)에서 함께 센다.
  'a-href-count': {
    desc: '화면 템플릿에서 href를 단 <a> 수',
    measure: () => countInTemplates(/<a [^>]*href/g),
  },
  'hx-post-count': {
    desc: '화면 템플릿의 hx-post 속성 수',
    measure: () => countInTemplates(/hx-post/g),
  },
  'form-count': {
    desc: '화면 템플릿의 <form> 수',
    measure: () => countInTemplates(/<form/g),
  },
  'srs-lines': {
    desc: '간격 반복 알고리즘 internal/srs/srs.go의 줄 수',
    measure: () => readFileSync(at('internal/srs/srs.go'), 'utf8').split('\n').length - 1,
  },
  'app-test-count': {
    desc: 'pkg/app 통합 테스트의 Test 함수 수',
    korean: true,
    measure: () => {
      let n = 0
      for (const f of readdirSync(at('pkg/app')).filter((x) => x.endsWith('_test.go'))) {
        n += (readFileSync(at('pkg/app', f), 'utf8').match(/^func Test[A-Za-z0-9_]*\(/gm) ?? []).length
      }
      return n
    },
  },
  'basic-routes': {
    desc: '2부 최소 앱(flashcard-basic)의 라우팅 등록 줄 수 = 핸들러 수',
    korean: true,
    measure: () => {
      const src = readFileSync(new URL('../../flashcard-basic/handlers.go', import.meta.url).pathname, 'utf8')
      return (src.match(/^\t*r(outer)?\.(GET|POST|PUT|DELETE)\(/gm) ?? []).length
    },
  },
  'symmetric-store-files': {
    desc: 'litestore와 pgstore에 이름이 같은 비테스트 파일 수 (두 구현의 대칭)',
    korean: true,
    measure: () => {
      const names = (d) => new Set(filesIn(d).filter((n) => n.endsWith('.go') && !n.endsWith('_test.go')))
      const lite = names('internal/litestore')
      return [...names('internal/pgstore')].filter((n) => lite.has(n)).length
    },
  },
  // app.css의 "규칙"은 선택자 하나에 딸린 중괄호 블록이다(@media 같은 at-rule은 뺀다).
  // 분류는 쉼표로 갈린 첫 선택자의 머리글자를 따른다: '.'는 클래스, '#'은 id,
  // 글자와 '*'는 태그. 어디에도 들지 않는 것(`:root`, 속성 선택자)은 세 부류 밖이다.
  ...cssRuleMetrics(),
  'direct-deps': {
    desc: 'go.mod의 직접 의존성 수 (indirect 제외)',
    korean: true,
    measure: () => {
      const mod = readFileSync(at('go.mod'), 'utf8')
      const block = mod.split('require (')[1]?.split('\n)')[0] ?? ''
      return block.split('\n').filter((l) => l.trim() && !l.includes('// indirect')).length
    },
  },
}

// 천 단위 구분 쉼표. 원고는 "6,853줄"처럼 쓴다.
export const withCommas = (n) => n.toLocaleString('en-US')

// 고유어 수사. 99까지만 쓴다(그 위는 원고가 한자어로 적는다).
const TENS = ['', '열', '스물', '서른', '마흔', '쉰', '예순', '일흔', '여든', '아흔']
const UNITS_NOUN = ['', '하나', '둘', '셋', '넷', '다섯', '여섯', '일곱', '여덟', '아홉']
const UNITS_ATTR = ['', '한', '두', '세', '네', '다섯', '여섯', '일곱', '여덟', '아홉']

export function korean(n, { attributive = false } = {}) {
  if (!Number.isInteger(n) || n < 1 || n > 99) return null
  const t = Math.floor(n / 10)
  const u = n % 10
  // '스물'은 단위 명사 앞에서 '스무'가 된다: 스무 개, 스물한 개.
  const tens = attributive && t === 2 && u === 0 ? '스무' : TENS[t]
  return tens + (attributive ? UNITS_ATTR[u] : UNITS_NOUN[u])
}
