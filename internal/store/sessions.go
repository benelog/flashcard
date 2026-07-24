package store

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
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

// 학습 방향. DB의 study_sessions.direction 열이 받는 값과 같다. 방향이 들어오는
// 길은 JSON API, 학습 시작 화면의 선택, 채점 폼의 hidden 필드, 지난 선택을 기억한
// 쿠키 넷인데, 어느 길로 들어오든 여기 있는 값 하나로 정해지도록 판정을 모았다.
const (
	TextToMeaning = "text_to_meaning"
	MeaningToText = "meaning_to_text"

	// DefaultDirection은 방향을 고르지 않았을 때의 값이다.
	DefaultDirection = TextToMeaning
)

// Directions lists every accepted direction, in the order the UI offers them.
var Directions = []string{TextToMeaning, MeaningToText}

// IsDirection reports whether d is one of the accepted directions. JSON API
// 요청처럼 잘못된 값을 400으로 되돌려 줘야 하는 곳에서 쓴다.
func IsDirection(d string) bool {
	return slices.Contains(Directions, d)
}

// NormalizeDirection maps blank or unrecognized input to DefaultDirection.
// 쿠키와 폼처럼 브라우저가 실어 오는 값에서 쓴다. 되돌려 보낼 화면이 없는
// 자리이므로, 알아볼 수 없는 값이면 기본 방향으로 학습을 이어 가는 편이 낫다.
func NormalizeDirection(d string) string {
	if IsDirection(d) {
		return d
	}
	return DefaultDirection
}

// 학습 모드. 세션이 어떤 카드 묶음을 도는지다. deck은 한 덱 전체, due는 오늘
// 복습할 카드, smart는 스마트 규칙이 고른 카드다.
const (
	ModeDeck  = "deck"
	ModeDue   = "due"
	ModeSmart = "smart"

	// DefaultMode는 모드를 지정하지 않고 학습 화면에 들어왔을 때의 값이다.
	DefaultMode = ModeDue
)

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
