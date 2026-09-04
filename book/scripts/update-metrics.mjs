#!/usr/bin/env node
// 원고가 인용하는 저장소 수치를 다시 세어 book/metrics.json에 적는다.
//
//   npm run metrics          값을 다시 세어 파일에 쓰고 바뀐 것을 보고한다
//   npm run metrics -- --check   쓰지 않고, 어긋난 것이 있으면 1로 끝난다 (빌드 전 검사)
//
// 원고 본문은 {n-이름} 토큰만 쓰고, 빌드가 이 파일의 값으로 치환한다.
// 수치의 정의(무엇을 어떤 범위에서 세는가)는 metrics.def.mjs에 있다.

import { readFileSync, writeFileSync } from 'node:fs'
import { definitions, korean, withCommas } from './metrics.def.mjs'

const OUT = new URL('../metrics.json', import.meta.url)
const check = process.argv.includes('--check')

// 한 정의에서 실제로 쓰이는 항목들을 만든다. 천 단위가 넘으면 쉼표를 넣고,
// korean: true면 한글 수사 두 형태(명사형·관형사형)를 함께 낸다.
function entriesFor(name, def) {
  const n = def.measure()
  if (!Number.isInteger(n) || n < 0) {
    throw new Error(`${name}: 측정 결과가 0 이상의 정수가 아니다 (${n})`)
  }
  const out = [[name, withCommas(n)]]
  if (def.korean) {
    const noun = korean(n, {})
    const attr = korean(n, { attributive: true })
    if (!noun || !attr) {
      throw new Error(`${name}: ${n}은 고유어 수사로 적을 수 있는 범위(1~99)를 벗어났다`)
    }
    out.push([`${name}-ko`, noun], [`${name}-ko-attr`, attr])
  }
  return out
}

const measured = {}
const failures = []
for (const [name, def] of Object.entries(definitions)) {
  try {
    for (const [k, v] of entriesFor(name, def)) measured[k] = v
  } catch (e) {
    failures.push(`  ${name}: ${e.message}`)
  }
}
if (failures.length) {
  console.error('수치를 세지 못했다:\n' + failures.join('\n'))
  process.exit(1)
}

const sorted = Object.fromEntries(Object.keys(measured).sort().map((k) => [k, measured[k]]))

let previous = {}
try {
  previous = JSON.parse(readFileSync(OUT, 'utf8'))
} catch {}

const changed = Object.keys(sorted).filter((k) => previous[k] !== sorted[k])
const removed = Object.keys(previous).filter((k) => !(k in sorted))

if (!changed.length && !removed.length) {
  console.log(`수치 ${Object.keys(sorted).length}개가 모두 저장소와 맞는다.`)
  process.exit(0)
}

for (const k of changed) {
  const was = previous[k] === undefined ? '(없음)' : previous[k]
  console.log(`  ${k}: ${was} → ${sorted[k]}`)
}
for (const k of removed) console.log(`  ${k}: ${previous[k]} → (정의에서 빠짐)`)

if (check) {
  console.error('\nmetrics.json이 저장소와 어긋난다. `npm run metrics`를 돌려 갱신하고,')
  console.error('바뀐 수치가 원고 문장의 논지와도 맞는지 확인하라.')
  process.exit(1)
}

writeFileSync(OUT, JSON.stringify(sorted, null, 2) + '\n')
console.log(`\nmetrics.json을 갱신했다(${changed.length + removed.length}곳).`)
console.log('원고에서 이 수치를 쓰는 문장이 새 값에서도 말이 되는지 확인하라.')
