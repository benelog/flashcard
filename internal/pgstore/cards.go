package pgstore

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/benelog/flashcard/internal/model"

	"github.com/benelog/flashcard/internal/smartrules"
)

const cardSelect = `
	select id, deck_id, text, meaning, card_type, tags, phonetic, example,
	       notes, created_at, attempts, error_rate, interval_days, due_at, last_reviewed_at
	from cards_with_stats`

func scanCard(row pgx.Row) (model.Card, error) {
	var c model.Card
	err := row.Scan(&c.ID, &c.DeckID, &c.Text, &c.Meaning, &c.CardType, &c.Tags,
		&c.Phonetic, &c.Example, &c.Notes, &c.CreatedAt,
		&c.Attempts, &c.ErrorRate, &c.IntervalDays, &c.DueAt, &c.LastReviewed)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, model.ErrNotFound
	}
	return c, err
}

func (s *Store) collectCards(rows pgx.Rows) ([]model.Card, error) {
	defer rows.Close()
	cards := []model.Card{}
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

func (s *Store) ListCards(ctx context.Context, userID, deckID uuid.UUID) ([]model.Card, error) {
	rows, err := s.pool.Query(ctx,
		cardSelect+` where user_id = $1 and deck_id = $2 order by created_at desc`, userID, deckID)
	if err != nil {
		return nil, err
	}
	return s.collectCards(rows)
}

func (s *Store) GetCard(ctx context.Context, userID, cardID uuid.UUID) (model.Card, error) {
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
	c.DeckSlug = model.EncodeDeckSlug(seq)
	return c, nil
}

// CreateCard inserts the card and its SRS row in one transaction; the deck
// ownership check doubles as the foreign-key guard.
func (s *Store) CreateCard(ctx context.Context, userID uuid.UUID, in model.CardInput) (model.Card, error) {
	if _, err := s.GetDeck(ctx, userID, in.DeckID); err != nil {
		return model.Card{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return model.Card{}, err
	}
	defer tx.Rollback(ctx)

	var cardID uuid.UUID
	err = tx.QueryRow(ctx,
		`insert into cards (user_id, deck_id, text, meaning, card_type, tags, phonetic, example, notes)
		 values ($1, $2, $3, $4, $5, $6, $7, $8, $9) returning id`,
		userID, in.DeckID, in.Text, in.Meaning, in.CardType, in.Tags, in.Phonetic, in.Example, in.Notes).
		Scan(&cardID)
	if err != nil {
		return model.Card{}, err
	}
	if _, err := tx.Exec(ctx,
		`insert into card_srs (card_id, user_id) values ($1, $2)`, cardID, userID); err != nil {
		return model.Card{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.Card{}, err
	}
	return s.GetCard(ctx, userID, cardID)
}

func (s *Store) UpdateCard(ctx context.Context, userID, cardID uuid.UUID, in model.CardInput) (model.Card, error) {
	err := requireRowAffected(s.pool.Exec(ctx,
		`update cards set
		   text = $3, meaning = $4, card_type = $5, tags = $6,
		   phonetic = $7, example = $8, notes = $9, updated_at = now()
		 where user_id = $1 and id = $2`,
		userID, cardID, in.Text, in.Meaning, in.CardType, in.Tags, in.Phonetic, in.Example, in.Notes))
	if err != nil {
		return model.Card{}, err
	}
	return s.GetCard(ctx, userID, cardID)
}

func (s *Store) DeleteCard(ctx context.Context, userID, cardID uuid.UUID) error {
	return requireRowAffected(s.pool.Exec(ctx,
		`delete from cards where user_id = $1 and id = $2`, userID, cardID))
}

// BulkCreateCards inserts many cards, skipping texts that already exist
// in the deck (or repeat within the batch), compared case- and space-insensitively.
func (s *Store) BulkCreateCards(ctx context.Context, userID, deckID uuid.UUID, inputs []model.CardInput) (model.BulkResult, error) {
	var res model.BulkResult
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

func (s *Store) DueCards(ctx context.Context, userID uuid.UUID, dueBefore time.Time, limit int) ([]model.Card, error) {
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
func (s *Store) CardsByRule(ctx context.Context, userID uuid.UUID, rule smartrules.Rule) ([]model.Card, error) {
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
		return []model.Card{}, nil
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
	return model.SortCardsByIDOrder(cards, ids), nil
}

func (s *Store) CountByRule(ctx context.Context, userID uuid.UUID, rule smartrules.Rule) (int, error) {
	q, extra := rule.CountQuery()
	args := append([]any{userID}, extra...)
	var n int
	err := s.pool.QueryRow(ctx, q, args...).Scan(&n)
	return n, err
}
