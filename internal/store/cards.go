package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/benelog/flashcard/internal/smartrules"
)

type Card struct {
	ID        uuid.UUID `json:"id"`
	DeckID    uuid.UUID `json:"deckId"`
	DeckSlug  string    `json:"deckSlug"` // populated by GetCard for the edit page's deck link
	Text      string    `json:"text"`
	Meaning   string    `json:"meaning"`
	CardType  string    `json:"cardType"`
	Tags      []string  `json:"tags"`
	Phonetic  *string   `json:"phonetic"`
	Example   *string   `json:"example"`
	Notes     *string   `json:"notes"`
	CreatedAt time.Time `json:"createdAt"`

	// SRS summary from cards_with_stats.
	Attempts     int        `json:"attempts"`
	ErrorRate    float64    `json:"errorRate"`
	IntervalDays float64    `json:"intervalDays"`
	DueAt        time.Time  `json:"dueAt"`
	LastReviewed *time.Time `json:"lastReviewedAt"`
}

type CardInput struct {
	DeckID   uuid.UUID `json:"deckId"`
	Text     string    `json:"text"`
	Meaning  string    `json:"meaning"`
	CardType string    `json:"cardType"`
	Tags     []string  `json:"tags"`
	Phonetic *string   `json:"phonetic"`
	Example  *string   `json:"example"`
	Notes    *string   `json:"notes"`
}

// 카드 종류. DB의 cards.card_type 열이 받는 값과 같다. 카드가 들어오는 길은
// JSON API, 웹 폼, CSV 가져오기 셋인데, 어느 길로 들어오든 여기 있는 값 하나로
// 정해지도록 판정을 이 파일에 모았다.
const (
	CardTypeWord     = "word"
	CardTypeSentence = "sentence"
	CardTypeIdiom    = "idiom"
	CardTypeConcept  = "concept"

	// DefaultCardType은 종류를 고르지 않았을 때의 값이다.
	DefaultCardType = CardTypeWord
)

// CardTypes lists every accepted card type, in the order the UI offers them.
var CardTypes = []string{CardTypeWord, CardTypeSentence, CardTypeIdiom, CardTypeConcept}

// IsCardType reports whether t is one of the accepted types. JSON API 요청처럼
// 잘못된 값을 400으로 되돌려 줘야 하는 곳에서 쓴다.
func IsCardType(t string) bool {
	for _, valid := range CardTypes {
		if t == valid {
			return true
		}
	}
	return false
}

// NormalizeCardType maps blank or unrecognized input to DefaultCardType. 폼과
// CSV처럼 사람이 손으로 채우는 입력에서 쓴다. 오타 하나로 카드 등록 전체를
// 실패시키는 것보다 기본 종류로 받아 두는 편이 낫기 때문이다.
func NormalizeCardType(t string) string {
	if IsCardType(t) {
		return t
	}
	return DefaultCardType
}

const cardSelect = `
	select id, deck_id, text, meaning, card_type, tags, phonetic, example,
	       notes, created_at, attempts, error_rate, interval_days, due_at, last_reviewed_at
	from cards_with_stats`

