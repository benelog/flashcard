package study

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/model"
	"github.com/benelog/flashcard/internal/smartrules"
)

// stubStore는 study.Store를 만족하는 스텁이다. 이 패키지가 저장소에서 쓰는
// 부분집합만 인터페이스로 정의해 둔 덕에, model.Store 전체가 아니라 네 메서드만
// 채우면 저장소를 갈아 끼울 수 있다. 각 테스트는 자기가 쓰는 func 필드만 채우고,
// 채우지 않은 메서드를 부르면 nil 함수 호출로 그 자리에서 터지므로 어느 조회를
// 건드리는지가 테스트마다 그대로 드러난다.
type stubStore struct {
	listCards   func(deckID uuid.UUID) ([]model.Card, error)
	dueCards    func(dueBefore time.Time, limit int) ([]model.Card, error)
	cardsByRule func(rule smartrules.Rule) ([]model.Card, error)
	countByRule func(rule smartrules.Rule) (int, error)
}

func (s stubStore) ListCards(_ context.Context, _, deckID uuid.UUID) ([]model.Card, error) {
	return s.listCards(deckID)
}

func (s stubStore) DueCards(_ context.Context, _ uuid.UUID, dueBefore time.Time, limit int) ([]model.Card, error) {
	return s.dueCards(dueBefore, limit)
}

func (s stubStore) CardsByRule(_ context.Context, _ uuid.UUID, rule smartrules.Rule) ([]model.Card, error) {
	return s.cardsByRule(rule)
}

func (s stubStore) CountByRule(_ context.Context, _ uuid.UUID, rule smartrules.Rule) (int, error) {
	return s.countByRule(rule)
}

func cards(texts ...string) []model.Card {
	out := make([]model.Card, 0, len(texts))
	for _, text := range texts {
		out = append(out, model.Card{ID: uuid.New(), Text: text})
	}
	return out
}

func texts(cards []model.Card) []string {
	out := make([]string, 0, len(cards))
	for _, c := range cards {
		out = append(out, c.Text)
	}
	return out
}

// 잘못 만들어진 요청은 저장소를 건드리기 전에 걸러야 한다. 스텁의 어느 메서드도
// 채우지 않았으므로, 조회를 시도했다면 테스트가 패닉으로 실패한다.
func TestPickRejectsBadRequests(t *testing.T) {
	tests := []struct {
		name string
		req  Request
		want error
	}{
		{"모르는 모드", Request{Mode: "nonsense"}, ErrUnknownMode},
		{"모드가 비었음", Request{}, ErrUnknownMode},
		{"덱 모드인데 덱이 없음", Request{Mode: model.ModeDeck}, ErrDeckRequired},
		{"스마트 모드인데 규칙이 없음", Request{Mode: model.ModeSmart}, ErrRuleRequired},
		{"규칙이 빈 문자열", Request{Mode: model.ModeSmart, Rule: json.RawMessage("")}, ErrRuleRequired},
		{"규칙이 깨진 JSON", Request{Mode: model.ModeSmart, Rule: json.RawMessage("{nope")}, ErrInvalidRule},
		{"모르는 규칙 종류", Request{Mode: model.ModeSmart, Rule: json.RawMessage(`{"type":"nope"}`)}, ErrInvalidRule},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Pick(t.Context(), stubStore{}, uuid.New(), tt.req)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Pick() err = %v, want %v", err, tt.want)
			}
		})
	}
}

// 규칙이 왜 잘못됐는지는 감싸인 원인에 남아 있어야 API가 그대로 알려 줄 수 있다.
func TestPickKeepsTheRuleFailureReason(t *testing.T) {
	_, err := Pick(t.Context(), stubStore{}, uuid.New(),
		Request{Mode: model.ModeSmart, Rule: json.RawMessage(`{"type":"tag"}`)})

	if !errors.Is(err, ErrInvalidRule) {
		t.Fatalf("err = %v, want ErrInvalidRule", err)
	}
	if got := err.Error(); !strings.Contains(got, "tag rule requires tags") {
		t.Errorf("err = %q, want it to carry the parse failure", got)
	}
}

