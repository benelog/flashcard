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

// 추천 타일은 홈 화면과 JSON API가 같은 목록을 쓴다. 그 목록이 스스로 유효한
// 규칙이어야 조회 단계까지 갈 수 있다.
func TestSuggestedRulesAreValid(t *testing.T) {
	suggested := Suggested()
	if len(suggested) == 0 {
		t.Fatal("Suggested() is empty")
	}
	for _, rule := range suggested {
		if err := rule.Validate(); err != nil {
			t.Errorf("suggested rule %+v is invalid: %v", rule, err)
		}
		if q, _ := rule.Query(); q == "" {
			t.Errorf("suggested rule %+v builds no query", rule)
		}
	}
}

// 부르는 쪽이 규칙을 손봐도 다음 호출에 새어 나가면 안 된다.
func TestSuggestedIsNotShared(t *testing.T) {
	Suggested()[0].Limit = 1

	if got := Suggested()[0].Limit; got == 1 {
		t.Error("Suggested() handed out a shared slice")
	}
}
