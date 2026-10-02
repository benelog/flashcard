package web

import (
	"strings"
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

// 차트보다 오래된 날이 섞여 들어와도 막대의 기준은 보이는 30일 안에서 고른다.
func TestBuildChartIgnoresOlderDays(t *testing.T) {
	today := time.Date(2026, 7, 25, 9, 0, 0, 0, time.UTC)
	daily := []model.DailyStat{
		{Date: "2026-01-01", Total: 100, Correct: 100},
		{Date: "2026-07-25", Total: 4, Correct: 4},
	}
	days := buildChart(daily, today)
	if last := days[len(days)-1]; last.CorrectPct != 100 {
		t.Errorf("today bar = %d%%, want 100%% (older busy day must not count)", last.CorrectPct)
	}
}

func TestBuildHeatmap(t *testing.T) {
	today := time.Date(2026, 10, 3, 22, 0, 0, 0, time.UTC) // 토요일
	daily := []model.DailyStat{
		{Date: "2025-09-27", Total: 50}, // 잔디보다 이른 날: 세지 않는다
		{Date: "2026-09-29", Total: 1},
		{Date: "2026-09-30", Total: 2},
		{Date: "2026-10-01", Total: 8},
		{Date: "2026-10-03", Total: 3},
	}

	h := buildHeatmap(daily, today)

	if len(h.Weeks) != heatmapWeeks {
		t.Fatalf("weeks = %d, want %d", len(h.Weeks), heatmapWeeks)
	}
	if first := h.Weeks[0].Days[0].Title; first[:10] != "2025-09-28" {
		t.Errorf("first cell = %q, want the Sunday 2025-09-28", first)
	}
	week := h.Weeks[heatmapWeeks-1].Days
	// 가장 많이 푼 날(8회)이 4단계, 한 번만 풀어도 1단계다.
	wantLevels := [7]int{0, 0, 1, 1, 4, 0, 2} // 일 9/27 ~ 토 10/3
	for d, want := range wantLevels {
		if week[d].Level != want {
			t.Errorf("%s level = %d, want %d", week[d].Title, week[d].Level, want)
		}
	}
	if !strings.HasPrefix(week[6].Title, "2026-10-03 (토)") {
		t.Errorf("today title = %q", week[6].Title)
	}
	if h.Weeks[heatmapWeeks-1].Month != "10월" {
		t.Errorf("last week month = %q, want 10월", h.Weeks[heatmapWeeks-1].Month)
	}
	if h.StudiedDays != 4 || h.Longest != 3 || !h.TodayDone {
		t.Errorf("studied=%d longest=%d todayDone=%v, want 4/3/true", h.StudiedDays, h.Longest, h.TodayDone)
	}
	// 첫 칸부터 오늘까지가 DB에서 읽어야 할 날 수다.
	if got, want := heatmapDays(today), (heatmapWeeks-1)*7+7; got != want {
		t.Errorf("heatmapDays = %d, want %d", got, want)
	}
}

// 이번 주의 남은 날은 그리지 않고, 오늘 안 했으면 TodayDone이 거짓이다.
func TestBuildHeatmapMidWeek(t *testing.T) {
	today := time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC) // 수요일
	h := buildHeatmap([]model.DailyStat{{Date: "2026-09-29", Total: 5}}, today)

	week := h.Weeks[heatmapWeeks-1].Days
	for d, cell := range week {
		if wantFuture := d > int(time.Wednesday); cell.Future != wantFuture {
			t.Errorf("day %d future = %v, want %v", d, cell.Future, wantFuture)
		}
	}
	if h.TodayDone || h.Longest != 1 {
		t.Errorf("todayDone=%v longest=%d, want false/1", h.TodayDone, h.Longest)
	}
	if got, want := heatmapDays(today), (heatmapWeeks-1)*7+4; got != want {
		t.Errorf("heatmapDays = %d, want %d", got, want)
	}
}
