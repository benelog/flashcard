// 수치 토큰({n-이름})을 book.config의 metrics에서 치환한다.
//
// 원고가 저장소 코드를 센 수(파일 줄 수, 메서드 개수, 화면 개수)를 본문에
// 그대로 적으면, 코드가 자랄 때마다 원고가 조용히 틀린 말을 하게 된다.
// 빌드도 테스트도 이것을 잡아 주지 않는다. 대신 {n-이름} 토큰을 쓰면 값이
// metrics 한 곳에만 있고, 그 값은 스크립트가 저장소를 직접 세어 갱신한다.
//
//   {n-sw-lines}줄       → "76줄"
//   {n-store-methods}개  → "35개"
//
// 토큰은 수(數)만 치환하고 단위(줄·개·장)는 원고에 남긴다. 단위가 조사를
// 정하므로, 값이 바뀌어도 뒤따르는 조사가 깨지지 않는다.
// 코드 블록(----, ...., ++++) 안은 치환하지 않는다.

import { replaceTokens } from './tokens.mjs'

const TOKEN = /\{n-([a-z0-9-]+)\}/g

export function applyMetricRefs(source, metrics, file, lineNumbers) {
  return replaceTokens(
    source,
    TOKEN,
    (name) => {
      const value = metrics?.[name]
      if (value === undefined) {
        throw new Error(`metrics에 없는 수치 참조다: {n-${name}}`)
      }
      return String(value)
    },
    { file, lineNumbers, label: '수치 토큰 오류' },
  )
}
