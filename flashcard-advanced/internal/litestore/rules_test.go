package litestore

import (
	"strings"
	"testing"
	"time"

	"github.com/benelog/flashcard/internal/smartrules"
)

// ruleQuery는 SQL 문자열을 만들 뿐이라 DB 없이 확인할 수 있다. 짝인
// internal/pgstore/rules_test.go가 하는 검사를 SQLite 방언으로 반복한다. 문자열
// 조립(placeholders)을 하는 쪽은 이쪽이므로, 값이 바인딩으로만 전달되는지가
// 특히 중요하다.

// ruleNow는 컷오프 계산 검증에 쓰는 고정 시각이다.
var ruleNow = time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)

func TestQueriesBuild(t *testing.T) {
	for _, raw := range []string{
		`{"type":"high_error"}`,
		`{"type":"stale"}`,
		`{"type":"tag","tags":["verb"]}`,
		`{"type":"recent"}`,
	} {
		r, err := smartrules.Parse([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		if q, _ := ruleQuery(r, ruleNow); q == "" {
			t.Fatalf("empty query for %s", raw)
		}
		if _, args := ruleQuery(r, ruleNow); len(args) == 0 {
			t.Fatalf("no bound arguments for %s", raw)
		}
	}
}

// 규칙의 값은 SQL 문자열에 이어 붙지 않고 자리표시자로 넘어가야 한다. 태그처럼
// 사용자가 직접 적은 값이 문장에 섞이면 SQL 인젝션이 열린다.
func TestRuleValuesAreBoundNotInterpolated(t *testing.T) {
	r, err := smartrules.Parse([]byte(`{"type":"tag","tags":["'; drop table cards--"]}`))
	if err != nil {
		t.Fatal(err)
	}

	q, args := ruleQuery(r, ruleNow)

	if strings.Contains(q, "drop table") {
		t.Fatalf("the tag value was pasted into the SQL: %s", q)
	}
	if len(args) != 2 {
		t.Fatalf("args = %v, want the tag and the limit", args)
	}
	if args[0] != "'; drop table cards--" {
		t.Errorf("args[0] = %#v, want the tag passed through as a value", args[0])
	}
}

// 첫 ?는 언제나 사용자 id다. 부르는 쪽이 그 자리에 사용자를 넣고 나머지를 이어
// 붙이므로, ? 개수가 args 개수보다 하나 많아야 앞뒤가 맞는다.
func TestEveryRuleScopesToTheUser(t *testing.T) {
	for _, raw := range []string{
		`{"type":"high_error"}`,
		`{"type":"stale"}`,
		`{"type":"tag","tags":["verb"]}`,
		`{"type":"recent"}`,
	} {
		r, err := smartrules.Parse([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}

		q, args := ruleQuery(r, ruleNow)

		if !strings.Contains(q, "user_id = ?") {
			t.Errorf("%s: query is not scoped to the user: %s", raw, q)
		}
		if got := strings.Count(q, "?"); got != len(args)+1 {
			t.Errorf("%s: %d placeholders for %d args + user id in %s", raw, got, len(args), q)
		}
	}
}

// SQLite에는 날짜 셈 함수를 맡길 수 없어 컷오프를 Go에서 계산한다. 그 계산이
// 주입받은 now를 기준으로 하는지 고정 시각으로 확인한다.
func TestCutoffsComeFromNow(t *testing.T) {
	stale, err := smartrules.Parse([]byte(`{"type":"stale","notReviewedDays":7}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, args := ruleQuery(stale, ruleNow); args[0] != fmtTime(ruleNow.AddDate(0, 0, -7)) {
		t.Errorf("stale cutoff = %v, want now-7d", args[0])
	}

	recent, err := smartrules.Parse([]byte(`{"type":"recent","addedWithinDays":3}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, args := ruleQuery(recent, ruleNow); args[0] != fmtTime(ruleNow.AddDate(0, 0, -3)) {
		t.Errorf("recent cutoff = %v, want now-3d", args[0])
	}
}

// 추천 타일의 고정 규칙도 실제로 조회문이 되어야 홈 화면까지 갈 수 있다.
func TestSuggestedRulesBuildQueries(t *testing.T) {
	for _, rule := range smartrules.Suggested() {
		if q, _ := ruleQuery(rule, ruleNow); q == "" {
			t.Errorf("suggested rule %+v builds no query", rule)
		}
	}
}

// 모르는 종류는 빈 문자열이다. smartrules.Parse가 먼저 걸러 주므로 여기까지
// 오지 않지만, 온다면 조용히 전체 조회가 되는 대신 아무것도 만들지 않는다.
func TestUnknownRuleTypeBuildsNothing(t *testing.T) {
	if q, args := ruleQuery(smartrules.Rule{Type: "nonsense"}, ruleNow); q != "" || args != nil {
		t.Errorf("ruleQuery(unknown) = %q, %v; want empty", q, args)
	}
}

// placeholders(0)은 빈 문자열이라 in ()이 되어 SQL 오류다. 지금은 태그 규칙
// 검증(빈 태그 거부)과 CardsByRule의 빈 목록 조기 반환이 이중으로 막고 있는데,
// 그 전제를 표로 고정해 둔다.
func TestPlaceholders(t *testing.T) {
	for _, tt := range []struct {
		n    int
		want string
	}{
		{0, ""},
		{1, "?"},
		{3, "?, ?, ?"},
	} {
		if got := placeholders(tt.n); got != tt.want {
			t.Errorf("placeholders(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}
