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

    // 표지 카드 위쪽(라벨·제목·부제)을 1200:630 비율로 자를 영역을 계산한다.
    const clip = await page.evaluate(() => {
      document.querySelector('.VPNav')?.remove()
      const card = document.querySelector('.fc-book').getBoundingClientRect()
      const sub = document.querySelector('.fc-book-subtitle').getBoundingClientRect()
      const pad = 18
      const height = sub.bottom + pad - (card.top - pad)
      const width = (height * 1200) / 630
      return {
        x: card.left - (width - card.width) / 2,
        y: card.top - pad,
        width,
        height,
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
