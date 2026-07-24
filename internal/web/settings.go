package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/model"
	"github.com/gin-gonic/gin"
)

// 설정 화면과, 프로필에 저장되는 설정 JSON의 해석.

// profileSettings mirrors the JSON blob stored in profiles.settings.
type profileSettings struct {
	TtsRate   float64 `json:"ttsRate,omitempty"`
	DailyGoal int     `json:"dailyGoal,omitempty"`
}

// 설정을 저장한 적 없는 사용자, 그리고 저장된 값이 범위를 벗어난 경우의 값이다.
const (
	defaultTtsRate   = 0.9
	defaultDailyGoal = 50
)

func defaultSettings() profileSettings {
	return profileSettings{TtsRate: defaultTtsRate, DailyGoal: defaultDailyGoal}
}

// settingsFrom reads the profile's settings blob, falling back to the defaults
// for anything missing or out of range. 설정 JSON은 사용자가 바꿔 온 것이 아니라
// 우리가 쓴 것이지만, 예전 버전이 쓴 값이 남아 있을 수 있어 늘 검사한다.
func settingsFrom(profile model.Profile) profileSettings {
	settings := defaultSettings()
	_ = json.Unmarshal(profile.Settings, &settings)
	if settings.TtsRate <= 0 {
		settings.TtsRate = defaultTtsRate
	}
	if settings.DailyGoal <= 0 {
		settings.DailyGoal = defaultDailyGoal
	}
	return settings
}

func (w *Web) settingsPage(c *gin.Context) {
	profile, err := w.store.GetOrCreateProfile(c.Request.Context(), auth.UserID(c), "")
	if err != nil {
		w.failPage(c, err)
		return
	}
	w.render(c, http.StatusOK, "settings", "설정", gin.H{
		"Profile":  profile,
		"Settings": settingsFrom(profile),
	})
}

// 설정값이 머무를 수 있는 범위. 벗어난 값이 오면 기본값을 그대로 둔다.
const (
	minTtsRate, maxTtsRate     = 0.5, 1.5
	minDailyGoal, maxDailyGoal = 5, 200
)

func (w *Web) saveSettings(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("display_name"))
	settings := defaultSettings()
	if rate, err := strconv.ParseFloat(c.PostForm("tts_rate"), 64); err == nil &&
		rate >= minTtsRate && rate <= maxTtsRate {
		settings.TtsRate = rate
	}
	if goal, err := strconv.Atoi(c.PostForm("daily_goal")); err == nil &&
		goal >= minDailyGoal && goal <= maxDailyGoal {
		settings.DailyGoal = goal
	}
	raw, _ := json.Marshal(settings)
	if _, err := w.store.UpdateProfile(c.Request.Context(), auth.UserID(c), &name, raw); err != nil {
		w.failPage(c, err)
		return
	}
	redirectWithFlash(c, flashInfo, "저장했어요", "/settings")
}
