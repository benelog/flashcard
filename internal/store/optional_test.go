package store

import "testing"

// "적지 않았다"(NULL)와 "빈 값을 적었다"가 섞이면 안 된다: 공백만 있는 입력도
// 적지 않은 것으로 본다.
func TestNilIfBlank(t *testing.T) {
	for _, blank := range []string{"", "   ", "\t\n"} {
		if got := NilIfBlank(blank); got != nil {
			t.Errorf("NilIfBlank(%q) = %q, want nil", blank, *got)
		}
	}
	got := NilIfBlank("  /ˈæpl/  ")
	if got == nil || *got != "/ˈæpl/" {
		t.Errorf("NilIfBlank() = %v, want the trimmed value", got)
	}
}

func TestOrEmpty(t *testing.T) {
	if got := OrEmpty(nil); got != "" {
		t.Errorf("OrEmpty(nil) = %q, want empty", got)
	}
	value := "사과"
	if got := OrEmpty(&value); got != value {
		t.Errorf("OrEmpty() = %q, want %q", got, value)
	}
}

// 두 함수는 서로의 역이다: 값을 넣었다 꺼내면 원래 값이 나온다.
func TestOptionalRoundTrip(t *testing.T) {
	for _, v := range []string{"사과", "/ˈæpl/", "a, b"} {
		if got := OrEmpty(NilIfBlank(v)); got != v {
			t.Errorf("round trip of %q = %q", v, got)
		}
	}
}
