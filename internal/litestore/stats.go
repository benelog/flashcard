package litestore

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/store"
)

// DailyStats groups reviews by local date in the given (pre-validated) IANA
// timezone, covering the last `days` days including today. SQLite has no
// timezone database, so rows are fetched and bucketed in Go; local data stays
// small enough for that.
func (s *Store) DailyStats(ctx context.Context, userID uuid.UUID, tz string, days int) ([]store.DailyStat, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -(days - 1))

	rows, err := s.db.QueryContext(ctx,
		`select reviewed_at, result from review_logs
		 where user_id = ? and reviewed_at >= ?
		 order by reviewed_at`,
		userID.String(), fmtTime(start))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stats := []store.DailyStat{}
	for rows.Next() {
		var reviewedAt string
		var result bool
		if err := rows.Scan(&reviewedAt, &result); err != nil {
			return nil, err
		}
		t, err := parseTime(reviewedAt)
		if err != nil {
			return nil, err
		}
		day := t.In(loc).Format("2006-01-02")
		if len(stats) == 0 || stats[len(stats)-1].Date != day {
			stats = append(stats, store.DailyStat{Date: day})
		}
		stats[len(stats)-1].Total++
		if result {
			stats[len(stats)-1].Correct++
		}
	}
	return stats, rows.Err()
}

func (s *Store) StatsSummary(ctx context.Context, userID uuid.UUID, tz string, loc *time.Location) (store.Summary, error) {
	var sum store.Summary
	err := s.db.QueryRowContext(ctx,
		`select count(*), coalesce(sum(result), 0)
		 from review_logs where user_id = ? and is_retry = 0`, userID.String()).
		Scan(&sum.TotalReviews, &sum.CorrectReviews)
	if err != nil {
		return sum, err
	}

	// Streak: bucket review times into local dates in Go, then count back from
	// today, letting the streak end yesterday — same semantics as store.streak.
	rows, err := s.db.QueryContext(ctx,
		`select reviewed_at from review_logs where user_id = ?`, userID.String())
	if err != nil {
		return sum, err
	}
	days := []time.Time{}
	for rows.Next() {
		var reviewedAt string
		if err := rows.Scan(&reviewedAt); err != nil {
			rows.Close()
			return sum, err
		}
		t, err := parseTime(reviewedAt)
		if err != nil {
			rows.Close()
			return sum, err
		}
		days = append(days, t.In(loc))
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return sum, err
	}
	sum.Streak = store.Streak(days, time.Now().In(loc))

	rows, err = s.db.QueryContext(ctx,
		`select d.id, d.name,
		        count(cs.card_id),
		        count(cs.card_id) filter (where cs.interval_days >= 21)
		 from decks d
		 left join cards c on c.deck_id = d.id
		 left join card_srs cs on cs.card_id = c.id
		 where d.user_id = ?
		 group by d.id, d.name, d.created_at
		 order by d.created_at desc`, userID.String())
	if err != nil {
		return sum, err
	}
	defer rows.Close()
	sum.Decks = []store.DeckMastery{}
	for rows.Next() {
		var m store.DeckMastery
		var id string
		if err := rows.Scan(&id, &m.Name, &m.TotalCards, &m.MatureCards); err != nil {
			return sum, err
		}
		if m.DeckID, err = uuid.Parse(id); err != nil {
			return sum, err
		}
		sum.Decks = append(sum.Decks, m)
	}
	return sum, rows.Err()
}
