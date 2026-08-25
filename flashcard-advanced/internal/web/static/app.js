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

function englishVoice() {
  return (
    voices.find((v) => v.lang === "en-US" && v.name.includes("Google")) ||
    voices.find((v) => v.lang === "en-US") ||
    voices.find((v) => v.lang.startsWith("en")) ||
    null
  );
}

function speak(text, rate) {
  if (!("speechSynthesis" in window) || !text) return;
  speechSynthesis.cancel(); // Chrome은 취소하지 않으면 큐에 쌓인다
  const u = new SpeechSynthesisUtterance(text);
  u.lang = "en-US";
  u.rate = rate || 0.9;
  u.voice = englishVoice();
  speechSynthesis.speak(u);
}

// 스토리 듣기. 카드 읽어주기와 다루는 법이 둘 달라 따로 둔다.
// 1) 본문이 길다: 발화 하나로 넘기면 Chrome이 중간에 끊으므로 문장으로 잘라
//    큐에 넣는다. 잘린 조각의 참조를 배열에 쥐고 있어야 하는데, 참조가 끊긴
//    발화는 Chrome이 수거해 end 이벤트가 오지 않기 때문이다.
// 2) 한글 제목·설명이 섞인다: 영어 목소리로 읽으면 알아들을 수 없으므로
//    한글이 든 조각만 한국어로 읽는다.
const hangul = /[가-힣ㄱ-ㅎㅏ-ㅣ]/;
let storyChunks = [];

function splitStory(text) {
  return (text.match(/[^.!?\n]+[.!?]*/g) || []).map((s) => s.trim()).filter(Boolean);
}

function stopStory() {
  if (!("speechSynthesis" in window)) return;
  speechSynthesis.cancel();
  storyChunks = [];
  document.querySelectorAll("[data-playing]").forEach((el) => delete el.dataset.playing);
}

function speakStory(text, rate, button) {
  if (!("speechSynthesis" in window)) return;
  stopStory();
  storyChunks = splitStory(text).map((chunk) => {
    const u = new SpeechSynthesisUtterance(chunk);
    u.rate = rate || 0.9;
    if (hangul.test(chunk)) {
      u.lang = "ko-KR";
    } else {
      u.lang = "en-US";
      u.voice = englishVoice();
    }
    return u;
  });
  const last = storyChunks[storyChunks.length - 1];
  if (!last) return;
  last.addEventListener("end", stopStory);
  last.addEventListener("error", stopStory);
  button.dataset.playing = "true";
  storyChunks.forEach((u) => speechSynthesis.speak(u));
}

// 다른 화면으로 떠나도 읽던 소리는 계속 나므로 여기서 끊는다.
addEventListener("pagehide", stopStory);

// data-* 속성으로만 연결: 서버가 렌더링한(htmx로 갈아끼운) HTML에도 그대로 동작한다.
document.addEventListener("click", (e) => {
  const story = e.target.closest("[data-tts-story]");
  if (story) {
    e.preventDefault();
    if (story.dataset.playing) {
      stopStory(); // 읽는 중에 다시 누르면 멈춘다
      return;
    }
    const body = document.getElementById(story.dataset.ttsStory);
    speakStory(body ? body.textContent : "", parseFloat(story.dataset.ttsRate), story);
    return;
  }

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
