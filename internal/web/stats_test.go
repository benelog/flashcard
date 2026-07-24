package web

import (
	"testing"
	"time"

	"github.com/benelog/flashcard/internal/model"
)

func TestBuildChartFillsEmptyDays(t *testing.T) {
	today := time.Date(2026, 7, 25, 9, 0, 0, 0, time.UTC)
	daily := []model.DailyStat{
		{Date: "2026-07-24", Total: 10, Correct: 5},
		{Date: "2026-07-25", Total: 4, Correct: 4},
	}

	days := buildChart(daily, today)

	if len(days) != chartDays {
		t.Fatalf("chart has %d days, want %d", len(days), chartDays)
	}
	last, yesterday := days[len(days)-1], days[len(days)-2]
	if last.Date != "2026-07-25" || last.Total != 4 {
		t.Errorf("last day = %+v, want today with 4 reviews", last)
	}
	// 막대 높이는 가장 많이 푼 날(10회)을 100%로 삼는다.
	if yesterday.CorrectPct != 50 || yesterday.WrongPct != 50 {
		t.Errorf("busiest day bars = %d/%d, want 50/50", yesterday.CorrectPct, yesterday.WrongPct)
	}
	if last.CorrectPct != 40 || last.WrongPct != 0 {
		t.Errorf("today bars = %d/%d, want 40/0", last.CorrectPct, last.WrongPct)
	}
	// 공부하지 않은 날도 자리를 차지해야 차트가 날짜대로 늘어선다.
	if first := days[0]; first.Total != 0 || first.Date != "2026-06-26" {
		t.Errorf("first day = %+v, want an empty 2026-06-26", first)
	}
}

func TestBuildChartWithNoHistory(t *testing.T) {
	days := buildChart(nil, time.Date(2026, 7, 25, 9, 0, 0, 0, time.UTC))
	if len(days) != chartDays {
		t.Fatalf("chart has %d days, want %d", len(days), chartDays)
	}
	for _, d := range days {
		if d.Total != 0 || d.CorrectPct != 0 {
			t.Fatalf("day %s is not empty: %+v", d.Date, d)
		}
	}
}

// 한 번도 풀지 않았으면 "0%"가 아니라 "정답률 없음"이다.
func TestAccuracy(t *testing.T) {
	tests := []struct {
		name    string
		summary model.Summary
		want    int
	}{
		{"기록 없음", model.Summary{}, -1},
		{"절반", model.Summary{TotalReviews: 4, CorrectReviews: 2}, 50},
		{"전부 정답", model.Summary{TotalReviews: 3, CorrectReviews: 3}, 100},
		{"전부 오답", model.Summary{TotalReviews: 3}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := accuracy(tt.summary); got != tt.want {
				t.Errorf("accuracy() = %d, want %d", got, tt.want)
			}
		})
	}
}