func scanCard(row pgx.Row) (Card, error) {
	var c Card
	err := row.Scan(&c.ID, &c.DeckID, &c.Text, &c.Meaning, &c.CardType, &c.Tags,
		&c.Phonetic, &c.Example, &c.Notes, &c.CreatedAt,
		&c.Attempts, &c.ErrorRate, &c.IntervalDays, &c.DueAt, &c.LastReviewed)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

func (s *Store) collectCards(rows pgx.Rows) ([]Card, error) {
	defer rows.Close()
	cards := []Card{}
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

func (s *Store) ListCards(ctx context.Context, userID, deckID uuid.UUID) ([]Card, error) {
	rows, err := s.pool.Query(ctx,
		cardSelect+` where user_id = $1 and deck_id = $2 order by created_at desc`, userID, deckID)
	if err != nil {
		return nil, err
	}
	return s.collectCards(rows)
}

func (s *Store) GetCard(ctx context.Context, userID, cardID uuid.UUID) (Card, error) {
	c, err := scanCard(s.pool.QueryRow(ctx, cardSelect+` where user_id = $1 and id = $2`, userID, cardID))
	if err != nil {
		return c, err
	}
	// The edit page reaches a card by /cards/{id} without a deck in the URL, so
	// hand it the deck slug for the back link.
	var seq int64
	if err := s.pool.QueryRow(ctx, `select seq from decks where id = $1`, c.DeckID).Scan(&seq); err != nil {
		return c, err
	}
	c.DeckSlug = EncodeDeckSlug(seq)
	return c, nil
}

// CreateCard inserts the card and its SRS row in one transaction; the deck
// ownership check doubles as the foreign-key guard.
func (s *Store) CreateCard(ctx context.Context, userID uuid.UUID, in CardInput) (Card, error) {
	if _, err := s.GetDeck(ctx, userID, in.DeckID); err != nil {
		return Card{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Card{}, err
	}
	defer tx.Rollback(ctx)

	var cardID uuid.UUID
	err = tx.QueryRow(ctx,
		`insert into cards (user_id, deck_id, text, meaning, card_type, tags, phonetic, example, notes)
		 values ($1, $2, $3, $4, $5, $6, $7, $8, $9) returning id`,
		userID, in.DeckID, in.Text, in.Meaning, in.CardType, in.Tags, in.Phonetic, in.Example, in.Notes).
		Scan(&cardID)
	if err != nil {
		return Card{}, err
	}
	if _, err := tx.Exec(ctx,
		`insert into card_srs (card_id, user_id) values ($1, $2)`, cardID, userID); err != nil {
		return Card{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Card{}, err
	}
	return s.GetCard(ctx, userID, cardID)
}

func (s *Store) UpdateCard(ctx context.Context, userID, cardID uuid.UUID, in CardInput) (Card, error) {
	err := requireRowAffected(s.pool.Exec(ctx,
		`update cards set
		   text = $3, meaning = $4, card_type = $5, tags = $6,
		   phonetic = $7, example = $8, notes = $9, updated_at = now()
		 where user_id = $1 and id = $2`,
		userID, cardID, in.Text, in.Meaning, in.CardType, in.Tags, in.Phonetic, in.Example, in.Notes))
	if err != nil {
		return Card{}, err
	}
	return s.GetCard(ctx, userID, cardID)
}

func (s *Store) DeleteCard(ctx context.Context, userID, cardID uuid.UUID) error {
	return requireRowAffected(s.pool.Exec(ctx,
		`delete from cards where user_id = $1 and id = $2`, userID, cardID))
}

type BulkResult struct {
	Added   int `json:"added"`
	Skipped int `json:"skipped"`
}

// MaxBulkCards bounds one bulk insert. 한 번의 요청이 통째로 한 트랜잭션이라,
// 무제한으로 받으면 서버리스 함수의 실행 시간 한도에 먼저 걸린다. CSV 업로드와
// JSON API의 대량 등록이 같은 한도를 쓴다.
const MaxBulkCards = 2000

// BulkCreateCards inserts many cards, skipping texts that already exist
// in the deck (or repeat within the batch), compared case- and space-insensitively.
func (s *Store) BulkCreateCards(ctx context.Context, userID, deckID uuid.UUID, inputs []CardInput) (BulkResult, error) {
	var res BulkResult
	if _, err := s.GetDeck(ctx, userID, deckID); err != nil {
		return res, err
	}
	// tag::seen-head[]
	seen := map[string]bool{}
	// end::seen-head[]
	rows, err := s.pool.Query(ctx,
		`select lower(trim(text)) from cards where user_id = $1 and deck_id = $2`, userID, deckID)
	if err != nil {
		return res, err
	}
	// tag::seen-fill[]
	for rows.Next() {
		var f string
		// end::seen-fill[]
		if err := rows.Scan(&f); err != nil {
			rows.Close()
			return res, err
		}
		// tag::seen-mark[]
		seen[f] = true
	}
	// end::seen-mark[]
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return res, err
	}
	defer tx.Rollback(ctx)
	// tag::dedup-key[]
	for _, in := range inputs {
		key := strings.ToLower(strings.TrimSpace(in.Text))
		if key == "" || seen[key] {
			res.Skipped++
			continue
		}
		seen[key] = true
		// end::dedup-key[]
		var cardID uuid.UUID
		err := tx.QueryRow(ctx,
			`insert into cards (user_id, deck_id, text, meaning, card_type, tags, phonetic, example, notes)
			 values ($1, $2, $3, $4, $5, $6, $7, $8, $9) returning id`,
			userID, deckID, strings.TrimSpace(in.Text), in.Meaning, in.CardType, in.Tags,
			in.Phonetic, in.Example, in.Notes).Scan(&cardID)
		if err != nil {
			return res, err
		}
		if _, err := tx.Exec(ctx,
			`insert into card_srs (card_id, user_id) values ($1, $2)`, cardID, userID); err != nil {
			return res, err
		}
		res.Added++
	}
	return res, tx.Commit(ctx)
}

func (s *Store) DueCards(ctx context.Context, userID uuid.UUID, dueBefore time.Time, limit int) ([]Card, error) {
	rows, err := s.pool.Query(ctx,
		cardSelect+` where user_id = $1 and due_at <= $2 order by due_at asc limit $3`,
		userID, dueBefore, limit)
	if err != nil {
		return nil, err
	}
	return s.collectCards(rows)
}

func (s *Store) DueCount(ctx context.Context, userID uuid.UUID, dueBefore time.Time) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`select count(*) from card_srs where user_id = $1 and due_at <= $2`, userID, dueBefore).Scan(&n)
	return n, err
}

// CardsByRule evaluates a smart rule and returns matching cards in rule order.
func (s *Store) CardsByRule(ctx context.Context, userID uuid.UUID, rule smartrules.Rule) ([]Card, error) {
	q, extra := rule.Query()
	args := append([]any{userID}, extra...)
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []Card{}, nil
	}

	// Pass ids as pgtype.UUID: the pool runs the simple protocol (Supabase's
	// transaction pooler), where pgx has no text encoder for a []uuid.UUID
	// slice against an unknown parameter type. pgtype.UUID is pgx's native
	// uuid type, so it encodes itself and no ::uuid[] cast is needed.
	pgIDs := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		pgIDs[i] = pgtype.UUID{Bytes: id, Valid: true}
	}
	rows, err = s.pool.Query(ctx, cardSelect+` where user_id = $1 and id = any($2)`, userID, pgIDs)
	if err != nil {
		return nil, err
	}
	cards, err := s.collectCards(rows)
	if err != nil {
		return nil, err
	}
	return SortCardsByIDOrder(cards, ids), nil
}

// SortCardsByIDOrder puts cards back in the order ids lists them, dropping any
// id that no longer resolves to a card.
//
// 스마트 규칙이 정한 순서(오답률 높은 순 등)는 ID 목록에만 남아 있다. 카드 본문을
// 가져오는 두 번째 조회는 그 순서를 지켜 주지 않으므로 여기서 되살린다. SQLite
// 구현(internal/litestore)도 같은 사정이라 이 함수를 함께 쓴다.
// tag::sort-by-id-order[]
func SortCardsByIDOrder(cards []Card, ids []uuid.UUID) []Card {
	cardOf := make(map[uuid.UUID]Card, len(cards))
	for _, card := range cards {
		cardOf[card.ID] = card
	}
	ordered := make([]Card, 0, len(cards))
	for _, id := range ids {
		if card, ok := cardOf[id]; ok {
			ordered = append(ordered, card)
		}
	}
	return ordered
}

// end::sort-by-id-order[]

func (s *Store) CountByRule(ctx context.Context, userID uuid.UUID, rule smartrules.Rule) (int, error) {
	q, extra := rule.CountQuery()
	args := append([]any{userID}, extra...)
	var n int
	err := s.pool.QueryRow(ctx, q, args...).Scan(&n)
	return n, err
}
