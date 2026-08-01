package litestore

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/model"
)

// DailyStats는 리뷰를 (미리 검증된) IANA 시간대의 현지 날짜로 묶어, 오늘을
// 포함한 최근 days일을 돌려준다. SQLite에는 시간대 데이터베이스가 없어 행을
// 가져와 Go에서 버킷팅한다. 로컬 데이터는 그래도 될 만큼 작다.
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

// reviewMoment는 review_logs 행 하나를 일별 버킷팅에 필요한 것만 남긴 것이다.
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

// bucketDaily는 리뷰 시각들을 현지 날짜로 묶는다. moments는 시각 순으로
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
		`select count(*), count(*) filter (where result)
		 from review_logs where user_id = ? and is_retry = 0`, userID.String()).
		Scan(&sum.TotalReviews, &sum.CorrectReviews)
	if err != nil {
		return sum, err
	}

	// 스트릭: 리뷰 시각을 Go에서 현지 날짜로 묶고 오늘부터 거슬러 센다. 어제
	// 끝난 스트릭도 인정한다(model.Streak의 의미 그대로). pgstore는 같은 조회에
	// 400일 상한을 두지만 여기는 없다: 로컬 DB는 한 사용자 것이라 그만큼 쌓이지
	// 않고, SQLite는 날짜 중복 제거를 Go에서 해야 해 어차피 전 행을 읽는다.
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
