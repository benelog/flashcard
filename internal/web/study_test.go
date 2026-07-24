package web

import (
	"net/url"
	"testing"

	"github.com/benelog/flashcard/internal/store"
)

func TestStudyTitle(t *testing.T) {
	tests := []struct {
		name      string
		requested string
		mode      string
		want      string
	}{
		{"링크가 실어 온 이름을 우선", "오답률 높은 카드", store.ModeSmart, "오답률 높은 카드"},
		{"오늘 복습", "", store.ModeDue, "오늘 복습"},
		{"스마트 학습", "", store.ModeSmart, "스마트 학습"},
		{"덱 학습", "", store.ModeDeck, "덱 학습"},
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

	got, err := url.ParseQuery(withParam(base, "direction", store.MeaningToText))
	if err != nil {
		t.Fatal(err)
	}
	if got.Get("direction") != store.MeaningToText {
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
