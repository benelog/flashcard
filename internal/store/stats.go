package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type DailyStat struct {
	Date    string `json:"date"`
	Total   int    `json:"total"`
	Correct int    `json:"correct"`
}

// DailyStats groups reviews by local date in the given (pre-validated) IANA
// timezone, covering the last `days` days including today.
func (s *Store) DailyStats(ctx context.Context, userID uuid.UUID, tz string, days int) ([]DailyStat, error) {
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
	defer rows.Close()
	stats := []DailyStat{}
	for rows.Next() {
		var d DailyStat
		if err := rows.Scan(&d.Date, &d.Total, &d.Correct); err != nil {
			return nil, err
		}
		stats = append(stats, d)
	}
	return stats, rows.Err()
}

type DeckMastery struct {
	DeckID      uuid.UUID `json:"deckId"`
	Name        string    `json:"name"`
	TotalCards  int       `json:"totalCards"`
	MatureCards int       `json:"matureCards"` // interval_days >= 21
}

type Summary struct {
	TotalReviews   int           `json:"totalReviews"`
	CorrectReviews int           `json:"correctReviews"`
	Streak         int           `json:"streak"`
	Decks          []DeckMastery `json:"decks"`
}

func (s *Store) StatsSummary(ctx context.Context, userID uuid.UUID, tz string, loc *time.Location) (Summary, error) {
	var sum Summary
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
	days := []time.Time{}
	for rows.Next() {
		var d time.Time
		if err := rows.Scan(&d); err != nil {
			rows.Close()
			return sum, err
		}
		days = append(days, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return sum, err
	}
	sum.Streak = Streak(days, time.Now().In(loc))

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
	defer rows.Close()
	sum.Decks = []DeckMastery{}
	for rows.Next() {
		var m DeckMastery
		if err := rows.Scan(&m.DeckID, &m.Name, &m.TotalCards, &m.MatureCards); err != nil {
			return sum, err
		}
		sum.Decks = append(sum.Decks, m)
	}
	return sum, rows.Err()
}

// Streak counts consecutive study days ending today, or yesterday when today's
// studying hasn't started yet: 하루가 아직 끝나지 않았다는 이유로 어제까지 쌓은
// 기록을 0으로 만들면 안 된다.
//
// reviewDays는 사용자의 시간대로 읽은 학습 시각들이고, 순서도 중복도 상관없다.
// Postgres 구현과 SQLite 구현이 각각 다른 방식으로 세던 것을 이 함수 하나로
// 모았으므로 두 환경의 연속일 수가 어긋날 일이 없다.
func Streak(reviewDays []time.Time, now time.Time) int {
	studied := make(map[string]bool, len(reviewDays))
	for _, d := range reviewDays {
		studied[d.Format(time.DateOnly)] = true
	}
	// 날짜만 세면 되므로 시각은 정오에 고정한다. 자정에서 하루씩 빼면 서머타임이
	// 시작되는 날 없는 시각이 되어 Go가 날짜를 하루 밀어 버릴 수 있다.
	day := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
	if !studied[day.Format(time.DateOnly)] {
		day = day.AddDate(0, 0, -1)
	}
	count := 0
	for studied[day.Format(time.DateOnly)] {
		count++
		day = day.AddDate(0, 0, -1)
	}
	return count
}
