package litestore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/srs"
	"github.com/benelog/flashcard/internal/store"
)

func (s *Store) CreateSession(ctx context.Context, userID uuid.UUID, mode, direction string, deckID *uuid.UUID, rule json.RawMessage, totalCards int) (store.Session, error) {
	sess := store.Session{
		ID:         uuid.New(),
		Mode:       mode,
		Direction:  direction,
		DeckID:     deckID,
		SmartRule:  rule,
		TotalCards: totalCards,
	}
	now := fmtTime(time.Now())
	var deckIDArg any
	if deckID != nil {
		deckIDArg = deckID.String()
	}
	_, err := s.db.ExecContext(ctx,
		`insert into study_sessions (id, user_id, mode, direction, deck_id, smart_rule, total_cards, started_at)
		 values (?, ?, ?, ?, ?, ?, ?, ?)`,
		sess.ID.String(), userID.String(), mode, direction, deckIDArg, jsonArg(rule), totalCards, now)
	if err != nil {
		return store.Session{}, err
	}
	sess.StartedAt, err = parseTime(now)
	return sess, err
}

// RecordReview logs one grade and, for first-pass grades, advances the card's
// SRS state and accuracy counters — all in a single transaction. Retry-round
// grades (isRetry) are logged only.
func (s *Store) RecordReview(ctx context.Context, userID, sessionID, cardID uuid.UUID, result, isRetry bool) (store.ReviewOutcome, error) {
	var out store.ReviewOutcome

	var owner string
	err := s.db.QueryRowContext(ctx,
		`select user_id from study_sessions where id = ?`, sessionID.String()).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && owner != userID.String()) {
		return out, store.ErrNotFound
	}
	if err != nil {
		return out, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()

	// No "for update" needed: the transaction holds SQLite's single writer.
	var state srs.State
	err = tx.QueryRowContext(ctx,
		`select ease_factor, interval_days, repetitions from card_srs
		 where card_id = ? and user_id = ?`, cardID.String(), userID.String()).
		Scan(&state.EaseFactor, &state.IntervalDays, &state.Repetitions)
	if errors.Is(err, sql.ErrNoRows) {
		return out, store.ErrNotFound
	}
	if err != nil {
		return out, err
	}

	now := time.Now()
	if _, err := tx.ExecContext(ctx,
		`insert into review_logs (user_id, card_id, session_id, result, is_retry, reviewed_at)
		 values (?, ?, ?, ?, ?, ?)`,
		userID.String(), cardID.String(), sessionID.String(), result, isRetry, fmtTime(now)); err != nil {
		return out, err
	}

	if isRetry {
		out.DueAt = now
		out.IntervalDays = state.IntervalDays
		return out, tx.Commit()
	}

	next, dueAt := srs.Grade(state, result, now)
	correct, incorrect := 0, 1
	if result {
		correct, incorrect = 1, 0
	}
	if _, err := tx.ExecContext(ctx,
		`update card_srs set
		   ease_factor = ?, interval_days = ?, repetitions = ?, due_at = ?,
		   last_reviewed_at = ?,
		   correct_count = correct_count + ?,
		   incorrect_count = incorrect_count + ?,
		   lapses = lapses + ?
		 where card_id = ? and user_id = ?`,
		next.EaseFactor, next.IntervalDays, next.Repetitions, fmtTime(dueAt),
		fmtTime(now), correct, incorrect, incorrect,
		cardID.String(), userID.String()); err != nil {
		return out, err
	}
	out.DueAt = dueAt
	out.IntervalDays = next.IntervalDays
	return out, tx.Commit()
}

func (s *Store) FinishSession(ctx context.Context, userID, sessionID uuid.UUID, completed bool) error {
	return requireRowAffected(s.db.ExecContext(ctx,
		`update study_sessions set ended_at = ?, completed = ?
		 where user_id = ? and id = ?`,
		fmtTime(time.Now()), completed, userID.String(), sessionID.String()))
}
