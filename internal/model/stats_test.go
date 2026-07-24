package model

import (
	"testing"
	"time"
)

// seoul은 UTC와 날짜가 갈리는 시간대다. 연속일 계산이 사용자의 하루를 기준으로
// 도는지 확인하는 데 쓴다.
var seoul = time.FixedZone("KST", 9*60*60)

// daysAgo는 now에서 거꾸로 센 학습 시각을 원소마다 하나씩 만든다.
func daysAgo(now time.Time, offsets ...int) []time.Time {
	times := make([]time.Time, 0, len(offsets))
	for _, d := range offsets {
		times = append(times, now.AddDate(0, 0, -d))
	}
	return times
}

func TestStreak(t *testing.T) {
	now := time.Date(2026, 7, 25, 21, 30, 0, 0, seoul)

	tests := []struct {
		name string
		days []time.Time
		want int
	}{
		{"공부한 적 없으면 0", nil, 0},
		{"오늘만 공부했으면 1", daysAgo(now, 0), 1},
		{"오늘부터 사흘 연속", daysAgo(now, 0, 1, 2), 3},
		// 오늘은 아직 안 했을 뿐 어제까지의 기록은 살아 있어야 한다.
		{"어제 끝난 기록은 유지", daysAgo(now, 1, 2), 2},
		{"그제까지만 했으면 끊김", daysAgo(now, 2, 3), 0},
		{"중간이 비면 거기서 멈춤", daysAgo(now, 0, 1, 3, 4), 2},
		// 하루에 여러 번 공부해도 하루로 센다. 순서도 상관없다.
		{"같은 날 여러 번은 하루", daysAgo(now, 0, 0, 1, 1, 1), 2},
		{"뒤죽박죽 순서도 같은 답", daysAgo(now, 2, 0, 1), 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Streak(tt.days, now); got != tt.want {
				t.Errorf("Streak() = %d, want %d", got, tt.want)
			}
		})
	}
}

// 사용자의 자정 직후, 어제 늦은 밤에 공부한 기록은 "어제"로 세어야 한다. UTC로
// 날짜를 끊으면 두 시각이 같은 날이 되어 연속일이 잘못 늘어난다.
func TestStreakUsesLocalDay(t *testing.T) {
	now := time.Date(2026, 7, 25, 0, 30, 0, 0, seoul)
	lateLastNight := time.Date(2026, 7, 24, 23, 50, 0, 0, seoul)
	theDayBefore := time.Date(2026, 7, 23, 20, 0, 0, 0, seoul)

	if got := Streak([]time.Time{lateLastNight, theDayBefore}, now); got != 2 {
		t.Errorf("Streak() = %d, want 2", got)
	}
}

// 연속일은 사용자가 오래 쉬었다가 돌아와도 마지막 구간만 센다.
func TestStreakCountsOnlyTheLatestRun(t *testing.T) {
	now := time.Date(2026, 7, 25, 9, 0, 0, 0, seoul)
	days := daysAgo(now, 0, 1, 30, 31, 32)

	if got := Streak(days, now); got != 2 {
		t.Errorf("Streak() = %d, want 2", got)
	}
}
