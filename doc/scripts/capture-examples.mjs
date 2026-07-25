// 6·7장(HTML/CSS 첫걸음) 그림 재촬영 스크립트.
//
// 원고의 연습용 예제(scripts/examples/)를 headless Chrome으로 열어 찍는다.
// 앱 캡처(capture-screenshots.mjs)와 달리 로그인도 서버도 필요 없다.
//
// 사용 절차:
//   1. doc/ 에서 실행: node scripts/capture-examples.mjs
//   2. 합성·축소(ImageMagick)까지 스크립트가 함께 처리해
//      doc/public/screenshots/ 에 아래 세 장을 남긴다.
//        html-first.png       그림 5  첫 문서
//        html-form.png        그림 6  폼과 입력 칸
//        css-before-after.png 그림 8  스타일 적용 전 / 후 / 다크 모드
//
// 예제 파일을 고치면 이 스크립트를 다시 돌린다. 원고의 코드 블록과 예제 파일이
// 어긋나면 그림만 조용히 틀려지므로, 코드 블록을 고칠 때 예제 파일도 함께 고친다.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import puppeteer from 'puppeteer-core';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const EXAMPLES = path.join(HERE, 'examples');
const RAW = process.env.OUT_DIR ?? '/tmp/fc-example-raw';
const OUT = path.join(HERE, '..', 'public', 'screenshots');
const CHROME = process.env.CHROME_PATH ?? '/usr/bin/google-chrome';

fs.mkdirSync(RAW, { recursive: true });

const browser = await puppeteer.launch({
  executablePath: CHROME,
  headless: 'new',
  args: ['--no-sandbox', '--font-render-hinting=none', '--force-device-scale-factor=1', '--lang=ko-KR'],
  // 파일 선택 버튼처럼 브라우저가 직접 그리는 글자를 한국어로 얻으려면 로케일까지 넘겨야 한다.
  env: { ...process.env, LANG: 'ko_KR.UTF-8', LANGUAGE: 'ko' },
});

// 예제 파일 하나를 고정 크기로 찍는다. css를 주면 스타일시트를 얹고,
// dark를 주면 운영체제가 어두운 테마를 고른 상태를 흉내 낸다.
async function shot(name, file, { width, height, css = null, dark = false }) {
  const page = await browser.newPage();
  await page.setViewport({ width, height, deviceScaleFactor: 2 });
  if (dark) {
    await page.emulateMediaFeatures([{ name: 'prefers-color-scheme', value: 'dark' }]);
  }
  await page.goto('file://' + path.join(EXAMPLES, file), { waitUntil: 'load' });
  if (css) await page.addStyleTag({ path: path.join(EXAMPLES, css) });
  await new Promise(r => setTimeout(r, 200));
  await page.screenshot({ path: `${RAW}/${name}.png` });
  await page.close();
  console.log('captured', name);
}

await shot('first', 'first.html', { width: 480, height: 140 });
await shot('form', 'form.html', { width: 480, height: 380 });
await shot('plain', 'page.html', { width: 330, height: 500 });
await shot('light', 'page.html', { width: 330, height: 500, css: 'style.css' });
await shot('dark', 'page.html', { width: 330, height: 500, css: 'style.css', dark: true });

await browser.close();

const magick = (args) => execFileSync('convert', args, { stdio: 'inherit' });

magick([`${RAW}/first.png`, '-resize', '700x', '-strip', `${OUT}/html-first.png`]);
magick([`${RAW}/form.png`, '-resize', '700x', '-strip', `${OUT}/html-form.png`]);
// 배경색이 거의 같은 화면이 이어 붙으므로 칸마다 얇은 테두리로 경계를 남긴다.
const panel = (name) => ['(', `${RAW}/${name}.png`, '-resize', '500x', '-bordercolor', '#d4d4d4', '-border', '1', ')'];
magick([...panel('plain'), ...panel('light'), ...panel('dark'), '+append', '-strip', `${OUT}/css-before-after.png`]);

console.log('done');
