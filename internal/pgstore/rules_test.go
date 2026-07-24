package pgstore

import (
	"strconv"
	"strings"
	"testing"

	"github.com/benelog/flashcard/internal/smartrules"
)

// ruleQuery는 SQL 문자열을 만들 뿐이라 DB 없이 확인할 수 있다. 짝인
// internal/litestore/rules.go도 같은 규칙을 자기 방언으로 옮기므로, 두 파일이
// 같은 규칙 목록을 빠짐없이 다루는지가 이 테스트의 관심사다.

// tag::queries-build[]
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
		if q, _ := ruleQuery(r); q == "" {
			t.Fatalf("empty query for %s", raw)
		}
		// end::queries-build[]
		if _, args := ruleQuery(r); len(args) == 0 {
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

	q, args := ruleQuery(r)

	if strings.Contains(q, "drop table") {
		t.Fatalf("the tag value was pasted into the SQL: %s", q)
	}
	if len(args) != 2 {
		t.Fatalf("args = %v, want the tag list and the limit", args)
	}
	tags, ok := args[0].([]string)
	if !ok || len(tags) != 1 || tags[0] != "'; drop table cards--" {
		t.Errorf("args[0] = %#v, want the tags passed through as a value", args[0])
	}
}

// $1은 언제나 사용자 id다. 부르는 쪽이 그 자리에 사용자를 넣고 나머지를 이어
// 붙이므로, 규칙마다 번호가 어긋나면 남의 카드를 읽게 된다.
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

		q, args := ruleQuery(r)

		if !strings.Contains(q, "user_id = $1") {
			t.Errorf("%s: query is not scoped to the user: %s", raw, q)
		}
		// 규칙이 쓰는 자리표시자는 $2부터다. 그래서 마지막 번호가 args 개수보다
		// 하나 많아야 앞뒤가 맞는다.
		want := "$" + strconv.Itoa(len(args)+1)
		if !strings.Contains(q, want) {
			t.Errorf("%s: %d args but no %s in %s", raw, len(args), want, q)
		}
	}
}

// 추천 타일의 고정 규칙도 실제로 조회문이 되어야 홈 화면까지 갈 수 있다.
func TestSuggestedRulesBuildQueries(t *testing.T) {
	for _, rule := range smartrules.Suggested() {
		if q, _ := ruleQuery(rule); q == "" {
			t.Errorf("suggested rule %+v builds no query", rule)
		}
	}
}

// 모르는 종류는 빈 문자열이다. smartrules.Parse가 먼저 걸러 주므로 여기까지
// 오지 않지만, 온다면 조용히 전체 조회가 되는 대신 아무것도 만들지 않는다.
func TestUnknownRuleTypeBuildsNothing(t *testing.T) {
	if q, args := ruleQuery(smartrules.Rule{Type: "nonsense"}); q != "" || args != nil {
		t.Errorf("ruleQuery(unknown) = %q, %v; want empty", q, args)
	}
}
