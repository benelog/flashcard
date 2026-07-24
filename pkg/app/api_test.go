package app

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/store"
)

func TestHealthz(t *testing.T) {
	a := newTestApp(t)
	rec := a.get("/api/healthz")
	mustStatus(t, rec, http.StatusOK)
	mustContain(t, rec, `"ok":true`)
}

func TestAPIDeckCRUD(t *testing.T) {
	a := newTestApp(t)

	created := a.sendJSON(http.MethodPost, "/api/decks", `{"name":"Verbs","description":"불규칙"}`)
	mustStatus(t, created, http.StatusCreated)
	deck := decodeJSON[store.Deck](t, created)
	if deck.Slug == "" || deck.Name != "Verbs" {
		t.Fatalf("created deck = %+v", deck)
	}

	got := decodeJSON[store.Deck](t, a.sendJSON(http.MethodGet, "/api/decks/"+deck.Slug, ""))
	if got.ID != deck.ID {
		t.Errorf("GET deck = %v, want %v", got.ID, deck.ID)
	}

	renamed := decodeJSON[store.Deck](t,
		a.sendJSON(http.MethodPatch, "/api/decks/"+deck.Slug, `{"name":"Irregular verbs"}`))
	if renamed.Name != "Irregular verbs" {
		t.Errorf("PATCH name = %q", renamed.Name)
	}
	// 설명은 건드리지 않았으니 그대로 남아 있어야 한다.
	if renamed.Description == nil || *renamed.Description != "불규칙" {
		t.Errorf("PATCH dropped the description: %v", renamed.Description)
	}

	list := decodeJSON[[]store.Deck](t, a.sendJSON(http.MethodGet, "/api/decks", ""))
	if len(list) != 1 {
		t.Fatalf("GET decks = %d decks, want 1", len(list))
	}

	mustStatus(t, a.sendJSON(http.MethodDelete, "/api/decks/"+deck.Slug, ""), http.StatusNoContent)
	mustStatus(t, a.sendJSON(http.MethodGet, "/api/decks/"+deck.Slug, ""), http.StatusNotFound)
}

func TestAPIDeckValidation(t *testing.T) {
	a := newTestApp(t)

	tests := []struct {
		name         string
		method, path string
		body         string
		want         int
	}{
		{"이름 없는 덱", http.MethodPost, "/api/decks", `{"name":"  "}`, http.StatusBadRequest},
		{"깨진 JSON", http.MethodPost, "/api/decks", `{`, http.StatusBadRequest},
		{"없는 덱 조회", http.MethodGet, "/api/decks/zzzz", "", http.StatusNotFound},
		{"없는 덱 삭제", http.MethodDelete, "/api/decks/zzzz", "", http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mustStatus(t, a.sendJSON(tt.method, tt.path, tt.body), tt.want)
		})
	}
}

func TestAPICardCRUD(t *testing.T) {
	a := newTestApp(t)
	deck := decodeJSON[store.Deck](t, a.sendJSON(http.MethodPost, "/api/decks", `{"name":"Verbs"}`))

	created := a.sendJSON(http.MethodPost, "/api/cards",
		`{"deckSlug":"`+deck.Slug+`","text":" run ","meaning":"달리다","cardType":"word","tags":["동사"]}`)
	mustStatus(t, created, http.StatusCreated)
	card := decodeJSON[store.Card](t, created)
	// 앞뒤 공백은 저장 전에 걷어낸다.
	if card.Text != "run" {
		t.Errorf("card.Text = %q, want %q", card.Text, "run")
	}

	updated := decodeJSON[store.Card](t, a.sendJSON(http.MethodPatch, "/api/cards/"+card.ID.String(),
		`{"text":"run","meaning":"뛰다","cardType":"sentence"}`))
	if updated.Meaning != "뛰다" || updated.CardType != store.CardTypeSentence {
		t.Errorf("updated card = %+v", updated)
	}

	cards := decodeJSON[[]store.Card](t, a.sendJSON(http.MethodGet, "/api/decks/"+deck.Slug+"/cards", ""))
	if len(cards) != 1 {
		t.Fatalf("deck has %d cards, want 1", len(cards))
	}

	mustStatus(t, a.sendJSON(http.MethodDelete, "/api/cards/"+card.ID.String(), ""), http.StatusNoContent)
	mustStatus(t, a.sendJSON(http.MethodGet, "/api/cards/"+card.ID.String(), ""), http.StatusNotFound)
}

