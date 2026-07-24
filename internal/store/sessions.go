package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/benelog/flashcard/internal/srs"
)

type Session struct {
	ID         uuid.UUID       `json:"id"`
	Mode       string          `json:"mode"`
	Direction  string          `json:"direction"`
	DeckID     *uuid.UUID      `json:"deckId"`
	SmartRule  json.RawMessage `json:"smartRule"`
	TotalCards int             `json:"totalCards"`
	StartedAt  time.Time       `json:"startedAt"`
}

func (s *Store) CreateSession(ctx context.Context, userID uuid.UUID, mode, direction string, deckID *uuid.UUID, rule json.RawMessage, totalCards int) (Session, error) {
	var sess Session
	err := s.pool.QueryRow(ctx,
		`insert into study_sessions (user_id, mode, direction, deck_id, smart_rule, total_cards)
		 values ($1, $2, $3, $4, $5, $6)
		 returning id, mode, direction, deck_id, smart_rule, total_cards, started_at`,
		userID, mode, direction, deckID, rule, totalCards).
		Scan(&sess.ID, &sess.Mode, &sess.Direction, &sess.DeckID, &sess.SmartRule, &sess.TotalCards, &sess.StartedAt)
	return sess, err
}

type ReviewOutcome struct {
	DueAt        time.Time `json:"dueAt"`
	IntervalDays float64   `json:"intervalDays"`
}

// RecordReview logs one grade and, for first-pass grades, advances the card's
// SRS state and accuracy counters — all in a single transaction. Retry-round
// grades (isRetry) are logged only.
func (s *Store) RecordReview(ctx context.Context, userID, sessionID, cardID uuid.UUID, result, isRetry bool) (ReviewOutcome, error) {
	var out ReviewOutcome

	var owner uuid.UUID
	err := s.pool.QueryRow(ctx,
		`select user_id from study_sessions where id = $1`, sessionID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && owner != userID) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)

	var state srs.State
	err = tx.QueryRow(ctx,
		`select ease_factor, interval_days, repetitions from card_srs
		 where card_id = $1 and user_id = $2 for update`, cardID, userID).
		Scan(&state.EaseFactor, &state.IntervalDays, &state.Repetitions)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}

	if _, err := tx.Exec(ctx,
		`insert into review_logs (user_id, card_id, session_id, result, is_retry)
		 values ($1, $2, $3, $4, $5)`,
		userID, cardID, sessionID, result, isRetry); err != nil {
		return out, err
	}

	now := time.Now()
	if isRetry {
		out.DueAt = now
		out.IntervalDays = state.IntervalDays
		return out, tx.Commit(ctx)
	}

	next, dueAt := srs.Grade(state, result, now)
	if _, err := tx.Exec(ctx,
		`update card_srs set
		   ease_factor = $3, interval_days = $4, repetitions = $5, due_at = $6,
		   last_reviewed_at = $7,
		   correct_count = correct_count + case when $8 then 1 else 0 end,
		   incorrect_count = incorrect_count + case when $8 then 0 else 1 end,
		   lapses = lapses + case when $8 then 0 else 1 end
		 where card_id = $1 and user_id = $2`,
		cardID, userID, next.EaseFactor, next.IntervalDays, next.Repetitions, dueAt, now, result); err != nil {
		return out, err
	}
	out.DueAt = dueAt
	out.IntervalDays = next.IntervalDays
	return out, tx.Commit(ctx)
}

func (s *Store) FinishSession(ctx context.Context, userID, sessionID uuid.UUID, completed bool) error {
	return requireRowAffected(s.pool.Exec(ctx,
		`update study_sessions set ended_at = now(), completed = $3
		 where user_id = $1 and id = $2`, userID, sessionID, completed))
}
