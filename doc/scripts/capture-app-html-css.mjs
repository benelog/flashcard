// 16장(HTML과 CSS) 그림 재촬영 스크립트.
//
// 앱 캡처(capture-screenshots.mjs)와 달리 배포본이 아니라 로컬 서버를 찍는다.
// 16장 그림은 개인 학습 데이터가 아니라 CSS 동작(알약 라디오, 다크 모드)을 보이는 것이라
// 로그인 없는 SQLite 로컬 모드로 충분하고, 촬영 전체가 자동화된다.
//
// 사용 절차:
//   1. 캡처용 빈 DB로 서버를 띄운다(개인 학습 데이터가 섞이지 않게 local-db를 쓰지 않는다):
//        cd .. && env -u DATABASE_URL SQLITE_PATH=/tmp/fc-book-capture.db PORT=8099 go run ./cmd/server
//   2. 데모 덱 "TOEIC 필수 단어"(카드 8장)를 만든다:
//        curl -X POST --data-urlencode "name=TOEIC 필수 단어" http://localhost:8099/decks
//        curl -X POST -F "file=@demo.csv;type=text/csv" http://localhost:8099/decks/<slug>/import
//      demo.csv의 열은 text,meaning,type,phonetic,example 이다.
//   3. doc/ 에서 실행: DECK_SLUG=<slug> node scripts/capture-app-html-css.mjs
//      doc/public/screenshots/ 에 아래 두 장을 남긴다.
//        app-card-form.png   그림 9   카드 입력 화면(알약 라디오)
//        app-light-dark.png  그림 10  같은 홈 화면의 밝은 테마와 어두운 테마
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import puppeteer from 'puppeteer-core';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const BASE = process.env.CAPTURE_BASE ?? 'http://localhost:8099';
const DECK_SLUG = process.env.DECK_SLUG ?? 'm8xj';
const RAW = process.env.OUT_DIR ?? '/tmp/fc-app-raw';
const OUT = path.join(HERE, '..', 'public', 'screenshots');
const CHROME = process.env.CHROME_PATH ?? '/usr/bin/google-chrome';

fs.mkdirSync(RAW, { recursive: true });

const browser = await puppeteer.launch({
  executablePath: CHROME,
  headless: 'new',
  args: ['--no-sandbox', '--font-render-hinting=none', '--lang=ko-KR'],
});

async function shot(name, url, { dark = false, height = 860, before = null } = {}) {
  const page = await browser.newPage();
  await page.setViewport({ width: 430, height, deviceScaleFactor: 3, isMobile: true, hasTouch: true });
  if (dark) {
    await page.emulateMediaFeatures([{ name: 'prefers-color-scheme', value: 'dark' }]);
  }
  await page.goto(BASE + url, { waitUntil: 'networkidle2', timeout: 30000 });
  if (before) await before(page);
  await new Promise(r => setTimeout(r, 400));
  await page.screenshot({ path: `${RAW}/${name}.png` });
  await page.close();
  console.log('captured', name);
}

// 카드 입력 화면: 카드 종류를 "숙어"로 바꿔 두어 선택된 알약만 파랗게 칠해지는 것을 보인다.
await shot('card-form', `/decks/${DECK_SLUG}/cards/new`, {
  height: 470,
  before: async (page) => {
    await page.evaluate(() => {
      // 라디오는 투명하게 숨겨져 있어 좌표 클릭이 닿지 않는다.
      document.querySelector('.pill-radio[value="idiom"]').click();
      document.querySelector('textarea[name="text"]').value = 'take into account';
      document.querySelector('textarea[name="meaning"]').value = '~을 고려하다';
    });
  },
});

await shot('home-light', '/', { height: 720 });
await shot('home-dark', '/', { height: 720, dark: true });

await browser.close();

const magick = (args) => execFileSync('convert', args, { stdio: 'inherit' });

magick([`${RAW}/card-form.png`, '-resize', '700x', '-strip', `${OUT}/app-card-form.png`]);
magick([
  '(', `${RAW}/home-light.png`, '-resize', '650x', ')',
  '(', `${RAW}/home-dark.png`, '-resize', '650x', ')',
  '+append', '-strip', `${OUT}/app-light-dark.png`,
]);

console.log('done');
