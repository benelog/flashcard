// 빌드 결과(dist)를 base 경로 그대로 서빙하는 임시 정적 서버.
// PDF 인쇄와 OG 이미지 캡처가 함께 쓴다 (cleanUrls 대응 라우팅 포함).
import { createServer } from 'node:http'
import { readFile, stat } from 'node:fs/promises'
import { existsSync } from 'node:fs'
import { extname, join } from 'node:path'

const MIME = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript',
  '.css': 'text/css',
  '.json': 'application/json',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.woff2': 'font/woff2',
  '.ttf': 'font/ttf',
  '.ico': 'image/x-icon',
}

// mounts: dist 밖의 디렉터리를 덧붙여 서빙한다 ({ '/경로/': 디렉터리 }). POD 글꼴 캐시가 쓴다.
export async function serveDist(dist, base, mounts = {}) {
  const server = createServer(async (req, res) => {
    const path = decodeURIComponent(new URL(req.url, 'http://localhost').pathname)
    const mount = Object.keys(mounts).find((prefix) => path.startsWith(prefix))
    if (mount) {
      const file = join(mounts[mount], path.slice(mount.length))
      if (!existsSync(file)) {
        res.writeHead(404)
        return res.end()
      }
      res.writeHead(200, { 'content-type': MIME[extname(file)] ?? 'application/octet-stream' })
      return res.end(await readFile(file))
    }
    if (!path.startsWith(base)) {
      res.writeHead(404)
      return res.end()
    }
    let file = join(dist, path.slice(base.length) || 'index.html')
    if (!existsSync(file) || (await stat(file)).isDirectory()) {
      if (existsSync(file + '.html')) file += '.html'
      else if (existsSync(join(file, 'index.html'))) file = join(file, 'index.html')
      else {
        res.writeHead(404)
        return res.end()
      }
    }
    res.writeHead(200, { 'content-type': MIME[extname(file)] ?? 'application/octet-stream' })
    res.end(await readFile(file))
  })
  await new Promise((resolve) => server.listen(0, resolve))
  return { port: server.address().port, close: () => server.close() }
}

// 시스템 Chrome 실행 파일을 찾는다 (puppeteer-core는 브라우저를 내려받지 않는다)
export function findChrome() {
  const chrome =
    process.env.PUPPETEER_EXECUTABLE_PATH ??
    [
      '/usr/bin/google-chrome-stable',
      '/usr/bin/google-chrome',
      '/usr/bin/chromium-browser',
      '/usr/bin/chromium',
      '/opt/google/chrome/chrome',
    ].find(existsSync)
  if (!chrome) throw new Error('Chrome 실행 파일을 찾지 못했다 (PUPPETEER_EXECUTABLE_PATH로 지정 가능)')
  return chrome
}
