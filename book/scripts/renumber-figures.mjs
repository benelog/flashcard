// 그림 번호를 목차 순서로 다시 매긴다.
// 캡션 `<p class="fc-caption">그림 N …</p>`의 N을 책 전체에서 1부터 순서대로 바꾼다.
// 본문에서 그림을 가리키는 `그림 N`(같은 파일, 캡션 앞 한 문단)은 캡션과 같은 번호를 받는다.
// 새 그림은 `그림 N`이라고 적어 두면 이 스크립트가 번호를 채운다.
import { readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import config from '../book.config.mjs';

const root = resolve(import.meta.dirname, '..');
const files = [];
for (const part of config.toc) for (const item of part.items) files.push(item.file);

let n = 0;
for (const file of files) {
  const path = resolve(root, file);
  let text = readFileSync(path, 'utf8');
  const before = text;
  // 캡션이 나오는 순서대로 번호를 붙인다. 캡션 앞의 본문 참조(그림 N 또는 이미 붙은 번호)는
  // "직전 캡션 이후 ~ 이 캡션" 구간에서만 바꾼다.
  const re = /<p class="fc-caption">그림 (N|\d+)/g;
  let out = '';
  let last = 0;
  let m;
  while ((m = re.exec(text))) {
    n += 1;
    const seg = text.slice(last, m.index);
    out += seg.replace(/그림 (N|\d+)(?=\D)/g, `그림 ${n}`);
    out += `<p class="fc-caption">그림 ${n}`;
    last = m.index + m[0].length;
  }
  out += text.slice(last);
  if (out !== before) writeFileSync(path, out);
}
console.log(`그림 ${n}장에 번호를 매겼다.`);
