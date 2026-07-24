package model

import (
	"time"

	"github.com/google/uuid"
)

type DailyStat struct {
	Date    string `json:"date"`
	Total   int    `json:"total"`
	Correct int    `json:"correct"`
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

// Streak은 오늘로 끝나는 연속 학습일 수를 센다. 오늘 학습을 아직 시작하지
// 않았으면 어제로 끝나는 구간을 센다. 하루가 아직 끝나지 않았다는 이유로
// 어제까지 쌓은 기록을 0으로 만들면 안 된다.
//
// reviewDays는 사용자의 시간대로 읽은 학습 시각들이고, 순서도 중복도 상관없다.
// 저장소 구현 둘이 각각 다른 방식으로 세던 것을 이 함수 하나로 모았으므로 두
// 환경의 연속일 수가 어긋날 일이 없다.
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
