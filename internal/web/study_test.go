package web

import (
	"net/url"
	"testing"

	"github.com/benelog/flashcard/internal/model"
)

func TestStudyTitle(t *testing.T) {
	tests := []struct {
		name      string
		requested string
		mode      string
		want      string
	}{
		{"링크가 실어 온 이름을 우선", "오답률 높은 카드", model.ModeSmart, "오답률 높은 카드"},
		{"오늘 복습", "", model.ModeDue, "오늘 복습"},
		{"스마트 학습", "", model.ModeSmart, "스마트 학습"},
		{"덱 학습", "", model.ModeDeck, "덱 학습"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := studyTitle(tt.requested, tt.mode); got != tt.want {
				t.Errorf("studyTitle(%q, %q) = %q, want %q", tt.requested, tt.mode, got, tt.want)
			}
		})
	}
}

// 방향 선택 링크는 나머지 질의 문자열을 그대로 물고 가야 한다. 그러지 않으면
// 스마트 학습에서 방향을 고르는 순간 규칙이 사라진다.
func TestWithParamKeepsOtherValues(t *testing.T) {
	base := url.Values{"mode": {"smart"}, "rule": {`{"type":"stale"}`}}

	got, err := url.ParseQuery(withParam(base, "direction", model.MeaningToText))
	if err != nil {
		t.Fatal(err)
	}
	if got.Get("direction") != model.MeaningToText {
		t.Errorf("direction = %q", got.Get("direction"))
	}
	if got.Get("mode") != "smart" || got.Get("rule") != `{"type":"stale"}` {
		t.Errorf("withParam dropped other values: %v", got)
	}
	// 원본은 건드리지 않는다.
	if base.Has("direction") {
		t.Error("withParam mutated the original query")
	}
}

func TestStudyBodyViewProgress(t *testing.T) {
	view := studyBodyView{
		State: studyState{RoundCards: 4, FirstPassTotal: 4, FirstPassCorrect: 3},
		Index: 1,
	}
	if got := view.ProgressPct(); got != 25 {
		t.Errorf("ProgressPct() = %d, want 25", got)
	}
	if got := view.Accuracy(); got != 75 {
		t.Errorf("Accuracy() = %d, want 75", got)
	}

	// 카드가 없는 세션에서도 0으로 나누지 않아야 한다.
	empty := studyBodyView{}
	if got := empty.ProgressPct(); got != 0 {
		t.Errorf("empty ProgressPct() = %d, want 0", got)
	}
	if got := empty.Accuracy(); got != 0 {
		t.Errorf("empty Accuracy() = %d, want 0", got)
	}
}

// hidden 필드로 실려 온 상태는 브라우저가 고칠 수 있는 값이다. 범위를 벗어난
// 숫자의 보정과, 큐가 hidden 필드를 한 바퀴 돌아도 그대로인지(왕복)를 확인한다.
func TestStateFromValues(t *testing.T) {
	form := url.Values{
		"session":    {"s-1"},
		"direction":  {model.MeaningToText},
		"title":      {"오늘 복습"},
		"return_url": {"/decks"},
		"queue":      {"a, b ,c"},
		"missed":     {""},
		"round":      {"2"},
		"round_len":  {"3"},
		"fp_total":   {"5"},
		"fp_correct": {"4"},
		"tts_rate":   {"1.2"},
	}

	got := stateFromValues(form)

	if got.SessionID != "s-1" || got.Direction != model.MeaningToText || got.Title != "오늘 복습" {
		t.Errorf("identity fields = %+v", got)
	}
	if len(got.Queue) != 3 || got.Queue[0] != "a" || got.Queue[2] != "c" {
		t.Errorf("Queue = %v, want trimmed 3 items", got.Queue)
	}
	if len(got.Missed) != 0 {
		t.Errorf("Missed = %v, want empty", got.Missed)
	}
	if got.Round != 2 || got.RoundCards != 3 || got.FirstPassTotal != 5 || got.FirstPassCorrect != 4 {
		t.Errorf("counters = %+v", got)
	}
	if got.TtsRate != 1.2 {
		t.Errorf("TtsRate = %v", got.TtsRate)
	}
}

func TestStateFromValuesCorrectsHostileInput(t *testing.T) {
	form := url.Values{
		"round":      {"-3"},
		"tts_rate":   {"0"},
		"return_url": {"https://evil.example/phish"},
	}

	got := stateFromValues(form)

	if got.Round != 1 {
		t.Errorf("Round = %d, want corrected to 1", got.Round)
	}
	if got.TtsRate != defaultTtsRate {
		t.Errorf("TtsRate = %v, want the default", got.TtsRate)
	}
	if got.ReturnURL != "/" {
		t.Errorf("ReturnURL = %q, want other origins rejected", got.ReturnURL)
	}
	if got.Direction != model.DefaultDirection {
		t.Errorf("Direction = %q, want the default", got.Direction)
	}
}

func TestQueueJoining(t *testing.T) {
	view := studyBodyView{State: studyState{
		Queue:  []string{"a", "b"},
		Missed: nil,
	}}
	if got := view.QueueJoined(); got != "a,b" {
		t.Errorf("QueueJoined() = %q, want %q", got, "a,b")
	}
	// 빈 목록은 빈 문자열이라야 다음 요청에서 다시 빈 목록으로 읽힌다.
	if got := view.MissedJoined(); got != "" {
		t.Errorf("MissedJoined() = %q, want empty", got)
	}
	if got := splitAndTrim(view.MissedJoined(), ","); len(got) != 0 {
		t.Errorf("round trip of an empty queue = %v, want empty", got)
	}
}
