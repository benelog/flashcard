package app

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/benelog/flashcard/internal/model"
)

// 로그인한 사용자가 여는 화면은 전부 200이어야 한다. 템플릿이 참조하는 값 하나가
// 사라지면 여기서 500으로 잡힌다.
func TestPagesRender(t *testing.T) {
	a := newTestApp(t)
	slug := a.makeDeck("Verbs")
	a.makeCard(slug, "run", "달리다")

	for _, path := range []string{
		"/", "/decks", "/decks/" + slug, "/decks/" + slug + "/cards/new",
		"/stats", "/settings", "/shared", "/study",
	} {
		t.Run(path, func(t *testing.T) {
			mustStatus(t, a.get(path), http.StatusOK)
		})
	}
}

func TestDeckAndCardLifecycle(t *testing.T) {
	a := newTestApp(t)

	slug := a.makeDeck("Verbs")
	mustContain(t, a.get("/decks"), "Verbs")

	a.makeCard(slug, "run", "달리다")
	deckPage := a.get("/decks/" + slug)
	mustContain(t, deckPage, "run")
	mustContain(t, deckPage, "달리다")

	card := a.cards(slug)[0]
	mustContain(t, a.get("/cards/"+card.ID.String()), "달리다")

	// 수정은 카드 목록으로 되돌려 보낸다.
	rec := a.postForm("/cards/"+card.ID.String(), url.Values{
		"text":      {"run"},
		"meaning":   {"뛰다"},
		"card_type": {model.CardTypeSentence},
		"tags":      {"동사, 기초"},
	})
	if to := mustRedirect(t, rec); to != "/decks/"+slug {
		t.Errorf("update redirected to %q, want /decks/%s", to, slug)
	}
	updated := a.cards(slug)[0]
	if updated.Meaning != "뛰다" || updated.CardType != model.CardTypeSentence {
		t.Errorf("card = %+v, want 뛰다/sentence", updated)
	}
	if len(updated.Tags) != 2 || updated.Tags[0] != "동사" {
		t.Errorf("tags = %v, want [동사 기초]", updated.Tags)
	}

	// htmx 삭제는 조각을 지우도록 빈 200을 돌려준다.
	del := a.postHTMX("/cards/"+card.ID.String()+"/delete", nil)
	mustStatus(t, del, http.StatusOK)
	if body := strings.TrimSpace(del.Body.String()); body != "" {
		t.Errorf("htmx delete body = %q, want empty", body)
	}
	if got := a.cards(slug); len(got) != 0 {
		t.Errorf("deck still has %d cards after delete", len(got))
	}
}

// 원문이나 뜻이 비면 저장하지 않고 폼으로 되돌려 보낸다.
func TestCardFormRequiresTextAndMeaning(t *testing.T) {
	a := newTestApp(t)
	slug := a.makeDeck("Verbs")

	rec := a.postForm("/decks/"+slug+"/cards", url.Values{"text": {"run"}, "meaning": {"  "}})
	if to := mustRedirect(t, rec); to != "/decks/"+slug+"/cards/new" {
		t.Errorf("redirected to %q, want the form again", to)
	}
	if got := a.cards(slug); len(got) != 0 {
		t.Fatalf("saved %d cards, want none", len(got))
	}
}

// 남의 덱이나 없는 덱, 손으로 고친 주소는 모두 404다.
func TestUnknownURLsAre404(t *testing.T) {
	a := newTestApp(t)

	for _, path := range []string{
		"/decks/zzzz",       // 형식은 맞지만 없는 슬러그
		"/decks/!!",         // 슬러그 형식도 아님
		"/cards/not-a-uuid", // UUID가 아닌 카드 주소
		"/shared/nope",      // 공유가 풀린 덱
		"/그런페이지없어요",         // 아예 없는 경로
	} {
		t.Run(path, func(t *testing.T) {
			mustStatus(t, a.get(path), http.StatusNotFound)
		})
	}
}

