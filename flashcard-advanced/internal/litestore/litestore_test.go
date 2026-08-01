package litestore

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/model"
	"github.com/benelog/flashcard/internal/smartrules"
)

// tag::fixture[]
func testStore(t *testing.T) (*Store, uuid.UUID) {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	userID := uuid.New()
	if _, err := s.GetOrCreateProfile(context.Background(), userID, ""); err != nil {
		t.Fatal(err)
	}
	return s, userID
}

// end::fixture[]

func mustDeck(t *testing.T, s *Store, userID uuid.UUID, name string) model.Deck {
	t.Helper()
	deck, err := s.CreateDeck(context.Background(), userID, name, nil)
	if err != nil {
		t.Fatal(err)
	}
	return deck
}

func mustCard(t *testing.T, s *Store, userID, deckID uuid.UUID, text string, tags []string) model.Card {
	t.Helper()
	card, err := s.CreateCard(context.Background(), userID, model.CardInput{
		DeckID: deckID, Text: text, Meaning: "meaning of " + text, CardType: "word", Tags: tags,
	})
	if err != nil {
		t.Fatal(err)
	}
	return card
}

// Open은 멱등이어야 한다: 이미 있는 파일을 다시 열어도 스키마 재적용이 오류
// 없이 지나간다.
func TestOpenIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	for i := 0; i < 2; i++ {
		s, err := Open(path)
		if err != nil {
			t.Fatalf("open #%d: %v", i+1, err)
		}
		s.Close()
	}
}

func TestDeckCRUDAndSlug(t *testing.T) {
	s, userID := testStore(t)
	ctx := context.Background()

	desc := "irregular verbs"
	deck, err := s.CreateDeck(ctx, userID, "Verbs", &desc)
	if err != nil {
		t.Fatal(err)
	}
	if len(deck.Slug) != 4 {
		t.Fatalf("deck.Slug = %q, want 4 Base36 chars", deck.Slug)
	}
	if deck.Description == nil || *deck.Description != desc {
		t.Fatalf("deck.Description = %v, want %q", deck.Description, desc)
	}

	bySlug, err := s.GetDeckBySlug(ctx, userID, deck.Slug)
	if err != nil || bySlug.ID != deck.ID {
		t.Fatalf("GetDeckBySlug(%q) = %v, %v; want deck %v", deck.Slug, bySlug.ID, err, deck.ID)
	}
	id, err := s.DeckIDBySlug(ctx, userID, deck.Slug)
	if err != nil || id != deck.ID {
		t.Fatalf("DeckIDBySlug(%q) = %v, %v; want %v", deck.Slug, id, err, deck.ID)
	}
	if _, err := s.DeckIDBySlug(ctx, userID, "zzzz"); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("DeckIDBySlug(unknown) err = %v, want ErrNotFound", err)
	}
	if _, err := s.DeckIDBySlug(ctx, userID, "!!"); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("DeckIDBySlug(malformed) err = %v, want ErrNotFound", err)
	}

	name := "Irregular verbs"
	updated, err := s.UpdateDeck(ctx, userID, deck.ID, &name, nil)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != name || updated.Description == nil || *updated.Description != desc {
		t.Fatalf("UpdateDeck = %+v, want renamed with description kept", updated)
	}

	decks, err := s.ListDecks(ctx, userID)
	if err != nil || len(decks) != 1 {
		t.Fatalf("ListDecks = %d decks, %v; want 1", len(decks), err)
	}

	if err := s.DeleteDeck(ctx, userID, deck.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetDeck(ctx, userID, deck.ID); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("GetDeck after delete err = %v, want ErrNotFound", err)
	}
	if err := s.DeleteDeck(ctx, userID, deck.ID); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("DeleteDeck twice err = %v, want ErrNotFound", err)
	}
}

