package model

import (
	"strings"
	"testing"
)

func TestNewShareSlug(t *testing.T) {
	distinct := make(map[string]bool)
	for i := 0; i < 200; i++ {
		slug := NewShareSlug()
		if len(slug) != shareSlugLen {
			t.Fatalf("NewShareSlug() = %q, want %d chars", slug, shareSlugLen)
		}
		for _, c := range slug {
			if !strings.ContainsRune(slugAlphabet, c) {
				t.Fatalf("NewShareSlug() = %q contains non-Base36 char %q", slug, c)
			}
		}
		distinct[slug] = true
	}
	// 생성기는 상수가 아니라 무작위여야 한다. 5글자 공간에서 충돌은 있을 수 있고
	// ShareDeck의 unique 인덱스 재시도가 처리하므로, 전역 유일성은 일부러 검사하지
	// 않는다.
	if len(distinct) < 2 {
		t.Fatalf("NewShareSlug() looks constant: %d distinct in 200 draws", len(distinct))
	}
}