// API 경로의 404는 HTML이 아니라 JSON이어야 한다.
func TestUnknownAPIPathReturnsJSON(t *testing.T) {
	a := newTestApp(t)
	rec := a.get("/api/no-such-thing")
	mustStatus(t, rec, http.StatusNotFound)
	mustContain(t, rec, `"error"`)
	mustNotContain(t, rec, "<html")
}

// 학습 한 판을 처음부터 끝까지 돈다. 상태는 서버가 아니라 폼의 hidden 필드에
// 실려 오가므로, 매 단계 응답에서 필드를 그대로 꺼내 다음 요청에 싣는다.
func TestStudySessionFullRound(t *testing.T) {
	a := newTestApp(t)
	slug := a.makeDeck("Verbs")
	a.makeCard(slug, "run", "달리다")
	a.makeCard(slug, "walk", "걷다")

	deck, err := a.store.GetDeckBySlug(t.Context(), a.userID, slug)
	if err != nil {
		t.Fatal(err)
	}

	// 방향을 고르지 않으면 먼저 방향 선택 화면이 나온다.
	chooser := a.get("/study?mode=deck&deckId=" + deck.ID.String())
	mustStatus(t, chooser, http.StatusOK)
	mustContain(t, chooser, "어느 방향으로 학습할까요?")
	// 두 링크 모두 나머지 질의 문자열을 물고 가야 한다.
	mustContain(t, chooser, "deckId="+deck.ID.String())

	first := a.get("/study?mode=deck&direction=text_to_meaning&deckId=" + deck.ID.String())
	mustStatus(t, first, http.StatusOK)
	state := hiddenFields(t, first)
	if got := state.Get("round"); got != "1" {
		t.Fatalf("round = %q, want 1", got)
	}
	if n := len(strings.Split(state.Get("queue"), ",")); n != 2 {
		t.Fatalf("queue has %d cards, want 2", n)
	}

	// 첫 장은 맞히고 둘째 장은 틀린다.
	state.Set("correct", "true")
	second := a.postHTMX("/study/grade", state)
	mustStatus(t, second, http.StatusOK)

	state = hiddenFields(t, second)
	if got := state.Get("fp_correct"); got != "1" {
		t.Fatalf("fp_correct = %q, want 1 after one right answer", got)
	}
	state.Set("correct", "false")
	afterRound := a.postHTMX("/study/grade", state)
	mustStatus(t, afterRound, http.StatusOK)

	// 틀린 카드가 있으니 "다시 풀기" 국면이다.
	mustContain(t, afterRound, "다시 풀기")
	state = hiddenFields(t, afterRound)
	if got := state.Get("missed"); got == "" {
		t.Fatal("missed queue is empty, want the wrong card")
	}

	nextRound := a.postHTMX("/study/next-round", state)
	mustStatus(t, nextRound, http.StatusOK)
	state = hiddenFields(t, nextRound)
	if got := state.Get("round"); got != "2" {
		t.Fatalf("round = %q, want 2", got)
	}

	state.Set("correct", "true")
	finished := a.postHTMX("/study/grade", state)
	mustStatus(t, finished, http.StatusOK)
	mustContain(t, finished, "학습 완료")

	// 1차 정답률은 재도전 결과에 흔들리지 않는다: 두 장 중 한 장이라 50%다.
	summary, err := a.store.StatsSummary(t.Context(), a.userID, "UTC", time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalReviews != 2 || summary.CorrectReviews != 1 {
		t.Errorf("first-pass reviews = %d/%d, want 1/2", summary.CorrectReviews, summary.TotalReviews)
	}
}

// 카드가 한 장도 없으면 채점 화면 대신 안내를 보여 준다.
func TestStudyWithNoCards(t *testing.T) {
	a := newTestApp(t)
	rec := a.get("/study?direction=text_to_meaning")
	mustStatus(t, rec, http.StatusOK)
	mustContain(t, rec, "학습할 카드가 없어요")
}

func TestStudyRejectsBadModeAndRule(t *testing.T) {
	a := newTestApp(t)

	for _, path := range []string{
		"/study?direction=text_to_meaning&mode=nonsense",
		"/study?direction=text_to_meaning&mode=deck&deckId=not-a-uuid",
		"/study?direction=text_to_meaning&mode=smart&rule={}",
	} {
		t.Run(path, func(t *testing.T) {
			mustStatus(t, a.get(path), http.StatusNotFound)
		})
	}
}

// 그만두기는 세션을 미완료로 닫고 원래 있던 곳으로 돌려보낸다. return_url은
// 폼에서 오므로 다른 사이트로 튕겨 나가지 않는지도 함께 본다.
func TestQuitStudyReturnsToSafePath(t *testing.T) {
	a := newTestApp(t)
	slug := a.makeDeck("Verbs")
	a.makeCard(slug, "run", "달리다")

	state := hiddenFields(t, a.get("/study?direction=text_to_meaning"))
	state.Set("return_url", "https://evil.example.com/")
	if to := mustRedirect(t, a.postForm("/study/quit", state)); to != "/" {
		t.Errorf("quit redirected to %q, want /", to)
	}
}

// 내보낸 CSV를 다른 덱으로 그대로 가져올 수 있어야 한다.
func TestCSVExportImportRoundTrip(t *testing.T) {
	a := newTestApp(t)
	source := a.makeDeck("Verbs")
	a.makeCard(source, "run", "달리다")
	a.makeCard(source, "walk", "걷다")

	exported := a.get("/decks/" + source + "/export")
	mustStatus(t, exported, http.StatusOK)
	if got := exported.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/csv") {
		t.Errorf("Content-Type = %q, want text/csv", got)
	}
	mustContain(t, exported, "달리다")

	target := a.makeDeck("Copy")
	rec := a.upload("/decks/"+target+"/import", "file", "deck.csv", exported.Body.String())
	mustRedirect(t, rec)

	imported := a.cards(target)
	if len(imported) != 2 {
		t.Fatalf("imported %d cards, want 2", len(imported))
	}

	// 같은 파일을 한 번 더 가져오면 전부 중복으로 걸러진다.
	a.upload("/decks/"+target+"/import", "file", "deck.csv", exported.Body.String())
	if got := a.cards(target); len(got) != 2 {
		t.Errorf("re-import left %d cards, want 2 (duplicates skipped)", len(got))
	}
}

