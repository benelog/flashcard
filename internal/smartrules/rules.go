// Package smartrules defines the rule types behind virtual "smart decks".
// A rule is stored as JSON and turned into a query against cards_with_stats
// at study time, so smart decks never go stale.
package smartrules

import (
	"encoding/json"
	"fmt"
)

type RuleType string

const (
	HighError RuleType = "high_error"
	Stale     RuleType = "stale"
	Tag       RuleType = "tag"
	Recent    RuleType = "recent"
)

// tag::rule[]
type Rule struct {
	Type            RuleType `json:"type"`
	MinAttempts     int      `json:"minAttempts,omitempty"`
	MinErrorRate    float64  `json:"minErrorRate,omitempty"`
	NotReviewedDays int      `json:"notReviewedDays,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	AddedWithinDays int      `json:"addedWithinDays,omitempty"`
	Limit           int      `json:"limit,omitempty"`
}

// end::rule[]

// Suggested lists the canned rules the app recommends without being asked:
// 홈 화면의 추천 타일과 JSON API의 /suggestions가 같은 묶음을 내놓도록 목록을
// 여기 하나만 둔다. 부르는 쪽이 Limit 등을 손봐도 서로 영향이 없게 매번 새로
// 만들어 돌려준다.
func Suggested() []Rule {
	return []Rule{
		{Type: HighError, MinAttempts: 3, MinErrorRate: 0.4, Limit: 20},
		{Type: Stale, NotReviewedDays: 7, Limit: 20},
	}
}

// tag::parse[]
func Parse(raw []byte) (Rule, error) {
	var r Rule
	if err := json.Unmarshal(raw, &r); err != nil {
		return r, fmt.Errorf("invalid rule json: %w", err)
	}
	return r, r.Validate()
}

// end::parse[]

// tag::validate-head[]
func (r *Rule) Validate() error {
	if r.Limit <= 0 || r.Limit > 200 {
		r.Limit = 20
	}
	switch r.Type {
	case HighError:
		if r.MinAttempts <= 0 {
			r.MinAttempts = 3
		}
		// end::validate-head[]
		if r.MinErrorRate <= 0 || r.MinErrorRate > 1 {
			r.MinErrorRate = 0.4
		}
	case Stale:
		if r.NotReviewedDays <= 0 {
			r.NotReviewedDays = 7
		}
	// tag::validate-tag[]
	case Tag:
		if len(r.Tags) == 0 {
			return fmt.Errorf("tag rule requires tags")
		}
	// end::validate-tag[]
	case Recent:
		if r.AddedWithinDays <= 0 {
			r.AddedWithinDays = 7
		}
	default:
		return fmt.Errorf("unknown rule type %q", r.Type)
	}
	return nil
}

// 규칙을 SQL로 옮기는 일은 여기 없다. 저장소마다 방언이 다르기 때문이다:
// internal/pgstore/rules.go와 internal/litestore/rules.go가 각자 자기 몫을
// 맡고, 이 패키지는 어느 DB도 모른 채 규칙의 모양과 유효성만 책임진다.
