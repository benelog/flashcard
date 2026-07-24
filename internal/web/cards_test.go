package web

import (
	"net/url"
	"testing"

	"github.com/benelog/flashcard/internal/model"
)

func TestCardInputFromForm(t *testing.T) {
	form := url.Values{
		"text":      {"  run  "},
		"meaning":   {" 달리다 "},
		"card_type": {"idiom"},
		"tags":      {"동사, 기초 ,"},
		"phonetic":  {""},
		"example":   {"Run fast."},
	}

	in, ok := cardInputFromForm(form)

	if !ok {
		t.Fatal("a filled form must be savable")
	}
	if in.Text != "run" || in.Meaning != "달리다" {
		t.Errorf("text/meaning not trimmed: %+v", in)
	}
	if in.CardType != model.CardTypeIdiom {
		t.Errorf("CardType = %q", in.CardType)
	}
	if len(in.Tags) != 2 || in.Tags[0] != "동사" || in.Tags[1] != "기초" {
		t.Errorf("Tags = %v, want trimmed 2 items", in.Tags)
	}
	if in.Phonetic != nil {
		t.Errorf("Phonetic = %v, want nil for a blank field", in.Phonetic)
	}
	if in.Example == nil || *in.Example != "Run fast." {
		t.Errorf("Example = %v", in.Example)
	}
}

// 원문과 뜻이 모두 있어야 저장할 수 있다. 공백만 있는 값은 없는 것이다.
func TestCardInputFromFormRequiresTextAndMeaning(t *testing.T) {
	for name, form := range map[string]url.Values{
		"뜻 없음":    {"text": {"run"}},
		"원문 없음":   {"meaning": {"달리다"}},
		"공백뿐인 원문": {"text": {"   "}, "meaning": {"달리다"}},
	} {
		if _, ok := cardInputFromForm(form); ok {
			t.Errorf("%s: form accepted, want rejected", name)
		}
	}
}

// 폼은 API와 달리 모르는 종류를 거절하지 않고 기본 종류로 받아 둔다. 오타 하나로
// 카드 등록 전체를 실패시키지 않기 위해서다(model.NormalizeCardType의 계약).
func TestCardInputFromFormNormalizesUnknownType(t *testing.T) {
	form := url.Values{"text": {"run"}, "meaning": {"달리다"}, "card_type": {"nonsense"}}

	in, ok := cardInputFromForm(form)

	if !ok || in.CardType != model.DefaultCardType {
		t.Errorf("CardType = %q, want the default", in.CardType)
	}
}