func TestCSVImportRejectsUnusableFile(t *testing.T) {
	a := newTestApp(t)
	slug := a.makeDeck("Verbs")

	rec := a.upload("/decks/"+slug+"/import", "file", "deck.csv", "a,b\n1,2\n")
	mustRedirect(t, rec)
	if got := a.cards(slug); len(got) != 0 {
		t.Errorf("imported %d cards from a headerless file, want 0", len(got))
	}
}

// 공유를 켜면 갤러리에 뜨고, 다른 사람이 자기 덱으로 복사해 갈 수 있다. 공유를
// 풀면 그 링크는 곧바로 죽는다.
func TestShareAndImportSharedDeck(t *testing.T) {
	a := newTestApp(t)
	slug := a.makeDeck("Verbs")
	a.makeCard(slug, "run", "달리다")

	mustRedirect(t, a.postForm("/decks/"+slug+"/share", nil))
	deck, err := a.store.GetDeckBySlug(t.Context(), a.userID, slug)
	if err != nil {
		t.Fatal(err)
	}
	if deck.ShareSlug == nil {
		t.Fatal("deck has no share slug after sharing")
	}
	shareSlug := *deck.ShareSlug

	gallery := a.get("/shared")
	mustStatus(t, gallery, http.StatusOK)
	mustContain(t, gallery, "Verbs")

	page := a.get("/shared/" + shareSlug)
	mustStatus(t, page, http.StatusOK)
	mustContain(t, page, "달리다")

	to := mustRedirect(t, a.postForm("/shared/"+shareSlug+"/import", nil))
	copySlug := strings.TrimPrefix(to, "/decks/")
	if copySlug == to || copySlug == slug {
		t.Fatalf("import redirected to %q, want a new deck", to)
	}
	if got := a.cards(copySlug); len(got) != 1 {
		t.Errorf("copied deck has %d cards, want 1", len(got))
	}

	mustRedirect(t, a.postForm("/decks/"+slug+"/unshare", nil))
	mustStatus(t, a.get("/shared/"+shareSlug), http.StatusNotFound)
}