// API는 폼·CSV와 달리 모르는 카드 종류를 조용히 고치지 않고 400으로 알린다.
func TestAPICardValidation(t *testing.T) {
	a := newTestApp(t)
	deck := decodeJSON[store.Deck](t, a.sendJSON(http.MethodPost, "/api/decks", `{"name":"Verbs"}`))

	tests := []struct {
		name string
		body string
		want int
	}{
		{"뜻이 빠짐", `{"deckSlug":"` + deck.Slug + `","text":"run"}`, http.StatusBadRequest},
		{"모르는 종류", `{"deckSlug":"` + deck.Slug + `","text":"run","meaning":"달리다","cardType":"banana"}`, http.StatusBadRequest},
		{"없는 덱", `{"deckSlug":"zzzz","text":"run","meaning":"달리다"}`, http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mustStatus(t, a.sendJSON(http.MethodPost, "/api/cards", tt.body), tt.want)
		})
	}
	// 종류를 아예 적지 않으면 기본 종류로 채운다.
	created := a.sendJSON(http.MethodPost, "/api/cards",
		`{"deckSlug":"`+deck.Slug+`","text":"walk","meaning":"걷다"}`)
	mustStatus(t, created, http.StatusCreated)
	if got := decodeJSON[store.Card](t, created); got.CardType != store.DefaultCardType {
		t.Errorf("cardType = %q, want the default %q", got.CardType, store.DefaultCardType)
	}
}

func TestAPIBulkCreateSkipsDuplicates(t *testing.T) {
	a := newTestApp(t)
	deck := decodeJSON[store.Deck](t, a.sendJSON(http.MethodPost, "/api/decks", `{"name":"Verbs"}`))

	rec := a.sendJSON(http.MethodPost, "/api/decks/"+deck.Slug+"/cards/bulk", `{"cards":[
		{"text":"run","meaning":"달리다"},
		{"text":" RUN ","meaning":"달리다"},
		{"text":"walk","meaning":"걷다"},
		{"text":"","meaning":"뜻만 있음"}
	]}`)
	mustStatus(t, rec, http.StatusOK)

	got := decodeJSON[struct {
		Added   int `json:"added"`
		Skipped int `json:"skipped"`
		Invalid int `json:"invalid"`
	}](t, rec)
	if got.Added != 2 || got.Skipped != 1 || got.Invalid != 1 {
		t.Errorf("bulk result = %+v, want 2 added / 1 skipped / 1 invalid", got)
	}
}

func TestAPIBulkRejectsEmptyBatch(t *testing.T) {
	a := newTestApp(t)
	deck := decodeJSON[store.Deck](t, a.sendJSON(http.MethodPost, "/api/decks", `{"name":"Verbs"}`))
	mustStatus(t, a.sendJSON(http.MethodPost, "/api/decks/"+deck.Slug+"/cards/bulk", `{"cards":[]}`),
		http.StatusBadRequest)
}

