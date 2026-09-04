package pgstore

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/benelog/flashcard/internal/model"
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

func (s *Store) ListCards(ctx context.Context, userID, deckID uuid.UUID) ([]model.Card, error) {
	// 없는 덱과 빈 덱을 구별한다. 남의 덱 id로 물으면 빈 목록이 아니라
	// ErrNotFound가 나가야 학습 세션이 남의 덱을 가리키며 만들어지지 않는다.
	if _, err := s.GetDeck(ctx, userID, deckID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		cardSelect+` where user_id = $1 and deck_id = $2 order by created_at desc`, userID, deckID)
	if err != nil {
		return nil, err
	}
	return collect(rows, scanCard)
}

func (s *Store) GetCard(ctx context.Context, userID, cardID uuid.UUID) (model.Card, error) {
	c, err := scanCard(s.pool.QueryRow(ctx, cardSelect+` where user_id = $1 and id = $2`, userID, cardID))
	if err != nil {
		return c, err
	}
	// 편집 화면은 URL에 덱 없이 /cards/{id}로 들어오므로, 뒤로 가기 링크에 쓸
	// 덱 슬러그를 함께 준다.
	var seq int64
	if err := s.pool.QueryRow(ctx, `select seq from decks where id = $1`, c.DeckID).Scan(&seq); err != nil {
		return c, err
	}
	c.DeckSlug = model.EncodeDeckSlug(seq)
	return c, nil
}

// insertCard는 tx 안에서 카드와 그 SRS 행을 넣는다. internal/litestore에 같은
// 이름의 짝이 있다: 그쪽은 SQLite에 기본값 함수가 없어 id와 시각을 Go가 만들고,
// 여기서는 열 기본값(gen_random_uuid(), now())이 만든다.
func insertCard(ctx context.Context, tx pgx.Tx, userID uuid.UUID, in model.CardInput) (uuid.UUID, error) {
	var cardID uuid.UUID
	err := tx.QueryRow(ctx,
		`insert into cards (user_id, deck_id, text, meaning, card_type, tags, phonetic, example, notes)
		 values ($1, $2, $3, $4, $5, $6, $7, $8, $9) returning id`,
		userID, in.DeckID, in.Text, in.Meaning, in.CardType, in.Tags, in.Phonetic, in.Example, in.Notes).
		Scan(&cardID)
	if err != nil {
		return uuid.Nil, err
	}
	_, err = tx.Exec(ctx,
		`insert into card_srs (card_id, user_id) values ($1, $2)`, cardID, userID)
	return cardID, err
}

// CreateCard는 카드와 그 SRS 행을 한 트랜잭션으로 넣는다. 덱 소유 확인이
// 외래 키 검사를 겸한다.
func (s *Store) CreateCard(ctx context.Context, userID uuid.UUID, in model.CardInput) (model.Card, error) {
	if _, err := s.GetDeck(ctx, userID, in.DeckID); err != nil {
		return model.Card{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return model.Card{}, err
	}
	defer tx.Rollback(ctx)

	cardID, err := insertCard(ctx, tx, userID, in)
	if err != nil {
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

// BulkCreateCards는 카드 여러 장을 넣되, 덱에 이미 있거나 배치 안에서 반복되는
// text는 대소문자·공백을 무시하고 비교해 건너뛴다.
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
		in.DeckID = deckID
		in.Text = strings.TrimSpace(in.Text)
		if _, err := insertCard(ctx, tx, userID, in); err != nil {
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
	return collect(rows, scanCard)
}

func (s *Store) DueCount(ctx context.Context, userID uuid.UUID, dueBefore time.Time) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`select count(*) from card_srs where user_id = $1 and due_at <= $2`, userID, dueBefore).Scan(&n)
	return n, err
}