func TestDeleteDeckRemovesItsCards(t *testing.T) {
	a := newTestApp(t)
	slug := a.makeDeck("Verbs")
	a.makeCard(slug, "run", "달리다")

	rec := a.postHTMX("/decks/"+slug+"/delete", nil)
	mustStatus(t, rec, http.StatusOK)
	if to := rec.Header().Get("HX-Redirect"); to != "/decks" {
		t.Errorf("HX-Redirect = %q, want /decks", to)
	}
	mustStatus(t, a.get("/decks/"+slug), http.StatusNotFound)
}

// 설정은 범위를 벗어난 값을 조용히 기본값으로 되돌린다.
func TestSaveSettings(t *testing.T) {
	a := newTestApp(t)

	mustRedirect(t, a.postForm("/settings", url.Values{
		"display_name": {"벤"},
		"tts_rate":     {"1.2"},
		"daily_goal":   {"30"},
	}))
	page := a.get("/settings")
	mustContain(t, page, "벤")
	mustContain(t, page, "1.2")
	mustContain(t, page, "30")

	mustRedirect(t, a.postForm("/settings", url.Values{
		"display_name": {"벤"},
		"tts_rate":     {"99"},   // 최대 1.5
		"daily_goal":   {"9999"}, // 최대 200
	}))
	mustContain(t, a.get("/settings"), "0.9") // 기본 속도
}

// 스마트 덱은 학습 화면의 저장 버튼(htmx)으로도, 덱 목록에서도 만들어진다.
func TestSmartDeckSaveAndDelete(t *testing.T) {
	a := newTestApp(t)

	rec := a.postHTMX("/smart-decks", url.Values{
		"name": {"오답 모음"},
		"rule": {`{"type":"high_error"}`},
	})
	mustStatus(t, rec, http.StatusOK)
	mustContain(t, rec, "스마트 덱으로 저장했어요")
	mustContain(t, a.get("/decks"), "오답 모음")

	decks, err := a.store.ListSmartDecks(t.Context(), a.userID)
	if err != nil || len(decks) != 1 {
		t.Fatalf("ListSmartDecks = %d decks, %v; want 1", len(decks), err)
	}
	mustStatus(t, a.postHTMX("/smart-decks/"+decks[0].ID.String()+"/delete", nil), http.StatusOK)

	if decks, _ := a.store.ListSmartDecks(t.Context(), a.userID); len(decks) != 0 {
		t.Errorf("smart deck survived delete")
	}
}

func TestSmartDeckRejectsBadRule(t *testing.T) {
	a := newTestApp(t)
	mustRedirect(t, a.postForm("/smart-decks", url.Values{
		"name": {"엉터리"},
		"rule": {`{"type":"nonsense"}`},
	}))
	if decks, _ := a.store.ListSmartDecks(t.Context(), a.userID); len(decks) != 0 {
		t.Errorf("saved %d smart decks from an unknown rule type, want 0", len(decks))
	}
}

// 개인 화면은 절대 공유 캐시에 들어가면 안 된다.
func TestPagesAreNotCacheable(t *testing.T) {
	a := newTestApp(t)
	if got := a.get("/").Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}

// 정적 자원은 내용 해시가 붙은 주소일 때만 영구 캐시가 된다.
func TestStaticAssetCaching(t *testing.T) {
	a := newTestApp(t)

	home := a.get("/")
	versioned := assetURL(t, home.Body.String(), "/static/app.css")
	if got := a.get(versioned).Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Errorf("versioned asset Cache-Control = %q, want immutable", got)
	}
	if got := a.get("/static/app.css").Header().Get("Cache-Control"); strings.Contains(got, "immutable") {
		t.Errorf("bare asset Cache-Control = %q, must not be immutable", got)
	}
}

func assetURL(t *testing.T, page, path string) string {
	t.Helper()
	i := strings.Index(page, path+"?v=")
	if i < 0 {
		t.Fatalf("page does not reference %s with a version", path)
	}
	rest := page[i:]
	return rest[:strings.IndexAny(rest, `"'`)]
}
