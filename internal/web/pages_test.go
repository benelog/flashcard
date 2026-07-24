package web

import (
	"testing"
	"time"

	"github.com/benelog/flashcard/internal/store"
)

func TestBuildChartFillsEmptyDays(t *testing.T) {
	today := time.Date(2026, 7, 25, 9, 0, 0, 0, time.UTC)
	daily := []store.DailyStat{
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
		summary store.Summary
		want    int
	}{
		{"기록 없음", store.Summary{}, -1},
		{"절반", store.Summary{TotalReviews: 4, CorrectReviews: 2}, 50},
		{"전부 정답", store.Summary{TotalReviews: 3, CorrectReviews: 3}, 100},
		{"전부 오답", store.Summary{TotalReviews: 3}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := accuracy(tt.summary); got != tt.want {
				t.Errorf("accuracy() = %d, want %d", got, tt.want)
			}
		})
	}
}

// 설정 JSON은 우리가 쓴 것이지만 예전 버전이 남긴 값이 있을 수 있어 늘 검사한다.
func TestSettingsFrom(t *testing.T) {
	tests := []struct {
		name     string
		settings string
		wantRate float64
		wantGoal int
	}{
		{"저장한 적 없음", "", defaultTtsRate, defaultDailyGoal},
		{"빈 객체", `{}`, defaultTtsRate, defaultDailyGoal},
		{"정상 값", `{"ttsRate":1.2,"dailyGoal":30}`, 1.2, 30},
		{"0은 기본값으로", `{"ttsRate":0,"dailyGoal":0}`, defaultTtsRate, defaultDailyGoal},
		{"음수도 기본값으로", `{"ttsRate":-1,"dailyGoal":-5}`, defaultTtsRate, defaultDailyGoal},
		{"깨진 JSON", `{nope`, defaultTtsRate, defaultDailyGoal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := settingsFrom(store.Profile{Settings: []byte(tt.settings)})
			if got.TtsRate != tt.wantRate || got.DailyGoal != tt.wantGoal {
				t.Errorf("settingsFrom() = %+v, want rate %v goal %d", got, tt.wantRate, tt.wantGoal)
			}
		})
	}
}

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"빈 입력", "", nil},
		{"공백만", " , , ", nil},
		{"앞뒤 공백 제거", " 동사 , 기초 ", []string{"동사", "기초"}},
		{"꼬리 쉼표", "동사,", []string{"동사"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitAndTrim(tt.raw, ",")
			if len(got) != len(tt.want) {
				t.Fatalf("splitAndTrim(%q) = %v, want %v", tt.raw, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("splitAndTrim(%q) = %v, want %v", tt.raw, got, tt.want)
				}
			}
		})
	}
}

// 카드 종류마다 사람이 읽을 이름이 있어야 한다. 종류를 늘리고 이름표를 빠뜨리면
// 화면에 "concept" 같은 원본 값이 그대로 새어 나온다.
func TestTypeLabelCoversEveryCardType(t *testing.T) {
	for _, cardType := range store.CardTypes {
		if label := typeLabel(cardType); label == cardType {
			t.Errorf("card type %q has no Korean label", cardType)
		}
	}
	// 모르는 값은 지어내지 않고 그대로 보여 준다.
	if got := typeLabel("banana"); got != "banana" {
		t.Errorf("typeLabel(unknown) = %q, want it unchanged", got)
	}
}
