package litestore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/model"
)

const cardSelect = `
	select id, deck_id, text, meaning, card_type, tags, phonetic, example,
	       notes, created_at, attempts, error_rate, interval_days, due_at, last_reviewed_at
	from cards_with_stats`

func scanCard(r rowScanner) (model.Card, error) {
	var c model.Card
	var id, deckID, tags, createdAt, dueAt string
	var lastReviewed sql.NullString
	err := r.Scan(&id, &deckID, &c.Text, &c.Meaning, &c.CardType, &tags,
		&c.Phonetic, &c.Example, &c.Notes, &createdAt,
		&c.Attempts, &c.ErrorRate, &c.IntervalDays, &dueAt, &lastReviewed)
	if errors.Is(err, sql.ErrNoRows) {
		return c, model.ErrNotFound
	}
	if err != nil {
		return c, err
	}
	if c.ID, err = uuid.Parse(id); err != nil {
		return c, err
	}
	if c.DeckID, err = uuid.Parse(deckID); err != nil {
		return c, err
	}
	if err = json.Unmarshal([]byte(tags), &c.Tags); err != nil {
		return c, err
	}
	if c.CreatedAt, err = parseTime(createdAt); err != nil {
		return c, err
	}
	if c.DueAt, err = parseTime(dueAt); err != nil {
		return c, err
	}
	if c.LastReviewed, err = parseNullTime(lastReviewed); err != nil {
		return c, err
	}
	return c, nil
}

func (s *Store) ListCards(ctx context.Context, userID, deckID uuid.UUID) ([]model.Card, error) {
	rows, err := s.db.QueryContext(ctx,
		cardSelect+` where user_id = ? and deck_id = ? order by created_at desc`,
		userID.String(), deckID.String())
	if err != nil {
		return nil, err
	}
	return collect(rows, scanCard)
}

func (s *Store) GetCard(ctx context.Context, userID, cardID uuid.UUID) (model.Card, error) {
	c, err := scanCard(s.db.QueryRowContext(ctx,
		cardSelect+` where user_id = ? and id = ?`, userID.String(), cardID.String()))
	if err != nil {
		return c, err
	}
	// 편집 화면은 URL에 덱 없이 /cards/{id}로 들어오므로, 뒤로 가기 링크에 쓸
	// 덱 슬러그를 함께 준다.
	var seq int64
	if err := s.db.QueryRowContext(ctx,
		`select seq from decks where id = ?`, c.DeckID.String()).Scan(&seq); err != nil {
		return c, err
	}
	c.DeckSlug = model.EncodeDeckSlug(seq)
	return c, nil
}

// insertCard는 tx 안에서 카드와 그 SRS 행을 넣는다. internal/pgstore에 같은
// 이름의 짝이 있다: 그쪽은 열 기본값(gen_random_uuid(), now())이 id와 시각을
// 만들고, SQLite에는 그런 기본값이 없어 여기서 만든다.
func insertCard(ctx context.Context, tx *sql.Tx, userID uuid.UUID, in model.CardInput, now string) (uuid.UUID, error) {
	cardID := uuid.New()
	_, err := tx.ExecContext(ctx,
		`insert into cards (id, user_id, deck_id, text, meaning, card_type, tags,
		                    phonetic, example, notes, created_at, updated_at)
		 values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cardID.String(), userID.String(), in.DeckID.String(), in.Text, in.Meaning,
		in.CardType, tagsJSON(in.Tags), in.Phonetic, in.Example, in.Notes, now, now)
	if err != nil {
		return uuid.Nil, err
	}
	_, err = tx.ExecContext(ctx,
		`insert into card_srs (card_id, user_id, due_at) values (?, ?, ?)`,
		cardID.String(), userID.String(), now)
	return cardID, err
}

// CreateCard는 카드와 그 SRS 행을 한 트랜잭션으로 넣는다. 덱 소유 확인이
// 외래 키 검사를 겸한다.
func (s *Store) CreateCard(ctx context.Context, userID uuid.UUID, in model.CardInput) (model.Card, error) {
	if _, err := s.GetDeck(ctx, userID, in.DeckID); err != nil {
		return model.Card{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Card{}, err
	}
	defer tx.Rollback()

	cardID, err := insertCard(ctx, tx, userID, in, fmtTime(time.Now()))
	if err != nil {
		return model.Card{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Card{}, err
	}
	return s.GetCard(ctx, userID, cardID)
}

func (s *Store) UpdateCard(ctx context.Context, userID, cardID uuid.UUID, in model.CardInput) (model.Card, error) {
	err := requireRowAffected(s.db.ExecContext(ctx,
		`update cards set
		   text = ?, meaning = ?, card_type = ?, tags = ?,
		   phonetic = ?, example = ?, notes = ?, updated_at = ?
		 where user_id = ? and id = ?`,
		in.Text, in.Meaning, in.CardType, tagsJSON(in.Tags),
		in.Phonetic, in.Example, in.Notes, fmtTime(time.Now()),
		userID.String(), cardID.String()))
	if err != nil {
		return model.Card{}, err
	}
	return s.GetCard(ctx, userID, cardID)
}

func (s *Store) DeleteCard(ctx context.Context, userID, cardID uuid.UUID) error {
	return requireRowAffected(s.db.ExecContext(ctx,
		`delete from cards where user_id = ? and id = ?`, userID.String(), cardID.String()))
}

// BulkCreateCards는 카드 여러 장을 넣되, 덱에 이미 있거나 배치 안에서 반복되는
// text는 대소문자·공백을 무시하고 비교해 건너뛴다.
func (s *Store) BulkCreateCards(ctx context.Context, userID, deckID uuid.UUID, inputs []model.CardInput) (model.BulkResult, error) {
	var res model.BulkResult
	if _, err := s.GetDeck(ctx, userID, deckID); err != nil {
		return res, err
	}
	seen := map[string]bool{}
	rows, err := s.db.QueryContext(ctx,
		`select lower(trim(text)) from cards where user_id = ? and deck_id = ?`,
		userID.String(), deckID.String())
	if err != nil {
		return res, err
	}
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			rows.Close()
			return res, err
		}
		seen[f] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return res, err
	}
	defer tx.Rollback()
	now := fmtTime(time.Now())
	for _, in := range inputs {
		key := strings.ToLower(strings.TrimSpace(in.Text))
		if key == "" || seen[key] {
			res.Skipped++
			continue
		}
		seen[key] = true
		in.DeckID = deckID
		in.Text = strings.TrimSpace(in.Text)
		if _, err := insertCard(ctx, tx, userID, in, now); err != nil {
			return res, err
		}
		res.Added++
	}
	return res, tx.Commit()
}

func (s *Store) DueCards(ctx context.Context, userID uuid.UUID, dueBefore time.Time, limit int) ([]model.Card, error) {
	rows, err := s.db.QueryContext(ctx,
		cardSelect+` where user_id = ? and due_at <= ? order by due_at asc limit ?`,
		userID.String(), fmtTime(dueBefore), limit)
	if err != nil {
		return nil, err
	}
	return collect(rows, scanCard)
}

func (s *Store) DueCount(ctx context.Context, userID uuid.UUID, dueBefore time.Time) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`select count(*) from card_srs where user_id = ? and due_at <= ?`,
		userID.String(), fmtTime(dueBefore)).Scan(&n)
	return n, err
}
