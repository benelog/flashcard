package smartrules

import "testing"

func TestParseDefaults(t *testing.T) {
	r, err := Parse([]byte(`{"type":"high_error"}`))
	if err != nil {
		t.Fatal(err)
	}
	if r.MinAttempts != 3 || r.MinErrorRate != 0.4 || r.Limit != 20 {
		t.Fatalf("defaults not applied: %+v", r)
	}
}

func TestParseRejectsUnknownType(t *testing.T) {
	if _, err := Parse([]byte(`{"type":"nope"}`)); err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestTagRuleRequiresTags(t *testing.T) {
	if _, err := Parse([]byte(`{"type":"tag"}`)); err == nil {
		t.Fatal("expected error for empty tags")
	}
}

func TestLimitClamped(t *testing.T) {
	r, err := Parse([]byte(`{"type":"stale","limit":9999}`))
	if err != nil {
		t.Fatal(err)
	}
	if r.Limit != 20 {
		t.Fatalf("limit = %d, want clamped 20", r.Limit)
	}
}

// tag::queries-build[]
func TestQueriesBuild(t *testing.T) {
	for _, raw := range []string{
		`{"type":"high_error"}`,
		`{"type":"stale"}`,
		`{"type":"tag","tags":["verb"]}`,
		`{"type":"recent"}`,
	} {
		r, err := Parse([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		if q, _ := r.Query(); q == "" {
			t.Fatalf("empty query for %s", raw)
		}
		// end::queries-build[]
		if q, _ := r.CountQuery(); q == "" {
			t.Fatalf("empty count query for %s", raw)
		}
	}
}