// 세션 하나를 열고 채점을 기록한 뒤 닫는다. 채점 결과가 다음 복습일로 이어지는지가
// 이 API의 핵심이다.
func TestAPIStudySession(t *testing.T) {
	a := newTestApp(t)
	deck := decodeJSON[store.Deck](t, a.sendJSON(http.MethodPost, "/api/decks", `{"name":"Verbs"}`))
	card := decodeJSON[store.Card](t, a.sendJSON(http.MethodPost, "/api/cards",
		`{"deckSlug":"`+deck.Slug+`","text":"run","meaning":"달리다"}`))

	due := decodeJSON[struct {
		Count int `json:"count"`
	}](t, a.sendJSON(http.MethodGet, "/api/due-count", ""))
	if due.Count != 1 {
		t.Fatalf("due count = %d, want 1 (new cards are due at once)", due.Count)
	}

	opened := a.sendJSON(http.MethodPost, "/api/sessions", `{"mode":"due","direction":"text_to_meaning"}`)
	mustStatus(t, opened, http.StatusCreated)
	session := decodeJSON[struct {
		Session store.Session `json:"session"`
		Cards   []store.Card  `json:"cards"`
	}](t, opened)
	if len(session.Cards) != 1 {
		t.Fatalf("session has %d cards, want 1", len(session.Cards))
	}

	graded := a.sendJSON(http.MethodPost, "/api/sessions/"+session.Session.ID.String()+"/reviews",
		`{"cardId":"`+card.ID.String()+`","result":true}`)
	mustStatus(t, graded, http.StatusOK)
	if out := decodeJSON[store.ReviewOutcome](t, graded); out.IntervalDays != 1 {
		t.Errorf("interval = %v, want 1 day after the first correct answer", out.IntervalDays)
	}

	mustStatus(t, a.sendJSON(http.MethodPost, "/api/sessions/"+session.Session.ID.String()+"/finish",
		`{"completed":true}`), http.StatusNoContent)

	// 하루 뒤로 밀렸으니 오늘 볼 카드는 없다.
	if got := decodeJSON[struct {
		Count int `json:"count"`
	}](t, a.sendJSON(http.MethodGet, "/api/due-count", "")); got.Count != 0 {
		t.Errorf("due count after review = %d, want 0", got.Count)
	}
}

func TestAPISessionValidation(t *testing.T) {
	a := newTestApp(t)

	tests := []struct {
		name string
		body string
		want int
	}{
		{"모르는 모드", `{"mode":"nonsense"}`, http.StatusBadRequest},
		{"모르는 방향", `{"mode":"due","direction":"sideways"}`, http.StatusBadRequest},
		{"덱 모드인데 덱 없음", `{"mode":"deck"}`, http.StatusBadRequest},
		{"스마트 모드인데 규칙 없음", `{"mode":"smart"}`, http.StatusBadRequest},
		{"규칙이 엉터리", `{"mode":"smart","rule":{"type":"nonsense"}}`, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mustStatus(t, a.sendJSON(http.MethodPost, "/api/sessions", tt.body), tt.want)
		})
	}

	// 남의(=없는) 세션에 채점을 기록할 수는 없다.
	mustStatus(t, a.sendJSON(http.MethodPost, "/api/sessions/"+uuid.New().String()+"/reviews",
		`{"cardId":"`+uuid.New().String()+`","result":true}`), http.StatusNotFound)
}

func TestAPISmartDecksAndSuggestions(t *testing.T) {
	a := newTestApp(t)

	created := a.sendJSON(http.MethodPost, "/api/smart-decks",
		`{"name":"오답 모음","rule":{"type":"high_error","limit":9999}}`)
	mustStatus(t, created, http.StatusCreated)
	smart := decodeJSON[store.SmartDeck](t, created)

	// 저장되는 규칙은 정규화된 것이라야 한다: limit이 허용 범위로 잘린다.
	var rule struct {
		Limit int `json:"limit"`
	}
	if err := json.Unmarshal(smart.Rule, &rule); err != nil {
		t.Fatal(err)
	}
	if rule.Limit != 20 {
		t.Errorf("stored limit = %d, want the clamped 20", rule.Limit)
	}

	// 아직 푼 카드가 없으니 추천할 것도 없다.
	if got := decodeJSON[[]any](t, a.sendJSON(http.MethodGet, "/api/suggestions", "")); len(got) != 0 {
		t.Errorf("suggestions = %d, want none for a fresh account", len(got))
	}

	mustStatus(t, a.sendJSON(http.MethodDelete, "/api/smart-decks/"+smart.ID.String(), ""), http.StatusNoContent)
	mustStatus(t, a.sendJSON(http.MethodDelete, "/api/smart-decks/"+uuid.New().String(), ""), http.StatusNotFound)
}

