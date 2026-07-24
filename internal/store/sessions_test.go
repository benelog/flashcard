package store

import "testing"

// 방향과 카드 종류는 DB의 열이 받는 값이면서 폼·CSV·API가 저마다 실어 오는
// 값이기도 하다. 판정이 한곳에 모여 있는지를 여기서 지킨다.

func TestNormalizeDirection(t *testing.T) {
	tests := []struct {
		name  string
		given string
		want  string
	}{
		{"빈 값", "", DefaultDirection},
		{"원문 먼저", TextToMeaning, TextToMeaning},
		{"뜻 먼저", MeaningToText, MeaningToText},
		{"모르는 값", "sideways", DefaultDirection},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeDirection(tt.given); got != tt.want {
				t.Errorf("NormalizeDirection(%q) = %q, want %q", tt.given, got, tt.want)
			}
		})
	}
}

func TestIsDirection(t *testing.T) {
	for _, d := range Directions {
		if !IsDirection(d) {
			t.Errorf("IsDirection(%q) = false, want true", d)
		}
	}
	for _, d := range []string{"", "sideways", "TEXT_TO_MEANING"} {
		if IsDirection(d) {
			t.Errorf("IsDirection(%q) = true, want false", d)
		}
	}
}

// 기본값은 반드시 허용 목록 안에 있어야 한다. 그러지 않으면 값을 정규화한 뒤에도
// DB가 거부한다.
func TestDefaultsAreValid(t *testing.T) {
	if !IsDirection(DefaultDirection) {
		t.Errorf("DefaultDirection %q is not in Directions", DefaultDirection)
	}
	if !IsCardType(DefaultCardType) {
		t.Errorf("DefaultCardType %q is not in CardTypes", DefaultCardType)
	}
}

func TestNormalizeCardType(t *testing.T) {
	tests := []struct {
		name  string
		given string
		want  string
	}{
		{"빈 값", "", DefaultCardType},
		{"아는 종류", CardTypeIdiom, CardTypeIdiom},
		{"오타", "wrod", DefaultCardType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeCardType(tt.given); got != tt.want {
				t.Errorf("NormalizeCardType(%q) = %q, want %q", tt.given, got, tt.want)
			}
		})
	}
}
