package web

import (
	"testing"

	"github.com/benelog/flashcard/internal/model"
)

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
			got := settingsFrom(model.Profile{Settings: []byte(tt.settings)})
			if got.TtsRate != tt.wantRate || got.DailyGoal != tt.wantGoal {
				t.Errorf("settingsFrom() = %+v, want rate %v goal %d", got, tt.wantRate, tt.wantGoal)
			}
		})
	}
}
