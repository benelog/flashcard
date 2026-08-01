package litestore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/model"
	"github.com/benelog/flashcard/internal/srs"
)

func (s *Store) CreateSession(ctx context.Context, userID uuid.UUID, mode, direction string, deckID *uuid.UUID, rule json.RawMessage, totalCards int) (model.Session, error) {
	sess := model.Session{
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
		return model.Session{}, err
	}
	sess.StartedAt, err = parseTime(now)
	return sess, err
}

// RecordReview는 채점 하나를 기록하고, 첫 응답이면 카드의 SRS 상태와 정답
// 집계까지 한 트랜잭션으로 전진시킨다. 재시도 라운드(isRetry)는 기록만 한다.
func (s *Store) RecordReview(ctx context.Context, userID, sessionID, cardID uuid.UUID, result, isRetry bool) (model.ReviewOutcome, error) {
	var out model.ReviewOutcome

	var owner string
	err := s.db.QueryRowContext(ctx,
		`select user_id from study_sessions where id = ?`, sessionID.String()).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && owner != userID.String()) {
		return out, model.ErrNotFound
	}
	if err != nil {
		return out, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()

	// "for update"가 필요 없다: 트랜잭션이 SQLite의 단일 쓰기 주체를 이미 쥔다.
	var state srs.State
	err = tx.QueryRowContext(ctx,
		`select ease_factor, interval_days, repetitions from card_srs
		 where card_id = ? and user_id = ?`, cardID.String(), userID.String()).
		Scan(&state.EaseFactor, &state.IntervalDays, &state.Repetitions)
	if errors.Is(err, sql.ErrNoRows) {
		return out, model.ErrNotFound
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
