package web

import (
	"strings"
	"testing"

	"github.com/benelog/flashcard/internal/config"
	"github.com/benelog/flashcard/internal/model"
)

// TestTemplatesParse는 embed된 템플릿이 전부 파싱되는지, 가장 복잡한 학습
// 조각이 단계마다 그려지는지 확인한다.
func TestTemplatesParse(t *testing.T) {
	w := New(&config.Config{AuthMode: "local"}, nil)

	expected := []string{
		"home", "decks", "deck", "card_form", "study", "study_direction",
		"stats", "settings", "shared", "shared_deck", "login", "error",
	}
	for _, name := range expected {
		if _, ok := w.pages[name]; !ok {
			t.Errorf("page template %q missing", name)
		}
	}

	phonetic := "/tɛst/"
	card := &model.Card{Text: "test", Meaning: "시험", CardType: model.CardTypeWord, Phonetic: &phonetic}
	state := studyState{
		SessionID: "s", Direction: "text_to_meaning", Title: "덱 학습",
		ReturnURL: "/", Queue: []string{"a", "b"}, Round: 1, RoundCards: 2,
		FirstPassTotal: 2, TtsRate: defaultTtsRate,
	}
	for _, v := range []studyBodyView{
		{Phase: "studying", State: state, Card: card, TextTTS: "test", BackTTS: "test"},
		{Phase: "break", State: state},
		{Phase: "finished", State: state},
		{Phase: "empty", State: state},
	} {
		var sb strings.Builder
		if err := w.partials.ExecuteTemplate(&sb, "study_body", v); err != nil {
			t.Errorf("study_body phase %s: %v", v.Phase, err)
		}
		if sb.Len() == 0 {
			t.Errorf("study_body phase %s rendered empty", v.Phase)
		}
	}

	var sb strings.Builder
	if err := w.partials.ExecuteTemplate(&sb, "lookup_result", cardFormView{Status: "ok"}); err != nil {
		t.Errorf("lookup_result: %v", err)
	}
}
