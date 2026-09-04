// 원고를 VitePress 소스(.generated/)로 펼친다.
// - toc의 각 장: .adoc이면 include 해석 → downdoc 파이프라인으로 md 변환, .md면 그대로 복사
// - index.md: book.config의 cover 데이터에서 생성
// - public: 책의 public/ 디렉터리를 심링크로 연결
import { mkdir, readFile, rm, symlink, writeFile } from 'node:fs/promises'
import { existsSync, lstatSync } from 'node:fs'
import { dirname, join, relative } from 'node:path'
import { convertAdoc } from './adoc.mjs'
import { homeCoverMarkdown } from './cover.mjs'
import { resolveIncludes } from './include.mjs'
import { applyMetricRefs } from './metrics.mjs'
import { applyChapterRefs, numberTitle } from './refs.mjs'
import { chapterLabels, flattenChapters } from './toc.mjs'

export const GEN_DIR = '.generated'

// toc 항목의 실제 원고 파일을 찾는다. 선언된 확장자가 우선이고,
// 전환기(md → adoc 이행 중)를 위해 반대 확장자도 받아 준다.
export function chapterSources(root, book) {
  return flattenChapters(book).map(({ file, route }) => {
    const exact = join(root, file)
    const alt = file.endsWith('.adoc')
      ? exact.replace(/\.adoc$/, '.md')
      : exact.replace(/\.md$/, '.adoc')
    const src = existsSync(exact) ? exact : existsSync(alt) ? alt : null
    if (!src) throw new Error(`원고 파일을 찾지 못했다: ${file}`)
    return { file, route, src }
  })
}

// generateChapter writes one chapter and reports the files it quoted with
// include:: (book dev watches them alongside the manuscript).
// book이 있으면 toc에서 장 라벨을 파생해 {ch-…} 토큰을 치환하고 제목에 번호를 붙인다.
export async function generateChapter(root, { file, route, src }, book) {
  const out = join(root, GEN_DIR, route + '.md')
  await mkdir(dirname(out), { recursive: true })
  const raw = await readFile(src, 'utf8')
  if (!src.endsWith('.adoc')) {
    await writeFile(out, raw)
    return []
  }
  const { text, lineNumbers, deps } = resolveIncludes(raw, src, root)
  const at = relative(root, src)
  let manuscript = text
  if (book) {
    const labels = chapterLabels(book)
    const key = route.replace(/^.*\//, '')
    manuscript = applyMetricRefs(text, book.metrics, at, lineNumbers)
    manuscript = numberTitle(applyChapterRefs(manuscript, labels, at, lineNumbers), labels.get(key))
  }
  await writeFile(out, convertAdoc(manuscript, at, { lineNumbers }))
  return deps
}

async function linkPublic(root) {
  const pub = join(root, 'public')
  if (!existsSync(pub)) return
  const link = join(root, GEN_DIR, 'public')
  try {
    if (lstatSync(link)) return
  } catch {}
  await symlink('../public', link, 'dir')
}

// generateAll rebuilds every chapter and returns the union of the files the
// manuscripts quote with include::.
export async function generateAll(root, book, { clean = false } = {}) {
  const gen = join(root, GEN_DIR)
  if (clean) await rm(gen, { recursive: true, force: true })
  await mkdir(gen, { recursive: true })
  const quoted = new Set()
  for (const chapter of chapterSources(root, book)) {
    for (const dep of await generateChapter(root, chapter, book)) quoted.add(dep)
  }
  await writeFile(join(gen, 'index.md'), homeCoverMarkdown(book))
  await linkPublic(root)
  return [...quoted]
}
