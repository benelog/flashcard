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

// 규칙 하나하나가 SQL로 옮겨지는지는 저장소 쪽 테스트가 본다(방언이 둘이라
// 옮기는 코드도 둘이다). 여기서는 규칙 자체의 해석만 확인한다.

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
	}
}

// 부르는 쪽이 규칙을 손봐도 다음 호출에 새어 나가면 안 된다.
func TestSuggestedIsNotShared(t *testing.T) {
	Suggested()[0].Limit = 1

	if got := Suggested()[0].Limit; got == 1 {
		t.Error("Suggested() handed out a shared slice")
	}
}
