package pgstore

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/benelog/flashcard/internal/model"

	"github.com/benelog/flashcard/internal/srs"
)

func (s *Store) CreateSession(ctx context.Context, userID uuid.UUID, mode, direction string, deckID *uuid.UUID, rule json.RawMessage, totalCards int) (model.Session, error) {
	var sess model.Session
	err := s.pool.QueryRow(ctx,
		`insert into study_sessions (user_id, mode, direction, deck_id, smart_rule, total_cards)
		 values ($1, $2, $3, $4, $5, $6)
		 returning id, mode, direction, deck_id, smart_rule, total_cards, started_at`,
		userID, mode, direction, deckID, rule, totalCards).
		Scan(&sess.ID, &sess.Mode, &sess.Direction, &sess.DeckID, &sess.SmartRule, &sess.TotalCards, &sess.StartedAt)
	return sess, err
}

func (s *Store) RecordReview(ctx context.Context, userID, sessionID, cardID uuid.UUID, result, isRetry bool) (model.ReviewOutcome, error) {
	var out model.ReviewOutcome

	var owner uuid.UUID
	err := s.pool.QueryRow(ctx,
		`select user_id from study_sessions where id = $1`, sessionID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && owner != userID) {
		return out, model.ErrNotFound
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
		return out, model.ErrNotFound
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
	// 정답/오답에 따른 증가분은 Go에서 셈해 바인딩한다. litestore의 같은 자리와
	// 같은 모양이고, SQL의 case when 세 벌보다 짧다.
	correct, incorrect := 0, 1
	if result {
		correct, incorrect = 1, 0
	}
	if _, err := tx.Exec(ctx,
		`update card_srs set
		   ease_factor = $3, interval_days = $4, repetitions = $5, due_at = $6,
		   last_reviewed_at = $7,
		   correct_count = correct_count + $8,
		   incorrect_count = incorrect_count + $9,
		   lapses = lapses + $9
		 where card_id = $1 and user_id = $2`,
		cardID, userID, next.EaseFactor, next.IntervalDays, next.Repetitions, dueAt,
		now, correct, incorrect); err != nil {
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