func TestAPIProfileAndStats(t *testing.T) {
	a := newTestApp(t)

	updated := decodeJSON[store.Profile](t,
		a.sendJSON(http.MethodPatch, "/api/me", `{"displayName":"벤","settings":{"dailyGoal":30}}`))
	if updated.DisplayName == nil || *updated.DisplayName != "벤" {
		t.Errorf("displayName = %v", updated.DisplayName)
	}
	if got := decodeJSON[store.Profile](t, a.sendJSON(http.MethodGet, "/api/me", "")); string(got.Settings) == "" {
		t.Error("settings came back empty")
	}

	summary := decodeJSON[store.Summary](t, a.sendJSON(http.MethodGet, "/api/stats/summary?tz=Asia/Seoul", ""))
	if summary.TotalReviews != 0 || summary.Streak != 0 {
		t.Errorf("fresh summary = %+v, want zeros", summary)
	}
	mustStatus(t, a.sendJSON(http.MethodGet, "/api/stats/daily?days=7", ""), http.StatusOK)
}

// 공유 갤러리는 로그인 없이도 열리는 공개 API다.
func TestAPISharedDecks(t *testing.T) {
	a := newTestApp(t)
	deck := decodeJSON[store.Deck](t, a.sendJSON(http.MethodPost, "/api/decks", `{"name":"Verbs"}`))
	a.sendJSON(http.MethodPost, "/api/cards", `{"deckSlug":"`+deck.Slug+`","text":"run","meaning":"달리다"}`)

	shared := decodeJSON[store.ShareInfo](t,
		a.sendJSON(http.MethodPost, "/api/decks/"+deck.Slug+"/share", ""))
	if shared.ShareSlug == "" {
		t.Fatal("share slug is empty")
	}

	gallery := decodeJSON[[]store.SharedDeckSummary](t, a.sendJSON(http.MethodGet, "/api/shared-decks", ""))
	if len(gallery) != 1 || gallery[0].CardCount != 1 {
		t.Fatalf("gallery = %+v, want one deck with one card", gallery)
	}

	mustStatus(t, a.sendJSON(http.MethodGet, "/api/shared-decks/"+shared.ShareSlug, ""), http.StatusOK)
	// 슬러그 형식부터 어긋나면 조회 전에 400으로 막는다.
	mustStatus(t, a.sendJSON(http.MethodGet, "/api/shared-decks/!!", ""), http.StatusBadRequest)

	imported := a.sendJSON(http.MethodPost, "/api/shared-decks/"+shared.ShareSlug+"/import", "")
	mustStatus(t, imported, http.StatusCreated)
	if copied := decodeJSON[store.Deck](t, imported); copied.ID == deck.ID {
		t.Error("import returned the original deck instead of a copy")
	}

	mustStatus(t, a.sendJSON(http.MethodDelete, "/api/decks/"+deck.Slug+"/share", ""), http.StatusNoContent)
	mustStatus(t, a.sendJSON(http.MethodGet, "/api/shared-decks/"+shared.ShareSlug, ""), http.StatusNotFound)
}

// 화면과 API는 같은 저장소를 본다. 한쪽으로 넣은 것이 다른 쪽에서 그대로 보여야
// 한다.
func TestWebAndAPIShareOneStore(t *testing.T) {
	a := newTestApp(t)
	slug := a.makeDeck("Verbs")
	a.makeCard(slug, "run", "달리다")

	cards := decodeJSON[[]store.Card](t, a.sendJSON(http.MethodGet, "/api/decks/"+slug+"/cards", ""))
	if len(cards) != 1 || cards[0].Text != "run" {
		t.Fatalf("API sees %+v, want the card the form created", cards)
	}

	a.sendJSON(http.MethodPost, "/api/cards", `{"deckSlug":"`+slug+`","text":"walk","meaning":"걷다"}`)
	mustContain(t, a.get("/decks/"+slug), "걷다")
}
