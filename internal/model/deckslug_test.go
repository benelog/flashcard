// tag::same-package[]
package model

import (
	"strings"
	"testing"
)

func TestDeckSlugRoundTrip(t *testing.T) {
	for _, seq := range []int64{1, 2, 35, 36, 1295, 1296, 46656, 1_000_000, 1_679_615} {
		slug := EncodeDeckSlug(seq)
		// end::same-package[]
		if len(slug) != slugLen {
			t.Fatalf("EncodeDeckSlug(%d) = %q, want %d chars", seq, slug, slugLen)
		}
		got, err := DecodeDeckSlug(slug)
		if err != nil {
			t.Fatalf("DecodeDeckSlug(%q): %v", slug, err)
		}
		if got != seq {
			t.Errorf("round trip %d -> %q -> %d", seq, slug, got)
		}
	}
}

// tag::bijective[]
// 넓은 범위의 왕복 검사는 순열이나 그 역의 산술 실수를 잡고, slug가 항상
// 4글자이며 충돌하지 않음을 확인한다.
func TestDeckSlugBijective(t *testing.T) {
	seen := make(map[string]int64)
	for seq := int64(1); seq < 20000; seq++ {
		slug := EncodeDeckSlug(seq)
		if len(slug) != slugLen {
			t.Fatalf("EncodeDeckSlug(%d) = %q, want %d chars", seq, slug, slugLen)
		}
		if prev, dup := seen[slug]; dup {
			t.Fatalf("slug %q collides: seq %d and %d", slug, prev, seq)
		}
		seen[slug] = seq
		if got, err := DecodeDeckSlug(slug); err != nil || got != seq {
			t.Fatalf("round trip %d -> %q -> %d (err %v)", seq, slug, got, err)
		}
	}
}

// end::bijective[]

// slug가 순번으로 읽히면 안 된다. 이웃한 seq는 서로 무관한 slug가 된다.
func TestDeckSlugNotSequential(t *testing.T) {
	if a, b := EncodeDeckSlug(1), EncodeDeckSlug(2); a == "0001" || b == "0002" {
		t.Errorf("slugs look sequential: 1 -> %q, 2 -> %q", a, b)
	}
}

// Base36 slug는 대소문자를 가리지 않는다. 대문자로 바꾼 slug도 같은 덱을 찾는다.
func TestDeckSlugCaseInsensitive(t *testing.T) {
	for _, seq := range []int64{1, 42, 1_000_000} {
		slug := EncodeDeckSlug(seq)
		got, err := DecodeDeckSlug(strings.ToUpper(slug))
		if err != nil || got != seq {
			t.Errorf("DecodeDeckSlug(%q) = %d, %v; want %d", strings.ToUpper(slug), got, err, seq)
		}
	}
}

func TestEncodeDeckSlugOutOfRange(t *testing.T) {
	for _, seq := range []int64{0, -5, slugSpace, slugSpace + 1} {
		if got := EncodeDeckSlug(seq); got != "" {
			t.Errorf("EncodeDeckSlug(%d) = %q, want empty", seq, got)
		}
	}
}

// tag::invalid-input[]
func TestDecodeDeckSlugInvalid(t *testing.T) {
	// 길이가 틀린 것, Base36 밖의 바이트, 멀티바이트 입력, 그리고 seq 0으로
	// 되돌아가는 영 slug는 모두 거부해야 한다.
	for _, s := range []string{"", "abc", "abcde", "abc!", "한글", "0000"} {
		if _, err := DecodeDeckSlug(s); err == nil {
			t.Errorf("DecodeDeckSlug(%q) expected error", s)
		}
	}
}

// end::invalid-input[]
