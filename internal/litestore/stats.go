package litestore

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/model"
)

// DailyStats groups reviews by local date in the given (pre-validated) IANA
// timezone, covering the last `days` days including today. SQLite has no
// timezone database, so rows are fetched and bucketed in Go; local data stays
// small enough for that.
func (s *Store) DailyStats(ctx context.Context, userID uuid.UUID, tz string, days int) ([]model.DailyStat, error) {
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
	moments, err := collect(rows, scanReviewMoment)
	if err != nil {
		return nil, err
	}
	return bucketDaily(moments, loc), nil
}

// reviewMoment is one review_logs row, reduced to what daily bucketing needs.
type reviewMoment struct {
	At      time.Time
	Correct bool
}

func scanReviewMoment(r rowScanner) (reviewMoment, error) {
	var m reviewMoment
	var reviewedAt string
	if err := r.Scan(&reviewedAt, &m.Correct); err != nil {
		return m, err
	}
	var err error
	m.At, err = parseTime(reviewedAt)
	return m, err
}

// bucketDaily groups review moments into local dates. moments는 시각 순으로
// 정렬되어 있어야 한다: 같은 날짜는 인접해 있을 때만 한 버킷으로 합친다.
// 시계도 DB도 보지 않으므로 시간대 경계(자정 전후)를 단위 테스트로 검증한다.
func bucketDaily(moments []reviewMoment, loc *time.Location) []model.DailyStat {
	stats := []model.DailyStat{}
	for _, m := range moments {
		day := m.At.In(loc).Format(time.DateOnly)
		if len(stats) == 0 || stats[len(stats)-1].Date != day {
			stats = append(stats, model.DailyStat{Date: day})
		}
		stats[len(stats)-1].Total++
		if m.Correct {
			stats[len(stats)-1].Correct++
		}
	}
	return stats
}

func (s *Store) StatsSummary(ctx context.Context, userID uuid.UUID, tz string, loc *time.Location) (model.Summary, error) {
	var sum model.Summary
	err := s.db.QueryRowContext(ctx,
		`select count(*), coalesce(sum(result), 0)
		 from review_logs where user_id = ? and is_retry = 0`, userID.String()).
		Scan(&sum.TotalReviews, &sum.CorrectReviews)
	if err != nil {
		return sum, err
	}

	// Streak: bucket review times into local dates in Go, then count back from
	// today, letting the streak end yesterday — same semantics as model.streak.
	rows, err := s.db.QueryContext(ctx,
		`select reviewed_at from review_logs where user_id = ?`, userID.String())
	if err != nil {
		return sum, err
	}
	days, err := collect(rows, func(r rowScanner) (time.Time, error) {
		var reviewedAt string
		if err := r.Scan(&reviewedAt); err != nil {
			return time.Time{}, err
		}
		t, err := parseTime(reviewedAt)
		return t.In(loc), err
	})
	if err != nil {
		return sum, err
	}
	sum.Streak = model.Streak(days, time.Now().In(loc))

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
	sum.Decks, err = collect(rows, scanDeckMastery)
	return sum, err
}

func scanDeckMastery(r rowScanner) (model.DeckMastery, error) {
	var m model.DeckMastery
	var id string
	if err := r.Scan(&id, &m.Name, &m.TotalCards, &m.MatureCards); err != nil {
		return m, err
	}
	var err error
	m.DeckID, err = uuid.Parse(id)
	return m, err
}
