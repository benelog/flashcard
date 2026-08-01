#!/usr/bin/env node
// 장별 분량을 재서 마크다운 리포트로 낸다.
//
//   node scripts/chapter-stats.mjs                        # 표준 출력으로 리포트
//   node scripts/chapter-stats.mjs --out chapter-dist.md
//   node scripts/chapter-stats.mjs --json                 # 기계가 읽을 형태
//   node scripts/chapter-stats.mjs --sections part3/gin.adoc   # 한 장의 절별 분량
//
// 파일 크기(바이트)로 재면 실제 분량과 어긋난다. 원고가 코드를 베끼지 않고
// `include::`로 인용하므로, 인용 한 줄이 실제로는 코드 수십 줄이 되기 때문이다.
// 그래서 여기서는 인용을 펼친 뒤 본문과 코드를 따로 센다.

import { readFileSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const DOC = resolve(dirname(fileURLToPath(import.meta.url)), '..')

// 환산 쪽수의 기준. 종이책 한 쪽에 들어가는 양의 어림값이라 정확한 조판 결과는
// 아니다. 장끼리 견주는 것이 목적이므로 값을 바꾸면 순위가 아니라 눈금만 바뀐다.
const PER_PAGE = {
  prose: 800, // 공백을 뺀 본문 글자 수
  code: 34, // 코드 블록 줄 수
  figure: 0.5, // 그림 하나가 차지하는 쪽
}

// ---------------------------------------------------------------- 인용 펼치기

const includeRe = /^include::([^[]+)\[([^\]]*)\]\s*$/

// `// tag::이름[]` … `// end::이름[]` 사이를 꺼낸다. 주석 기호는 언어마다 달라
// 아무 문자로 시작해도 되게 두고, 마커 줄 자체와 다른 태그의 마커 줄은 버린다.
function extractTag(source, tag) {
  const begin = new RegExp(`\\btag::${tag}\\[\\]`)
  const end = new RegExp(`\\bend::${tag}\\[\\]`)
  const anyMarker = /\b(tag|end)::[\w-]+\[\]/
  const out = []
  let inside = false
  for (const line of source.split('\n')) {
    if (!inside && begin.test(line)) {
      inside = true
      continue
    }
    if (inside && end.test(line)) break
    if (inside && !anyMarker.test(line)) out.push(line)
  }
  return out
}

function expandInclude(line, baseDir) {
  const m = line.match(includeRe)
  if (!m) return null
  const [, path, attrs] = m
  let source
  try {
    source = readFileSync(resolve(baseDir, path.trim()), 'utf8')
  } catch {
    return { lines: [], missing: path.trim() }
  }
  const tag = attrs.match(/tag=([\w-]+)/)?.[1]
  const lines = tag
    ? extractTag(source, tag)
    : source.split('\n').filter((l) => !/\b(tag|end)::[\w-]+\[\]/.test(l))
  // 앞뒤 빈 줄은 결과에서 빠진다(asciidoctor와 같은 규칙).
  while (lines.length && !lines[0].trim()) lines.shift()
  while (lines.length && !lines.at(-1).trim()) lines.pop()
  return { lines, missing: null }
}

// ------------------------------------------------------------------ 세는 일

// 글자 수를 셀 때 마크업은 빼고 표시되는 글자만 남긴다.
function plainText(line) {
  return line
    .replace(/https?:\/\/\S*?\[([^\]]*)\]/g, '$1') // 링크 매크로는 표시 문구만
    .replace(/\{ch-[^}]+\}/g, '00장') // 장 참조 토큰은 두 글자쯤
    .replace(/[`*_^~|]/g, '')
    .replace(/\s+/g, '')
}

function analyze(file) {
  const path = resolve(DOC, file)
  const baseDir = dirname(path)
  const raw = readFileSync(path, 'utf8')
  const stat = {
    file,
    bytes: Buffer.byteLength(raw),
    prose: 0, // 본문 글자 수(공백 제외)
    codeOwn: 0, // 원고에 직접 쓴 코드 줄
    codeCited: 0, // include로 끌어온 코드 줄
    sections: 0, // == 절
    subsections: 0, // === 소절
    figures: 0,
    tables: 0,
    includes: 0,
    missing: [],
  }

  // 절(`==`) 단위 분량도 함께 모은다. 큰 장을 어디서 나눌지 볼 때 쓴다.
  stat.bySection = []
  let cur = { title: '(장 도입부)', prose: 0, code: 0, figures: 0, subsections: 0 }
  const closeSection = () => {
    cur.pages =
      cur.prose / PER_PAGE.prose + cur.code / PER_PAGE.code + cur.figures * PER_PAGE.figure
    stat.bySection.push(cur)
  }

  let inCode = false
  for (const line of raw.split('\n')) {
    const t = line.trim()

    if (t.startsWith('----')) {
      inCode = !inCode
      continue
    }
    if (inCode) {
      const inc = expandInclude(t, baseDir)
      if (inc) {
        stat.includes++
        stat.codeCited += inc.lines.length
        cur.code += inc.lines.length
        if (inc.missing) stat.missing.push(inc.missing)
      } else if (t) {
        stat.codeOwn++
        cur.code++
      }
      continue
    }

    if (!t) continue
    if (t.startsWith('//')) continue // 원고 주석
    if (/^image::/.test(t)) {
      stat.figures++
      cur.figures++
      continue
    }
    if (t === '|===') {
      stat.tables++
      continue
    }
    if (/^\[[^\]]*\]$/.test(t)) continue // [source,go] 같은 블록 속성

    if (/^={2}\s/.test(t)) {
      stat.sections++
      closeSection()
      cur = { title: t.replace(/^==\s*/, ''), prose: 0, code: 0, figures: 0, subsections: 0 }
    } else if (/^={3}\s/.test(t)) {
      stat.subsections++
      cur.subsections++
    }

    cur.prose += plainText(t).length
    stat.prose += plainText(t).length
  }
  closeSection()

  stat.tables = Math.floor(stat.tables / 2) // |===는 열고 닫으며 두 번 나온다
  stat.code = stat.codeOwn + stat.codeCited
  stat.pages =
    stat.prose / PER_PAGE.prose + stat.code / PER_PAGE.code + stat.figures * PER_PAGE.figure
  return stat
}

// ------------------------------------------------------------------- 리포트

const n = (x, d = 0) =>
  x.toLocaleString('ko-KR', { maximumFractionDigits: d, minimumFractionDigits: d })
const bar = (pages) => '█'.repeat(Math.max(1, Math.round(pages / 1.5)))

// stdout에 동기로 쓴다. console.log 바로 뒤에 process.exit()을 부르면 파이프로
// 나가던 긴 출력(--json은 60KB가 넘는다)이 중간에서 잘린다.
const printSync = (text) => writeFileSync(1, text + '\n')

// 한 장의 절별 분량만 낸다. 큰 장을 나눌 자리를 찾을 때 쓴다.
function sectionsReport(target) {
  const stat = analyze(target)
  const out = [
    `${target} — 전체 ${n(stat.pages, 1)}쪽`,
    '',
    '| 절 | 환산쪽 | 본문자 | 코드줄 | 소절 | 분포 |',
    '|---|---:|---:|---:|---:|---|',
  ]
  for (const s of stat.bySection) {
    if (!s.prose && !s.code) continue
    out.push(
      `| ${s.title} | ${n(s.pages, 1)} | ${n(s.prose)} | ${n(s.code)} | ${s.subsections} | ${bar(s.pages)} |`,
    )
  }
  return out.join('\n')
}

const secIdx = process.argv.indexOf('--sections')
if (secIdx !== -1 && process.argv[secIdx + 1]) {
  printSync(sectionsReport(process.argv[secIdx + 1]))
  process.exit(0)
}

const { default: config } = await import(resolve(DOC, 'book.config.mjs'))

// toc에서 장 목록을 읽는다. 번호도 toc 제목("15장 …")에서 그대로 가져온다.
const chapters = []
for (const group of config.toc) {
  for (const item of group.items) {
    const label = item.text.match(/^(\S+장|부록|저자 서문|이 책을 읽는 법|무엇을)/)?.[0] ?? ''
    chapters.push({
      part: group.text,
      label: item.text.match(/^(\d+)장/)?.[1] ?? label,
      title: item.text.replace(/^\d+장\s*/, ''),
      special: !/^\d+장/.test(item.text), // 서문·읽는 법·도입·부록
      ...analyze(item.file),
    })
  }
}

const numbered = chapters.filter((c) => !c.special)
const sum = (xs, f) => xs.reduce((a, x) => a + f(x), 0)
const mean = sum(numbered, (c) => c.pages) / numbered.length
const sorted = [...numbered].sort((a, b) => a.pages - b.pages)
const median = sorted[Math.floor(sorted.length / 2)].pages
const sd = Math.sqrt(sum(numbered, (c) => (c.pages - mean) ** 2) / numbered.length)

if (process.argv.includes('--json')) {
  printSync(JSON.stringify({ chapters, mean, median, sd }, null, 2))
  process.exit(0)
}

const lines = []
lines.push('## 측정 결과')
lines.push('')
lines.push(
  `\`node scripts/chapter-stats.mjs\`가 만든 표다. 원고가 코드를 \`include::\`로 인용하므로 파일 크기로는 실제 분량을 알 수 없다. 인용을 펼친 뒤 본문 글자와 코드 줄을 따로 셌다.`,
)
lines.push('')
lines.push(
  `환산 쪽수는 본문 ${PER_PAGE.prose}자 / 코드 ${PER_PAGE.code}줄 / 그림 ${1 / PER_PAGE.figure}장을 각각 한 쪽으로 잡은 어림값이다. 조판 결과가 아니라 장끼리 견주기 위한 눈금이다.`,
)
lines.push('')
lines.push(`번호 있는 장 ${numbered.length}개 기준: 평균 ${n(mean, 1)}쪽, 중앙값 ${n(median, 1)}쪽, 표준편차 ${n(sd, 1)}쪽 (최소 ${n(sorted[0].pages, 1)} ~ 최대 ${n(sorted.at(-1).pages, 1)}쪽, ${n(sorted.at(-1).pages / sorted[0].pages, 1)}배).`)
lines.push('')

lines.push('## 전체 표')
lines.push('')
lines.push('| 장 | 제목 | 환산쪽 | 본문자 | 코드줄 | 인용 | 직접 | 절 | 그림 | 표 | 분포 |')
lines.push('|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|')
let part = null
for (const c of chapters) {
  if (c.part !== part) {
    part = c.part
    lines.push(`| | **${part}** | | | | | | | | | |`)
  }
  lines.push(
    `| ${c.label} | ${c.title} | ${n(c.pages, 1)} | ${n(c.prose)} | ${n(c.code)} | ${n(c.codeCited)} | ${n(c.codeOwn)} | ${c.sections} | ${c.figures} | ${c.tables} | ${bar(c.pages)} |`,
  )
}
lines.push('')

lines.push('## 부별 합계')
lines.push('')
lines.push('| 부 | 장 수 | 환산쪽 합 | 장당 평균 | 본문자 합 | 코드줄 합 |')
lines.push('|---|---:|---:|---:|---:|---:|')
for (const group of config.toc) {
  const cs = chapters.filter((c) => c.part === group.text)
  lines.push(
    `| ${group.text} | ${cs.length} | ${n(sum(cs, (c) => c.pages), 1)} | ${n(sum(cs, (c) => c.pages) / cs.length, 1)} | ${n(sum(cs, (c) => c.prose))} | ${n(sum(cs, (c) => c.code))} |`,
  )
}
lines.push('')

const big = numbered.filter((c) => c.pages > mean * 1.4).sort((a, b) => b.pages - a.pages)
const small = numbered.filter((c) => c.pages < mean * 0.6).sort((a, b) => a.pages - b.pages)
lines.push('## 평균에서 먼 장')
lines.push('')
lines.push(`- 큰 쪽(평균의 1.4배 초과): ${big.map((c) => `${c.label}장 ${c.title.split(':')[0]} ${n(c.pages, 1)}쪽`).join(', ') || '없음'}`)
lines.push(`- 작은 쪽(평균의 0.6배 미만): ${small.map((c) => `${c.label}장 ${c.title.split(':')[0]} ${n(c.pages, 1)}쪽`).join(', ') || '없음'}`)
lines.push('')

const missing = chapters.filter((c) => c.missing.length)
if (missing.length) {
  lines.push('## 찾지 못한 인용 파일')
  lines.push('')
  for (const c of missing) lines.push(`- ${c.file}: ${[...new Set(c.missing)].join(', ')}`)
  lines.push('')
}

const report = lines.join('\n')
const BEGIN = '<!-- chapter-stats:begin -->'
const END = '<!-- chapter-stats:end -->'

const outIdx = process.argv.indexOf('--out')
if (outIdx === -1 || !process.argv[outIdx + 1]) {
  printSync(report)
  process.exit(0)
}

// 리포트 파일에는 손으로 쓴 진단·제안도 함께 산다. 다시 재도 그 글이 날아가지
// 않도록 마커 사이만 갈아 끼운다. 마커가 없으면(첫 실행) 파일을 새로 만든다.
const outPath = resolve(process.cwd(), process.argv[outIdx + 1])
const block = `${BEGIN}\n\n${report}\n${END}`
let existing = null
try {
  existing = readFileSync(outPath, 'utf8')
} catch {
  /* 아직 없는 파일 */
}
if (existing && existing.includes(BEGIN) && existing.includes(END)) {
  const head = existing.slice(0, existing.indexOf(BEGIN))
  const tail = existing.slice(existing.indexOf(END) + END.length)
  writeFileSync(outPath, head + block + tail)
} else {
  writeFileSync(outPath, `# 장별 분량 비교\n\n${block}\n`)
}
console.error(`wrote ${process.argv[outIdx + 1]}`)