func TestCardCRUD(t *testing.T) {
	s, userID := testStore(t)
	ctx := context.Background()
	deck := mustDeck(t, s, userID, "Words")

	card := mustCard(t, s, userID, deck.ID, "run", []string{"verb"})
	if card.DeckSlug != deck.Slug {
		t.Fatalf("card.DeckSlug = %q, want %q", card.DeckSlug, deck.Slug)
	}
	if card.Attempts != 0 || card.ErrorRate != 0 || card.LastReviewed != nil {
		t.Fatalf("new card SRS summary = %+v, want zero state", card)
	}
	if card.DueAt.IsZero() || card.DueAt.After(time.Now()) {
		t.Fatalf("card.DueAt = %v, want due immediately", card.DueAt)
	}

	in := model.CardInput{
		DeckID: deck.ID, Text: "run", Meaning: "달리다",
		CardType: "sentence", Tags: []string{"verb", "motion"},
	}
	updated, err := s.UpdateCard(ctx, userID, card.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Meaning != "달리다" || updated.CardType != "sentence" || len(updated.Tags) != 2 {
		t.Fatalf("UpdateCard = %+v", updated)
	}

	cards, err := s.ListCards(ctx, userID, deck.ID)
	if err != nil || len(cards) != 1 {
		t.Fatalf("ListCards = %d cards, %v; want 1", len(cards), err)
	}

	if err := s.DeleteCard(ctx, userID, card.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetCard(ctx, userID, card.ID); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("GetCard after delete err = %v, want ErrNotFound", err)
	}
}

func TestBulkCreateCardsSkipsDuplicates(t *testing.T) {
	s, userID := testStore(t)
	ctx := context.Background()
	deck := mustDeck(t, s, userID, "Bulk")
	mustCard(t, s, userID, deck.ID, "run", nil)

	inputs := []model.CardInput{
		{Text: "run", Meaning: "m", CardType: "word"},   // 덱에 이미 있다
		{Text: " RUN ", Meaning: "m", CardType: "word"}, // 대소문자·공백만 다른 중복
		{Text: "walk", Meaning: "m", CardType: "word"},  // 새 카드
		{Text: "walk", Meaning: "m", CardType: "word"},  // 배치 안에서 반복
		{Text: "jump", Meaning: "m", CardType: "word"},  // 새 카드
	}
	res, err := s.BulkCreateCards(ctx, userID, deck.ID, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 2 || res.Skipped != 3 {
		t.Fatalf("BulkCreateCards = %+v, want added 2 skipped 3", res)
	}
	cards, err := s.ListCards(ctx, userID, deck.ID)
	if err != nil || len(cards) != 3 {
		t.Fatalf("ListCards = %d cards, %v; want 3", len(cards), err)
	}
}

func TestReviewSessionFlow(t *testing.T) {
	s, userID := testStore(t)
	ctx := context.Background()
	deck := mustDeck(t, s, userID, "Study")
	card := mustCard(t, s, userID, deck.ID, "run", nil)

	if n, err := s.DueCount(ctx, userID, time.Now()); err != nil || n != 1 {
		t.Fatalf("DueCount before review = %d, %v; want 1", n, err)
	}

	sess, err := s.CreateSession(ctx, userID, "deck", "text_to_meaning", &deck.ID, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Mode != "deck" || sess.TotalCards != 1 || sess.StartedAt.IsZero() {
		t.Fatalf("CreateSession = %+v", sess)
	}

	// 첫 응답 정답: repetitions 1 -> 간격 1일.
	out, err := s.RecordReview(ctx, userID, sess.ID, card.ID, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if out.IntervalDays != 1 {
		t.Fatalf("first review IntervalDays = %v, want 1", out.IntervalDays)
	}
	if d := time.Until(out.DueAt); d < 23*time.Hour || d > 25*time.Hour {
		t.Fatalf("first review DueAt = %v, want ~1 day out", out.DueAt)
	}

	// 재시도 라운드의 응답은 기록만 되고 채점되지 않는다.
	if out, err = s.RecordReview(ctx, userID, sess.ID, card.ID, false, true); err != nil {
		t.Fatal(err)
	}
	if out.IntervalDays != 1 {
		t.Fatalf("retry IntervalDays = %v, want unchanged 1", out.IntervalDays)
	}

	got, err := s.GetCard(ctx, userID, card.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Attempts != 1 || got.ErrorRate != 0 || got.IntervalDays != 1 || got.LastReviewed == nil {
		t.Fatalf("card after review = attempts %d, errorRate %v, interval %v", got.Attempts, got.ErrorRate, got.IntervalDays)
	}

	var logged int
	if err := s.db.QueryRow(`select count(*) from review_logs`).Scan(&logged); err != nil {
		t.Fatal(err)
	}
	if logged != 2 {
		t.Fatalf("review_logs rows = %d, want 2 (grade + retry)", logged)
	}

	// 카드가 하루 뒤로 밀렸다: 지금은 만기가 아니고 내일 만기다.
	if n, _ := s.DueCount(ctx, userID, time.Now()); n != 0 {
		t.Fatalf("DueCount after review = %d, want 0", n)
	}
	if n, _ := s.DueCount(ctx, userID, time.Now().Add(25*time.Hour)); n != 1 {
		t.Fatalf("DueCount tomorrow = %d, want 1", n)
	}
	if cards, _ := s.DueCards(ctx, userID, time.Now().Add(25*time.Hour), 10); len(cards) != 1 {
		t.Fatalf("DueCards tomorrow = %d cards, want 1", len(cards))
	}

	// 첫 응답 오답은 간격을 초기화하고 lapse를 하나 센다.
	if out, err = s.RecordReview(ctx, userID, sess.ID, card.ID, false, false); err != nil {
		t.Fatal(err)
	}
	if out.IntervalDays != 1 {
		t.Fatalf("failed review IntervalDays = %v, want reset to 1", out.IntervalDays)
	}
	got, _ = s.GetCard(ctx, userID, card.ID)
	if got.Attempts != 2 || got.ErrorRate != 0.5 {
		t.Fatalf("card after lapse = attempts %d, errorRate %v; want 2, 0.5", got.Attempts, got.ErrorRate)
	}

	if _, err := s.RecordReview(ctx, uuid.New(), sess.ID, card.ID, true, false); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("RecordReview as stranger err = %v, want ErrNotFound", err)
	}

	if err := s.FinishSession(ctx, userID, sess.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishSession(ctx, userID, uuid.New(), true); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("FinishSession unknown err = %v, want ErrNotFound", err)
	}
}

func TestSmartRules(t *testing.T) {
	s, userID := testStore(t)
	ctx := context.Background()
	deck := mustDeck(t, s, userID, "Smart")

	alpha := mustCard(t, s, userID, deck.ID, "alpha", []string{"verb"})
	beta := mustCard(t, s, userID, deck.ID, "beta", []string{"noun"})
	gamma := mustCard(t, s, userID, deck.ID, "gamma", nil)

	// 결정적 픽스처: alpha는 자주 틀리고 2일 전에 추가됐다. beta는 10일 전 리뷰
	// 뒤 방치됐고, gamma는 방금 리뷰했지만 한 달 전에 추가됐다.
	now := time.Now()
	fix := []struct {
		q    string
		args []any
	}{
		{`update card_srs set correct_count = 1, incorrect_count = 2 where card_id = ?`,
			[]any{alpha.ID.String()}},
		{`update cards set created_at = ? where id = ?`,
			[]any{fmtTime(now.AddDate(0, 0, -2)), alpha.ID.String()}},
		{`update cards set created_at = ? where id = ?`,
			[]any{fmtTime(now.AddDate(0, 0, -1)), beta.ID.String()}},
		{`update card_srs set last_reviewed_at = ? where card_id = ?`,
			[]any{fmtTime(now.AddDate(0, 0, -10)), beta.ID.String()}},
		{`update cards set created_at = ? where id = ?`,
			[]any{fmtTime(now.AddDate(0, 0, -30)), gamma.ID.String()}},
		{`update card_srs set last_reviewed_at = ? where card_id = ?`,
			[]any{fmtTime(now), gamma.ID.String()}},
	}
	for _, f := range fix {
		if _, err := s.db.Exec(f.q, f.args...); err != nil {
			t.Fatal(err)
		}
	}

	// tag::subtest-table[]
	tests := []struct {
		name string
		rule smartrules.Rule
		want []string // 규칙 순서대로 기대하는 카드 text
	}{
		{"high_error",
			smartrules.Rule{Type: smartrules.HighError, MinAttempts: 3, MinErrorRate: 0.4, Limit: 20},
			[]string{"alpha"}},
		{"stale nulls first",
			smartrules.Rule{Type: smartrules.Stale, NotReviewedDays: 7, Limit: 20},
			[]string{"alpha", "beta"}},
		// end::subtest-table[]
		{"tag overlap",
			smartrules.Rule{Type: smartrules.Tag, Tags: []string{"verb", "adj"}, Limit: 20},
			[]string{"alpha"}},
		{"recent newest first",
			smartrules.Rule{Type: smartrules.Recent, AddedWithinDays: 7, Limit: 20},
			[]string{"beta", "alpha"}},
	}
	// tag::subtest-run[]
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cards, err := s.CardsByRule(ctx, userID, tt.rule)
			// end::subtest-run[]
			if err != nil {
				t.Fatal(err)
			}
			texts := make([]string, len(cards))
			for i, c := range cards {
				texts[i] = c.Text
			}
			if len(texts) != len(tt.want) {
				t.Fatalf("CardsByRule = %v, want %v", texts, tt.want)
			}
			for i := range texts {
				if texts[i] != tt.want[i] {
					t.Fatalf("CardsByRule = %v, want %v", texts, tt.want)
				}
			}
			if n, err := s.CountByRule(ctx, userID, tt.rule); err != nil || n != len(tt.want) {
				t.Fatalf("CountByRule = %d, %v; want %d", n, err, len(tt.want))
			}
		})
	}
}

func TestSharedDeckFlow(t *testing.T) {
	s, owner := testStore(t)
	ctx := context.Background()
	viewer := uuid.New()
	if _, err := s.GetOrCreateProfile(ctx, viewer, "viewer"); err != nil {
		t.Fatal(err)
	}

	deck := mustDeck(t, s, owner, "Shared")
	mustCard(t, s, owner, deck.ID, "hello", []string{"greeting"})
	mustCard(t, s, owner, deck.ID, "bye", nil)

	info, err := s.ShareDeck(ctx, owner, deck.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.ShareSlug) != 5 {
		t.Fatalf("ShareSlug = %q, want 5 chars", info.ShareSlug)
	}
	again, err := s.ShareDeck(ctx, owner, deck.ID)
	if err != nil || again.ShareSlug != info.ShareSlug {
		t.Fatalf("ShareDeck twice = %q, %v; want stable slug %q", again.ShareSlug, err, info.ShareSlug)
	}

	list, err := s.ListSharedDecks(ctx, viewer)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListSharedDecks = %d decks, %v; want 1", len(list), err)
	}
	if list[0].IsMine || list[0].CardCount != 2 {
		t.Fatalf("viewer sees %+v, want isMine false, 2 cards", list[0])
	}
	mine, err := s.ListSharedDecks(ctx, owner)
	if err != nil || len(mine) != 1 || !mine[0].IsMine {
		t.Fatalf("owner sees %+v, %v; want isMine true", mine, err)
	}

	if _, err := s.GetSharedDeck(ctx, viewer, info.ShareSlug); err != nil {
		t.Fatal(err)
	}
	cards, err := s.GetSharedDeckCards(ctx, info.ShareSlug)
	if err != nil || len(cards) != 2 {
		t.Fatalf("GetSharedDeckCards = %d cards, %v; want 2", len(cards), err)
	}

	imported, err := s.ImportSharedDeck(ctx, viewer, info.ShareSlug)
	if err != nil {
		t.Fatal(err)
	}
	if imported.ID == deck.ID || imported.CardCount != 2 || imported.Name != deck.Name {
		t.Fatalf("ImportSharedDeck = %+v", imported)
	}
	copies, err := s.ListCards(ctx, viewer, imported.ID)
	if err != nil || len(copies) != 2 {
		t.Fatalf("imported cards = %d, %v; want 2", len(copies), err)
	}
	for _, c := range copies {
		if c.Attempts != 0 || c.LastReviewed != nil {
			t.Fatalf("imported card %q kept SRS state: %+v", c.Text, c)
		}
	}

	if err := s.UnshareDeck(ctx, owner, deck.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetSharedDeck(ctx, viewer, info.ShareSlug); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("GetSharedDeck after unshare err = %v, want ErrNotFound", err)
	}
	if _, err := s.ImportSharedDeck(ctx, viewer, info.ShareSlug); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("ImportSharedDeck after unshare err = %v, want ErrNotFound", err)
	}
}

