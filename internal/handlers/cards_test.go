package handlers

import (
	"testing"

	"github.com/benelog/flashcard/internal/model"
)

// toInput은 JSON API의 카드 입력 규칙이다. 폼·CSV(정규화)와 달리 모르는 종류를
// 400으로 거절하는 것이 의도된 차이이므로, 그 계약을 표로 고정한다.
func TestCardBodyToInput(t *testing.T) {
	t.Run("트리밍과 기본 종류", func(t *testing.T) {
		b := cardBody{Text: "  run  ", Meaning: " 달리다 "}
		in, msg := b.toInput()
		if msg != "" {
			t.Fatalf("msg = %q, want accepted", msg)
		}
		if in.Text != "run" || in.Meaning != "달리다" {
			t.Errorf("text/meaning not trimmed: %+v", in)
		}
		if in.CardType != model.DefaultCardType {
			t.Errorf("CardType = %q, want the default", in.CardType)
		}
		if in.Tags == nil || len(in.Tags) != 0 {
			t.Errorf("Tags = %#v, want an empty (non-nil) slice", in.Tags)
		}
	})

	t.Run("원문과 뜻은 필수", func(t *testing.T) {
		for name, b := range map[string]cardBody{
			"뜻 없음":  {Text: "run"},
			"원문 없음": {Meaning: "달리다"},
			"공백뿐":   {Text: "   ", Meaning: "달리다"},
		} {
			if _, msg := b.toInput(); msg != "text and meaning are required" {
				t.Errorf("%s: msg = %q", name, msg)
			}
		}
	})

	t.Run("모르는 종류는 거절", func(t *testing.T) {
		b := cardBody{Text: "run", Meaning: "달리다", CardType: "nonsense"}
		if _, msg := b.toInput(); msg == "" {
			t.Error("unknown cardType accepted, want a 400 message")
		}
	})

	t.Run("아는 종류는 통과", func(t *testing.T) {
		b := cardBody{Text: "run", Meaning: "달리다", CardType: model.CardTypeIdiom, Tags: []string{"동사"}}
		in, msg := b.toInput()
		if msg != "" || in.CardType != model.CardTypeIdiom || len(in.Tags) != 1 {
			t.Errorf("in = %+v, msg = %q", in, msg)
		}
	})
}
