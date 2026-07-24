package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/model"
)

// 여기 있는 테스트는 "DB가 살아 있지만 질의가 실패할 때" 앱이 어떻게 답하는지를
// 본다. 나머지 테스트는 전부 정상 경로라 이 갈래는 실제로 실행된 적이 없었다.
//
// 두 가지가 중요하다. 첫째, 화면은 HTML 오류 페이지로, /api/*는 JSON으로 답해야
// 한다. 브라우저에 JSON이 뜨거나 API 호출자가 HTML을 받으면 곤란하다. 둘째,
// 어느 쪽도 DB가 뱉은 원문을 방문자에게 그대로 보여 주지 않아야 한다.

var errStoreDown = errors.New("connection reset by peer: table cards is on fire")

// brokenStore는 진짜 저장소에서 고른 메서드만 깨뜨린 것이다. model.Store를
// 묻어 두었으므로 지정하지 않은 메서드 스물몇 개는 그대로 진짜 저장소로 간다.
type brokenStore struct {
	model.Store
	listDecks  bool
	dueCount   bool
	listCards  bool
	getProfile bool
}

func (b brokenStore) GetOrCreateProfile(ctx context.Context, userID uuid.UUID, displayName string) (model.Profile, error) {
	if b.getProfile {
		return model.Profile{}, errStoreDown
	}
	return b.Store.GetOrCreateProfile(ctx, userID, displayName)
}

func (b brokenStore) ListDecks(ctx context.Context, userID uuid.UUID) ([]model.Deck, error) {
	if b.listDecks {
		return nil, errStoreDown
	}
	return b.Store.ListDecks(ctx, userID)
}

func (b brokenStore) DueCount(ctx context.Context, userID uuid.UUID, dueBefore time.Time) (int, error) {
	if b.dueCount {
		return 0, errStoreDown
	}
	return b.Store.DueCount(ctx, userID, dueBefore)
}

func (b brokenStore) ListCards(ctx context.Context, userID, deckID uuid.UUID) ([]model.Card, error) {
	if b.listCards {
		return nil, errStoreDown
	}
	return b.Store.ListCards(ctx, userID, deckID)
}

func TestPagesShowAnErrorScreenWhenTheStoreFails(t *testing.T) {
	tests := []struct {
		name   string
		broken brokenStore
		path   string
	}{
		{"덱 목록", brokenStore{listDecks: true}, "/decks"},
		{"홈 화면", brokenStore{dueCount: true}, "/"},
		// 프로필 보장(auth.EnsureProfile)은 화면과 API가 공유하는 미들웨어라,
		// 화면 쪽 실패가 JSON으로 새지 않는지 따로 본다.
		{"프로필 보장", brokenStore{getProfile: true}, "/decks"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newAppWith(t, func(s model.Store) model.Store {
				tt.broken.Store = s
				return tt.broken
			})

			rec := a.get(tt.path)

			mustStatus(t, rec, http.StatusInternalServerError)
			if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
				t.Errorf("Content-Type = %q, want HTML for a page request", got)
			}
			mustContain(t, rec, "일시적인 오류가 발생했어요")
			mustNotContain(t, rec, "table cards is on fire")
		})
	}
}

func TestAPIReturnsJSONWhenTheStoreFails(t *testing.T) {
	tests := []struct {
		name   string
		broken brokenStore
	}{
		{"덱 목록", brokenStore{listDecks: true}},
		{"프로필 보장", brokenStore{getProfile: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newAppWith(t, func(s model.Store) model.Store {
				tt.broken.Store = s
				return tt.broken
			})

			rec := a.sendJSON(http.MethodGet, "/api/decks", "")

			mustStatus(t, rec, http.StatusInternalServerError)
			mustContain(t, rec, `"error"`)
			mustNotContain(t, rec, "<html")
			mustNotContain(t, rec, "table cards is on fire")
		})
	}
}

// 학습은 저장소 실패와 "잘못된 학습 링크"를 구별해야 한다. 후자는 404 화면이고
// (TestStudyRejectsBadModeAndRule), 전자는 500이다. internal/study가 요청 오류를
// sentinel로만 돌려주기 때문에 이 구분이 성립한다.
func TestStudyPageIs500WhenTheStoreFails(t *testing.T) {
	a := newAppWith(t, func(s model.Store) model.Store {
		return brokenStore{Store: s, listCards: true}
	})
	slug := a.makeDeck("Verbs")
	deck := a.deck(slug)

	rec := a.get("/study?direction=text_to_meaning&mode=deck&deckId=" + deck.ID.String())

	mustStatus(t, rec, http.StatusInternalServerError)
	mustContain(t, rec, "일시적인 오류가 발생했어요")
}

// 세션 생성도 마찬가지다. 요청이 잘못됐으면 400, 저장소가 넘어졌으면 500이라야
// 호출자가 재시도할지 요청을 고칠지 판단할 수 있다.
func TestAPISessionIs500WhenTheStoreFails(t *testing.T) {
	a := newAppWith(t, func(s model.Store) model.Store {
		return brokenStore{Store: s, listCards: true}
	})
	deck := decodeJSON[model.Deck](t, a.sendJSON(http.MethodPost, "/api/decks", `{"name":"Verbs"}`))

	rec := a.sendJSON(http.MethodPost, "/api/sessions",
		`{"mode":"deck","deckId":"`+deck.ID.String()+`"}`)

	mustStatus(t, rec, http.StatusInternalServerError)
	mustContain(t, rec, `"error"`)
}
