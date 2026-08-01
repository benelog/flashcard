/* Flashcard에 남은 유일한 자바스크립트.
   앱 로직은 전부 Go 서버에 있고, 이 파일은 브라우저에서만 접근할 수 있는
   API(음성 합성, 클립보드, 온라인 상태, 서비스 워커, 시간대)만 감싼다. */

// 시간대: 서버가 "오늘"의 경계와 통계 날짜를 사용자 기준으로 계산하도록 알린다.
document.cookie =
  "tz=" +
  encodeURIComponent(Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC") +
  ";path=/;max-age=31536000;samesite=lax";

// PWA: 서비스 워커 등록 (localhost 포함, http에서는 브라우저가 거부한다).
if ("serviceWorker" in navigator) {
  navigator.serviceWorker.register("/sw.js").catch(() => {});
}

// 로그아웃: 서비스 워커가 오프라인용으로 갖고 있던 사용자별 페이지 HTML을 지운다.
// 서버는 이 페이지들에 no-store를 붙이지만 캐시 저장소는 그와 무관하게 남는다.
// 서버가 /login?signed_out=1 로 보내 준 것이 신호다(sw.js의 PAGES 캐시 접두사).
if (new URLSearchParams(location.search).has("signed_out")) {
  if ("caches" in window) {
    caches
      .keys()
      .then((keys) =>
        Promise.all(
          keys
            .filter((k) => k.startsWith("flashcard-pages"))
            .map((k) => caches.delete(k)),
        ),
      )
      .catch(() => {});
  }
  history.replaceState(null, "", location.pathname); // 주소창에서 흔적을 지운다
}

// 오프라인 배너.
const banner = document.getElementById("offline-banner");
if (banner) {
  const update = () => banner.classList.toggle("show", !navigator.onLine);
  addEventListener("online", update);
  addEventListener("offline", update);
  update();
}

// TTS: Web Speech API로 영어 읽어주기. Chrome은 목소리 목록을 비동기로 채운다.
let voices = [];
if ("speechSynthesis" in window) {
  const load = () => (voices = speechSynthesis.getVoices());
  speechSynthesis.addEventListener("voiceschanged", load);
  load();
}

function speak(text, rate) {
  if (!("speechSynthesis" in window) || !text) return;
  speechSynthesis.cancel(); // Chrome은 취소하지 않으면 큐에 쌓인다
  const u = new SpeechSynthesisUtterance(text);
  u.lang = "en-US";
  u.rate = rate || 0.9;
  u.voice =
    voices.find((v) => v.lang === "en-US" && v.name.includes("Google")) ||
    voices.find((v) => v.lang === "en-US") ||
    voices.find((v) => v.lang.startsWith("en")) ||
    null;
  speechSynthesis.speak(u);
}

// data-* 속성으로만 연결: 서버가 렌더링한(htmx로 갈아끼운) HTML에도 그대로 동작한다.
document.addEventListener("click", (e) => {
  const tts = e.target.closest("[data-tts], [data-tts-from]");
  if (tts) {
    e.preventDefault(); // 카드 뒤집기(label) 등 부모 동작을 막는다
    e.stopPropagation();
    const from = tts.dataset.ttsFrom && document.getElementById(tts.dataset.ttsFrom);
    speak(from ? from.value : tts.dataset.tts, parseFloat(tts.dataset.ttsRate));
    return;
  }

  const copy = e.target.closest("[data-copy]");
  if (copy) {
    e.preventDefault();
    navigator.clipboard
      .writeText(copy.dataset.copy)
      .then(() => (copy.dataset.done = "true"))
      .catch(() => prompt("아래 링크를 복사하세요", copy.dataset.copy));
  }
});

// 설정 화면: 읽기 속도 슬라이더의 현재 값 표시와 "들어보기".
document.addEventListener("input", (e) => {
  const range = e.target.closest("[data-range-out]");
  if (!range) return;
  const out = document.getElementById(range.dataset.rangeOut);
  if (out) out.textContent = Number(range.value).toFixed(1);
});
document.addEventListener("click", (e) => {
  const test = e.target.closest("[data-tts-test]");
  if (!test) return;
  e.preventDefault();
  const range = document.getElementById(test.dataset.ttsTest);
  speak("The quick brown fox jumps over the lazy dog.", parseFloat(range?.value));
});