func TestPickDeckModeShufflesWithoutLosingCards(t *testing.T) {
	deckID := uuid.New()
	deck := cards("a", "b", "c", "d", "e", "f", "g", "h")

	var asked uuid.UUID
	store := stubStore{listCards: func(id uuid.UUID) ([]model.Card, error) {
		asked = id
		return append([]model.Card(nil), deck...), nil
	}}

	plan, err := Pick(t.Context(), store, uuid.New(), Request{Mode: model.ModeDeck, DeckID: &deckID})
	if err != nil {
		t.Fatal(err)
	}
	if asked != deckID {
		t.Errorf("asked the store for deck %v, want %v", asked, deckID)
	}
	if plan.DeckID == nil || *plan.DeckID != deckID {
		t.Errorf("plan.DeckID = %v, want %v — the session row records which deck", plan.DeckID, deckID)
	}
	if plan.Rule != nil {
		t.Errorf("plan.Rule = %s, want nil for a deck session", plan.Rule)
	}

	seen := map[string]int{}
	for _, c := range plan.Cards {
		seen[c.Text]++
	}
	if len(plan.Cards) != len(deck) || len(seen) != len(deck) {
		t.Fatalf("cards = %v, want the same %d cards exactly once each", texts(plan.Cards), len(deck))
	}
}

func TestPickDueModePassesTheWindowThrough(t *testing.T) {
	cutoff := time.Date(2026, 7, 25, 23, 59, 59, 0, time.UTC)
	var gotCutoff time.Time
	var gotLimit int
	store := stubStore{dueCards: func(dueBefore time.Time, limit int) ([]model.Card, error) {
		gotCutoff, gotLimit = dueBefore, limit
		return cards("run"), nil
	}}

	plan, err := Pick(t.Context(), store, uuid.New(),
		Request{Mode: model.ModeDue, DueBefore: cutoff, Limit: 30})
	if err != nil {
		t.Fatal(err)
	}
	if !gotCutoff.Equal(cutoff) || gotLimit != 30 {
		t.Errorf("store got (%v, %d), want (%v, 30)", gotCutoff, gotLimit, cutoff)
	}
	if plan.DeckID != nil {
		t.Errorf("plan.DeckID = %v, want nil — 오늘 복습은 덱에 매이지 않는다", plan.DeckID)
	}
	if len(plan.Cards) != 1 {
		t.Errorf("cards = %v, want the one due card", texts(plan.Cards))
	}
}

