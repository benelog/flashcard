// 책에 싣는 앱 화면 전부(그림 1~4, 9~10)를 찍는 스크립트.
//
// 배포본이 아니라 로컬 서버를 찍는다. 로그인이 없는 SQLite 로컬 모드라 서버 기동부터
// 데모 데이터 시드, 학습 세션 진행, 촬영, 합성까지 한 번에 자동으로 끝난다.
// 저자의 실제 학습 데이터가 책에 실리지 않는 것도 이 방식의 이점이다.
//
// 사용 절차:
//   cd book && node scripts/capture-app.mjs
//
// 필요한 것: go, google-chrome, ImageMagick(convert). 포트 8099를 잠깐 쓴다.
// 만드는 파일(book/public/screenshots/):
//   deck-cards.png      그림 1  덱 상세
//   study-flow.png      그림 2  학습 세션 세 단계
//   home.png            그림 3  홈 화면
//   stats-shared.png    그림 4  통계와 공유 덱 갤러리
//   app-card-form.png   그림 9  카드 입력 화면(알약 라디오)
//   app-light-dark.png  그림 10 밝은 테마와 어두운 테마
import { execFileSync, spawn } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { DatabaseSync } from 'node:sqlite';
import { fileURLToPath } from 'node:url';
import puppeteer from 'puppeteer-core';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const APP = path.join(HERE, '..', '..', 'flashcard-advanced');
const OUT = path.join(HERE, '..', 'public', 'screenshots');
const RAW = process.env.OUT_DIR ?? '/tmp/fc-app-raw';
const DB = process.env.CAPTURE_DB ?? '/tmp/fc-book-capture.db';
const PORT = process.env.PORT ?? '8099';
const BASE = `http://localhost:${PORT}`;
const CHROME = process.env.CHROME_PATH ?? '/usr/bin/google-chrome';

fs.mkdirSync(RAW, { recursive: true });
for (const f of [DB, `${DB}-wal`, `${DB}-shm`]) fs.rmSync(f, { force: true });

const sleep = (ms) => new Promise(r => setTimeout(r, ms));

// ── 서버 ───────────────────────────────────────────────────────────────
// `go run`이 아니라 미리 빌드한 바이너리를 띄운다. go run은 중간에 래퍼 프로세스를
// 하나 더 두어, 그것만 죽이면 정작 서버가 살아남아 다음 실행의 DB에 계속 쓴다.
const BIN = '/tmp/fc-capture-server';
execFileSync('go', ['build', '-o', BIN, './cmd/server'], { cwd: APP, stdio: 'inherit' });

const running = new Set();

// DATABASE_URL이 셸에 올라와 있으면 Postgres 모드로 뜨므로 지우고 넘긴다.
function startServer() {
  const env = { ...process.env, SQLITE_PATH: DB, PORT };
  delete env.DATABASE_URL;
  const proc = spawn(BIN, [], { cwd: APP, env, stdio: 'ignore' });
  running.add(proc);
  return proc;
}

async function isUp() {
  try {
    const res = await fetch(BASE + '/', { redirect: 'manual' });
    return res.status < 500;
  } catch {
    return false;
  }
}

async function waitUntilUp(timeoutMs = 60000) {
  const until = Date.now() + timeoutMs;
  while (Date.now() < until) {
    if (await isUp()) return;
    await sleep(300);
  }
  throw new Error('서버가 뜨지 않았다');
}

async function stopServer(proc) {
  proc.kill('SIGTERM');
  running.delete(proc);
  const until = Date.now() + 15000;
  while (Date.now() < until) {
    if (!(await isUp())) return;
    await sleep(300);
  }
  proc.kill('SIGKILL');
  await sleep(500);
}

