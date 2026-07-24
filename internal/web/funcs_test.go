package web

import (
	"testing"

	"github.com/benelog/flashcard/internal/model"
)

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
	for _, cardType := range model.CardTypes {
		if label := typeLabel(cardType); label == cardType {
			t.Errorf("card type %q has no Korean label", cardType)
		}
	}
	// 모르는 값은 지어내지 않고 그대로 보여 준다.
	if got := typeLabel("banana"); got != "banana" {
		t.Errorf("typeLabel(unknown) = %q, want it unchanged", got)
	}
}
