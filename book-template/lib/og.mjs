// SNS 공유 미리보기용 Open Graph 이미지(1200×630)를 만든다.
// 빌드된 홈의 책 표지에서 제목·부제 영역을 잘라 public/og-image.png로 저장한다.
// 저장 후 다시 빌드해야 dist에 포함된다. 표지 디자인이나 제목이 바뀌면 재생성한다.
import { join } from 'node:path'
import puppeteer from 'puppeteer-core'
import { findChrome, serveDist } from './server.mjs'

export async function exportOg(root, book) {
  const dist = join(root, '.vitepress/dist')
  const out = join(root, 'public/og-image.png')
  const { port, close } = await serveDist(dist, book.base)

  const browser = await puppeteer.launch({ executablePath: findChrome(), args: ['--no-sandbox'] })
  try {
    const page = await browser.newPage()
    await page.setViewport({ width: 1300, height: 1000, deviceScaleFactor: 1 })
    await page.goto(`http://127.0.0.1:${port}${book.base}`, { waitUntil: 'networkidle0' })
    await page.evaluateHandle('document.fonts.ready')

    // 표지 카드 위쪽(제목·부제·그림의 첫 판)을 1200:630 비율로 자를 영역을 계산한다.
    // 아래 경계를 첫 판에 맞춰, 판이나 라벨이 가로로 잘린 채 걸리지 않게 한다.
    // 가로는 카드가 여백까지 통째로 들어가는 폭 밑으로는 내려가지 않는다(제목이 잘린다).
    const clip = await page.evaluate(() => {
      document.querySelector('.VPNav')?.remove()
      // 자를 영역이 카드보다 넓으므로 옆에 놓인 책 설명·버튼이 화면에 걸린다. 먼저 걷어 낸다
      // (카드 위치가 달라지므로 크기를 재기 전에 지운다).
      document.querySelector('.fc-home-side')?.remove()
      const card = document.querySelector('.fc-book').getBoundingClientRect()
      const first = document.querySelector('.fc-layer').getBoundingClientRect()
      const pad = 18
      const block = first.bottom + pad * 2 - (card.top - pad) // 제목부터 첫 판까지의 높이
      const width = Math.max(card.width + pad * 2, (block * 1200) / 630)
      return {
        x: card.left + card.width / 2 - width / 2,
        y: card.top - pad,
        width,
        height: (width * 630) / 1200,
      }
    })

    // deviceScaleFactor로 확대해 잘라낸 결과가 정확히 1200×630 픽셀이 되게 한다.
    await page.setViewport({ width: 1300, height: 1000, deviceScaleFactor: 1200 / clip.width })
    await new Promise((r) => setTimeout(r, 300))
    await page.screenshot({ path: out, clip })
    console.log(
      `OG 이미지 생성 완료: ${out} (${Math.round(clip.width)}×${Math.round(clip.height)} CSS px → 1200×630)`,
    )
  } finally {
    await browser.close()
    close()
  }
}