// ── 데모 데이터 ─────────────────────────────────────────────────────────
async function createDeck(name) {
  const res = await fetch(BASE + '/decks', {
    method: 'POST',
    headers: { 'content-type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams({ name }),
    redirect: 'manual',
  });
  return res.headers.get('location').split('/').pop();
}

async function importCSV(slug, file) {
  const csv = fs.readFileSync(path.join(HERE, 'demo', file));
  const form = new FormData();
  form.append('file', new Blob([csv], { type: 'text/csv' }), file);
  await fetch(`${BASE}/decks/${slug}/import`, { method: 'POST', body: form, redirect: 'manual' });
}

async function shareDeck(slug) {
  await fetch(`${BASE}/decks/${slug}/share`, { method: 'POST', redirect: 'manual' });
}

// ── 학습 기록 ───────────────────────────────────────────────────────────
// 통계 화면에 보일 기록을 만든다. 카드를 실제로 채점해 앱이 남긴 기록이고,
// 며칠에 걸친 모양은 뒤에서 reviewed_at을 옮겨 만든다.
async function studyDeck(page, slug, wrongAt) {
  await page.goto(`${BASE}/decks/${slug}`, { waitUntil: 'networkidle2' });
  const studyUrl = await page.$eval('a[href^="/study"]', a => a.getAttribute('href'));
  const sep = studyUrl.includes('?') ? '&' : '?';
  await page.goto(BASE + studyUrl + sep + 'direction=text_to_meaning', { waitUntil: 'networkidle2' });

  for (let i = 0; ; i++) {
    const hasCard = await page.$('#reveal');
    if (!hasCard) break;
    await page.click('label.reveal-btn');
    await page.click(`.grade-grid button[value="${wrongAt.includes(i) ? 'false' : 'true'}"]`);
    await sleep(700);
  }
}

// 하루치 기록만으로는 연속 학습과 30일 그래프가 한 칸짜리가 된다.
// 앱이 남긴 기록의 날짜만 뒤로 옮겨 사흘에 걸쳐 공부한 모양으로 만든다.
function spreadReviewsOverDays() {
  const db = new DatabaseSync(DB);
  const ids = db.prepare('select id from review_logs order by id').all().map(r => r.id);
  const stamp = (daysAgo) => {
    const d = new Date();
    d.setUTCDate(d.getUTCDate() - daysAgo);
    return d.toISOString().replace(/\.\d{3}Z$/, '.000Z');
  };
  const update = db.prepare('update review_logs set reviewed_at = ? where id = ?');
  ids.forEach((id, i) => {
    // 앞쪽 기록일수록 오래전 것으로 민다: 이틀 전, 하루 전, 오늘.
    const daysAgo = i < ids.length / 3 ? 2 : i < (ids.length * 2) / 3 ? 1 : 0;
    update.run(stamp(daysAgo), id);
  });
  db.close();
}

// ── 촬영 ────────────────────────────────────────────────────────────────
async function newPage(browser, { height = 860, dark = false } = {}) {
  const page = await browser.newPage();
  await page.setViewport({ width: 430, height, deviceScaleFactor: 3, isMobile: true, hasTouch: true });
  if (dark) await page.emulateMediaFeatures([{ name: 'prefers-color-scheme', value: 'dark' }]);
  return page;
}

async function shot(page, name) {
  await sleep(400);
  await page.screenshot({ path: `${RAW}/${name}.png` });
  console.log('captured', name);
}

async function shotPage(browser, name, url, opts = {}) {
  const page = await newPage(browser, opts);
  await page.goto(BASE + url, { waitUntil: 'networkidle2' });
  if (opts.before) await opts.before(page);
  await shot(page, name);
  await page.close();
}

// ── 실행 ────────────────────────────────────────────────────────────────
// 지난 실행이 남긴 서버가 같은 포트를 잡고 있으면 그쪽으로 요청이 흘러 데이터가
// 누적된다. 촬영을 시작하기 전에 확인하고 멈춘다.
if (await isUp()) throw new Error(`${PORT} 포트에 이미 무언가 떠 있다. 그것을 끄고 다시 실행한다`);

const server = startServer();
try {
  await waitUntilUp();
  console.log('서버 기동');

  // 공유는 덱 상세를 찍은 뒤에 건다. 공유 중인 덱은 링크 버튼과 안내 문구가
  // 화면 위쪽을 차지해 그림 1에서 정작 보여야 할 카드 목록이 밀린다.
  const toeic = await createDeck('TOEIC 필수 단어');
  await importCSV(toeic, 'toeic.csv');
  const terms = await createDeck('웹 개발 용어');
  await importCSV(terms, 'web-terms.csv');
  console.log('데모 덱 준비 완료', { toeic, terms });

  const browser = await puppeteer.launch({
    executablePath: CHROME,
    headless: 'new',
    args: ['--no-sandbox', '--font-render-hinting=none', '--lang=ko-KR'],
    env: { ...process.env, LANG: 'ko_KR.UTF-8', LANGUAGE: 'ko' },
  });

  // 통계용 학습 기록: 여덟 장 중 한 장만 틀린다.
  const studyPage = await newPage(browser);
  await studyDeck(studyPage, toeic, [4]);
  await studyPage.close();
  console.log('학습 기록 생성');

  await browser.close();
  await stopServer(server);
  spreadReviewsOverDays();

  const server2 = startServer();
  await waitUntilUp();
  const browser2 = await puppeteer.launch({
    executablePath: CHROME,
    headless: 'new',
    args: ['--no-sandbox', '--font-render-hinting=none', '--lang=ko-KR'],
    env: { ...process.env, LANG: 'ko_KR.UTF-8', LANGUAGE: 'ko' },
  });

  await shotPage(browser2, 'home', '/', { height: 720 });
  await shotPage(browser2, 'home-dark', '/', { height: 720, dark: true });
  await shotPage(browser2, 'deck', `/decks/${toeic}`, { height: 860 });

  await shareDeck(toeic);
  await shareDeck(terms);
  await shotPage(browser2, 'stats', '/stats', { height: 760 });
  await shotPage(browser2, 'shared', '/shared', { height: 760 });
  await shotPage(browser2, 'card-form', `/decks/${toeic}/cards/new`, {
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

  // 학습 세 단계: 방향 선택 → 앞면 → 뒷면. 채점하지 않고 세션을 접는다.
  const study = await newPage(browser2, { height: 700 });
  await study.goto(`${BASE}/decks/${terms}`, { waitUntil: 'networkidle2' });
  const studyUrl = await study.$eval('a[href^="/study"]', a => a.getAttribute('href'));
  await study.goto(BASE + studyUrl, { waitUntil: 'networkidle2' });
  await shot(study, 'study-direction');
  const sep = studyUrl.includes('?') ? '&' : '?';
  await study.goto(BASE + studyUrl + sep + 'direction=text_to_meaning', { waitUntil: 'networkidle2' });
  await study.waitForSelector('#reveal');
  await shot(study, 'study-front');
  await study.evaluate(() => { document.getElementById('reveal').checked = true; });
  await shot(study, 'study-back');
  await study.evaluate(() => fetch('/study/quit', { method: 'POST' }).catch(() => {}));
  await study.close();

  await browser2.close();
  await stopServer(server2);
} finally {
  for (const proc of running) proc.kill('SIGKILL');
}

// ── 합성 ────────────────────────────────────────────────────────────────
const magick = (args) => execFileSync('convert', args, { stdio: 'inherit' });
const panel = (name, width) => ['(', `${RAW}/${name}.png`, '-resize', `${width}x`, ')'];

magick([`${RAW}/home.png`, '-resize', '700x', '-strip', `${OUT}/home.png`]);
magick([`${RAW}/deck.png`, '-resize', '700x', '-strip', `${OUT}/deck-cards.png`]);
magick([`${RAW}/card-form.png`, '-resize', '700x', '-strip', `${OUT}/app-card-form.png`]);
magick([...panel('study-direction', 600), ...panel('study-front', 600), ...panel('study-back', 600),
  '+append', '-strip', `${OUT}/study-flow.png`]);
magick([...panel('stats', 650), ...panel('shared', 650), '+append', '-strip', `${OUT}/stats-shared.png`]);
magick([...panel('home', 650), ...panel('home-dark', 650), '+append', '-strip', `${OUT}/app-light-dark.png`]);

console.log('done');