// 규칙은 조회에 쓰기 전에 정규화되고, 세션에 적히는 것도 그 정규화된 규칙이다.
// 나중에 같은 규칙을 다시 실행해도 같은 카드가 나오게 하기 위해서다.
func TestPickSmartModeNormalizesTheRule(t *testing.T) {
	var asked smartrules.Rule
	store := stubStore{cardsByRule: func(rule smartrules.Rule) ([]model.Card, error) {
		asked = rule
		return cards("alpha"), nil
	}}

	plan, err := Pick(t.Context(), store, uuid.New(), Request{
		Mode: model.ModeSmart,
		Rule: json.RawMessage(`{"type":"stale","limit":9999}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if asked.Limit != 20 || asked.NotReviewedDays != 7 {
		t.Errorf("store got %+v, want the clamped limit and the default day window", asked)
	}

	var stored smartrules.Rule
	if err := json.Unmarshal(plan.Rule, &stored); err != nil {
		t.Fatalf("plan.Rule is not valid json: %v", err)
	}
	if stored.Limit != 20 {
		t.Errorf("plan.Rule = %s, want the normalized rule the store ran", plan.Rule)
	}
}

// 저장소가 넘어지면 그대로 올려 보낸다. 잘못된 요청을 뜻하는 sentinel과 섞이면
// 부르는 쪽이 500을 400으로 잘못 알린다.
func TestPickPropagatesStoreFailure(t *testing.T) {
	boom := errors.New("connection reset")
	store := stubStore{dueCards: func(time.Time, int) ([]model.Card, error) { return nil, boom }}

	plan, err := Pick(t.Context(), store, uuid.New(), Request{Mode: model.ModeDue, Limit: 10})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the store's error", err)
	}
	for _, sentinel := range []error{ErrUnknownMode, ErrDeckRequired, ErrRuleRequired, ErrInvalidRule} {
		if errors.Is(err, sentinel) {
			t.Errorf("store failure also matches %v, so callers would answer 400", sentinel)
		}
	}
	if plan.Cards != nil {
		t.Errorf("plan.Cards = %v, want nothing to study on failure", texts(plan.Cards))
	}
}

// "오늘 복습"의 만기 상한은 방문자 시간대의 그날 마지막 순간이다. 시각을 주입
// 받으므로 자정 경계와 시간대 변환을 시계 없이 검증한다.
func TestEndOfDay(t *testing.T) {
	seoul, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		t.Fatal(err)
	}

	// UTC 7/24 16:00 = 서울 7/25 01:00. 서울 방문자의 "오늘"은 이미 25일이다.
	now := time.Date(2026, 7, 24, 16, 0, 0, 0, time.UTC)
	got := EndOfDay(now, seoul)
	want := time.Date(2026, 7, 25, 23, 59, 59, 0, seoul)
	if !got.Equal(want) {
		t.Errorf("EndOfDay(%v, Seoul) = %v, want %v", now, got, want)
	}

	// 같은 순간이라도 UTC 방문자의 하루는 아직 24일이다.
	got = EndOfDay(now, time.UTC)
	want = time.Date(2026, 7, 24, 23, 59, 59, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("EndOfDay(%v, UTC) = %v, want %v", now, got, want)
	}
}

// 하루 학습량은 화면과 JSON API가 같은 프로필 값을 읽어야 한다. 설정 JSON이
// 없거나 범위를 벗어나면 기본값으로 돌아간다.
func TestGoalFromProfile(t *testing.T) {
	tests := []struct {
		name     string
		settings string
		want     int
	}{
		{"저장한 적 없음", "", DefaultDailyGoal},
		{"빈 객체", `{}`, DefaultDailyGoal},
		{"정상 값", `{"dailyGoal":30}`, 30},
		{"0은 기본값으로", `{"dailyGoal":0}`, DefaultDailyGoal},
		{"상한 초과도 기본값으로", `{"dailyGoal":999}`, DefaultDailyGoal},
		{"깨진 JSON", `{nope`, DefaultDailyGoal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GoalFromProfile(model.Profile{Settings: []byte(tt.settings)})
			if got != tt.want {
				t.Errorf("GoalFromProfile(%q) = %d, want %d", tt.settings, got, tt.want)
			}
		})
	}
}

func TestSuggestionsKeepOnlyRulesWithCards(t *testing.T) {
	suggested := smartrules.Suggested()
	if len(suggested) < 2 {
		t.Skipf("이 테스트는 추천 규칙이 둘 이상일 때를 가정한다 (지금 %d개)", len(suggested))
	}

	// 첫 규칙만 카드가 있다.
	store := stubStore{countByRule: func(rule smartrules.Rule) (int, error) {
		if rule.Type == suggested[0].Type {
			return 7, nil
		}
		return 0, nil
	}}

	found, err := Suggestions(t.Context(), store, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 {
		t.Fatalf("Suggestions() = %d tiles, want only the rule that has cards", len(found))
	}
	if found[0].Rule.Type != suggested[0].Type || found[0].Count != 7 {
		t.Errorf("tile = %+v, want %v with 7 cards", found[0], suggested[0].Type)
	}
}

func TestSuggestionsAreEmptyForAFreshAccount(t *testing.T) {
	store := stubStore{countByRule: func(smartrules.Rule) (int, error) { return 0, nil }}

	found, err := Suggestions(t.Context(), store, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if found == nil {
		t.Error("Suggestions() = nil, want an empty slice so the API renders []")
	}
	if len(found) != 0 {
		t.Errorf("Suggestions() = %d tiles, want none", len(found))
	}
}

func TestSuggestionsPropagateStoreFailure(t *testing.T) {
	boom := errors.New("connection reset")
	store := stubStore{countByRule: func(smartrules.Rule) (int, error) { return 0, boom }}

	if _, err := Suggestions(t.Context(), store, uuid.New()); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the store's error", err)
	}
}
