package pgstore

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/benelog/flashcard/internal/model"
)

// DailyStats는 리뷰를 (미리 검증된) IANA 시간대의 현지 날짜로 묶어, 오늘을
// 포함한 최근 days일을 돌려준다.
func (s *Store) DailyStats(ctx context.Context, userID uuid.UUID, tz string, days int) ([]model.DailyStat, error) {
	rows, err := s.pool.Query(ctx,
		`select to_char(reviewed_at at time zone $2, 'YYYY-MM-DD') as day,
		        count(*)::int,
		        (count(*) filter (where result))::int
		 from review_logs
		 where user_id = $1
		   and reviewed_at >= ((now() at time zone $2)::date - ($3::int - 1)) at time zone $2
		 group by day
		 order by day`,
		userID, tz, days)
	if err != nil {
		return nil, err
	}
	return collect(rows, scanDailyStat)
}

func scanDailyStat(row pgx.Row) (model.DailyStat, error) {
	var d model.DailyStat
	err := row.Scan(&d.Date, &d.Total, &d.Correct)
	return d, err
}

func (s *Store) StatsSummary(ctx context.Context, userID uuid.UUID, tz string, loc *time.Location) (model.Summary, error) {
	var sum model.Summary
	err := s.pool.QueryRow(ctx,
		`select count(*)::int, (count(*) filter (where result))::int
		 from review_logs where user_id = $1 and is_retry = false`, userID).
		Scan(&sum.TotalReviews, &sum.CorrectReviews)
	if err != nil {
		return sum, err
	}

	rows, err := s.pool.Query(ctx,
		`select distinct (reviewed_at at time zone $2)::date as day
		 from review_logs where user_id = $1
		 order by day desc limit 400`, userID, tz)
	if err != nil {
		return sum, err
	}
	days, err := collect(rows, scanDay)
	if err != nil {
		return sum, err
	}
	sum.Streak = model.Streak(days, time.Now().In(loc))

	rows, err = s.pool.Query(ctx,
		`select d.id, d.name,
		        count(cs.card_id)::int,
		        (count(cs.card_id) filter (where cs.interval_days >= 21))::int
		 from decks d
		 left join cards c on c.deck_id = d.id
		 left join card_srs cs on cs.card_id = c.id
		 where d.user_id = $1
		 group by d.id, d.name, d.created_at
		 order by d.created_at desc`, userID)
	if err != nil {
		return sum, err
	}
	sum.Decks, err = collect(rows, scanDeckMastery)
	return sum, err
}

func scanDay(row pgx.Row) (time.Time, error) {
	var d time.Time
	err := row.Scan(&d)
	return d, err
}

func scanDeckMastery(row pgx.Row) (model.DeckMastery, error) {
	var m model.DeckMastery
	err := row.Scan(&m.DeckID, &m.Name, &m.TotalCards, &m.MatureCards)
	return m, err
}
