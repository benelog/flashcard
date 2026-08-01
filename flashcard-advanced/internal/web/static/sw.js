/* flashcard service worker.
   캐시는 둘로 나눈다. PAGES는 서버가 사용자별로 렌더링한 HTML이라 로그아웃할
   때 app.js가 지운다(서버는 이 응답에 no-store를 붙인다). STATIC은 사용자와
   무관한 자원이라 그대로 둔다. 페이지는 network-first(오프라인일 때만 마지막
   사본, 그것도 없으면 오프라인 안내 페이지), 정적 자원은
   stale-while-revalidate, API는 network only. */
const STATIC = "flashcard-static-v3";
const PAGES = "flashcard-pages-v3"; // 접두사 "flashcard-pages"는 app.js와 약속된 것
const KEEP = [STATIC, PAGES];
const OFFLINE = "/offline.html";

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches
      .open(STATIC)
      .then((cache) => cache.add(OFFLINE))
      .catch(() => {}) // 첫 설치가 오프라인이어도 등록 자체는 실패시키지 않는다
      .then(() => self.skipWaiting()),
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(
          keys.filter((k) => !KEEP.includes(k)).map((k) => caches.delete(k)),
        ),
      )
      .then(() => self.clients.claim()),
  );
});

self.addEventListener("fetch", (event) => {
  const url = new URL(event.request.url);
  const isData =
    url.pathname.startsWith("/api/") || url.origin !== self.location.origin;
  if (event.request.method !== "GET" || isData) return; // network only

  // 서버 렌더링 페이지: 항상 네트워크 우선, 오프라인이면 마지막 사본,
  // 그마저 없으면(한 번도 안 열어 본 주소) 오프라인 안내 페이지.
  if (event.request.mode === "navigate") {
    event.respondWith(
      (async () => {
        const pages = await caches.open(PAGES);
        try {
          const res = await fetch(event.request);
          if (res.ok) pages.put(event.request, res.clone());
          return res;
        } catch {
          const cached = await pages.match(event.request);
          if (cached) return cached;
          const offline = await caches.match(OFFLINE);
          return offline || Response.error();
        }
      })(),
    );
    return;
  }

  // 정적 자원: stale-while-revalidate. URL에 콘텐츠 해시(?v=)가 붙어 있어
  // 배포로 내용이 바뀌면 주소부터 달라진다. 낡은 사본이 새 HTML에 물리지 않는다.
  event.respondWith(
    caches.open(STATIC).then(async (cache) => {
      const cached = await cache.match(event.request);
      const network = fetch(event.request)
        .then((res) => {
          if (res.ok) cache.put(event.request, res.clone());
          return res;
        })
        .catch(() => cached);
      return cached || network;
    }),
  );
});
