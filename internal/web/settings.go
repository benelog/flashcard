package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/model"
	"github.com/benelog/flashcard/internal/study"
	"github.com/gin-gonic/gin"
)

// 설정 화면과, 프로필에 저장되는 설정 JSON의 해석.

// profileSettings는 profiles.settings에 저장되는 JSON과 같은 모양이다.
type profileSettings struct {
	TtsRate   float64 `json:"ttsRate,omitempty"`
	DailyGoal int     `json:"dailyGoal,omitempty"`
}

// 설정을 저장한 적 없는 사용자, 그리고 저장된 값이 범위를 벗어난 경우의 값이다.
// 하루 학습량의 숫자는 JSON API의 limit 보정과 같아야 하므로 internal/study가
// 정한다.
const (
	defaultTtsRate   = 0.9
	defaultDailyGoal = study.DefaultDailyGoal
)

func defaultSettings() profileSettings {
	return profileSettings{TtsRate: defaultTtsRate, DailyGoal: defaultDailyGoal}
}

// settingsFrom은 프로필의 설정 JSON을 읽고, 없거나 범위 밖인 값은 기본값으로
// 채운다. 설정 JSON은 사용자가 바꿔 온 것이 아니라 우리가 쓴 것이지만, 예전
// 버전이 쓴 값이 남아 있을 수 있어 늘 검사한다.
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
	minDailyGoal, maxDailyGoal = 5, study.MaxDailyGoal
)

// settingsFromForm은 제출된 설정을 읽는다. 없거나 깨졌거나 범위를 벗어난 값은
// 기본값을 그대로 둔다.
func settingsFromForm(form url.Values) profileSettings {
	settings := defaultSettings()
	if rate, err := strconv.ParseFloat(form.Get("tts_rate"), 64); err == nil &&
		rate >= minTtsRate && rate <= maxTtsRate {
		settings.TtsRate = rate
	}
	if goal, err := strconv.Atoi(form.Get("daily_goal")); err == nil &&
		goal >= minDailyGoal && goal <= maxDailyGoal {
		settings.DailyGoal = goal
	}
	return settings
}

func (w *Web) saveSettings(c *gin.Context) {
	form := postFormValues(c)
	name := strings.TrimSpace(form.Get("display_name"))
	raw, _ := json.Marshal(settingsFromForm(form))
	if _, err := w.store.UpdateProfile(c.Request.Context(), auth.UserID(c), &name, raw); err != nil {
		w.failPage(c, err)
		return
	}
	redirectWithFlash(c, flashInfo, "저장했어요", "/settings")
}