func TestStats(t *testing.T) {
	s, userID := testStore(t)
	ctx := context.Background()
	deck := mustDeck(t, s, userID, "Stats")
	a := mustCard(t, s, userID, deck.ID, "a", nil)
	b := mustCard(t, s, userID, deck.ID, "b", nil)

	sess, err := s.CreateSession(ctx, userID, "deck", "text_to_meaning", &deck.ID, nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []struct {
		card    uuid.UUID
		result  bool
		isRetry bool
	}{
		{a.ID, true, false},
		{b.ID, false, false},
		{b.ID, true, true}, // 재시도: 기록만 되고 채점되지 않는다
	} {
		if _, err := s.RecordReview(ctx, userID, sess.ID, r.card, r.result, r.isRetry); err != nil {
			t.Fatal(err)
		}
	}
	// 어제의 리뷰가 스트릭을 2로 늘린다.
	if _, err := s.db.Exec(
		`insert into review_logs (user_id, card_id, session_id, result, is_retry, reviewed_at)
		 values (?, ?, ?, 1, 0, ?)`,
		userID.String(), a.ID.String(), sess.ID.String(),
		fmtTime(time.Now().AddDate(0, 0, -1))); err != nil {
		t.Fatal(err)
	}
	// 성숙 카드 하나(간격 >= 21일).
	if _, err := s.db.Exec(
		`update card_srs set interval_days = 30 where card_id = ?`, a.ID.String()); err != nil {
		t.Fatal(err)
	}

	daily, err := s.DailyStats(ctx, userID, "UTC", 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(daily) != 2 {
		t.Fatalf("DailyStats = %d days, want 2", len(daily))
	}
	today := daily[len(daily)-1]
	if today.Date != time.Now().UTC().Format("2006-01-02") || today.Total != 3 || today.Correct != 2 {
		t.Fatalf("today = %+v, want total 3 (retries included), correct 2", today)
	}

	sum, err := s.StatsSummary(ctx, userID, "UTC", time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if sum.TotalReviews != 3 || sum.CorrectReviews != 2 {
		t.Fatalf("summary totals = %d/%d, want 3 first-pass with 2 correct", sum.CorrectReviews, sum.TotalReviews)
	}
	if sum.Streak != 2 {
		t.Fatalf("Streak = %d, want 2", sum.Streak)
	}
	if len(sum.Decks) != 1 || sum.Decks[0].TotalCards != 2 || sum.Decks[0].MatureCards != 1 {
		t.Fatalf("deck mastery = %+v, want 2 cards with 1 mature", sum.Decks)
	}
}

// timeLayout은 모든 값의 폭이 같아 "사전순 = 시간순"이라는 전제 위에 서 있다
// (litestore.go 머리 주석). 이 전제가 깨지면 due_at <= ? 비교가 조용히 틀리므로
// 표로 고정해 둔다. 왕복(저장 후 되읽기)도 함께 확인한다.
func TestFmtTimeOrderMatchesTimeOrder(t *testing.T) {
	moments := []time.Time{
		time.Date(1999, 12, 31, 23, 59, 59, 999000000, time.UTC),
		time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 25, 12, 30, 45, 123000000, time.UTC),
		time.Date(2026, 7, 25, 4, 5, 6, 0, time.FixedZone("KST", 9*3600)), // UTC로는 7/24
	}
	for _, a := range moments {
		for _, b := range moments {
			if (fmtTime(a) < fmtTime(b)) != a.Before(b) {
				t.Errorf("string order of %v vs %v disagrees with time order", a, b)
			}
		}
	}
	for _, m := range moments {
		back, err := parseTime(fmtTime(m))
		if err != nil {
			t.Fatal(err)
		}
		if !back.Equal(m) {
			t.Errorf("round trip %v -> %v", m, back)
		}
	}
}

// tags 열은 항상 JSON 배열이어야 한다. nil이 "null"로 저장되면 scanCard의
// Unmarshal이 nil 슬라이스를 만들어 API 응답의 tags가 []가 아니라 null이 된다.
func TestTagsJSON(t *testing.T) {
	if got := tagsJSON(nil); got != "[]" {
		t.Errorf("tagsJSON(nil) = %q, want []", got)
	}
	if got := tagsJSON([]string{"a", "b"}); got != `["a","b"]` {
		t.Errorf(`tagsJSON([a b]) = %q, want ["a","b"]`, got)
	}
}
