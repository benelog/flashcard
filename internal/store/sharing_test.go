package store

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
	// The generator must be random, not constant. Collisions in the 5-char space
	// are possible and handled by the unique-index retry in ShareDeck, so global
	// uniqueness is deliberately not asserted here.
	if len(distinct) < 2 {
		t.Fatalf("NewShareSlug() looks constant: %d distinct in 200 draws", len(distinct))
	}
}
