package web

import (
	"testing"
	"time"

	"github.com/benelog/flashcard/internal/model"
	"github.com/benelog/flashcard/internal/smartrules"
)

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"빈 입력", "", nil},
		{"공백만", " , , ", nil},
		{"앞뒤 공백 제거", " 동사 , 기초 ", []string{"동사", "기초"}},
		{"꼬리 쉼표", "동사,", []string{"동사"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitAndTrim(tt.raw, ",")
			if len(got) != len(tt.want) {
				t.Fatalf("splitAndTrim(%q) = %v, want %v", tt.raw, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("splitAndTrim(%q) = %v, want %v", tt.raw, got, tt.want)
				}
			}
		})
	}
}

// 카드 종류마다 사람이 읽을 이름이 있어야 한다. 종류를 늘리고 이름표를 빠뜨리면
// 화면에 "concept" 같은 원본 값이 그대로 새어 나온다.
func TestTypeLabelCoversEveryCardType(t *testing.T) {
	for _, cardType := range model.CardTypes {
		if label := typeLabel(cardType); label == cardType {
			t.Errorf("card type %q has no Korean label", cardType)
		}
	}
	// 모르는 값은 지어내지 않고 그대로 보여 준다.
	if got := typeLabel("banana"); got != "banana" {
		t.Errorf("typeLabel(unknown) = %q, want it unchanged", got)
	}
}

// 규칙 종류를 새로 만들면 이름표도 함께 붙여야 한다. 빠뜨리면 스마트 덱 목록에
// "high_error" 같은 내부 값이 그대로 보인다.
func TestRuleLabelCoversEveryRuleType(t *testing.T) {
	for _, rule := range []smartrules.Rule{
		{Type: smartrules.HighError, MinErrorRate: 0.4},
		{Type: smartrules.Stale, NotReviewedDays: 7},
		{Type: smartrules.Tag, Tags: []string{"동사"}},
		{Type: smartrules.Recent, AddedWithinDays: 7},
	} {
		label := ruleLabel(rule)
		if label == "" || label == string(rule.Type) {
			t.Errorf("rule type %q has no Korean label (got %q)", rule.Type, label)
		}
	}
}

func TestRuleLabel(t *testing.T) {
	tests := []struct {
		name string
		rule smartrules.Rule
		want string
	}{
		{"오답률", smartrules.Rule{Type: smartrules.HighError, MinErrorRate: 0.55}, "오답률 55% 이상"},
		{"오답률 기본값", smartrules.Rule{Type: smartrules.HighError}, "오답률 40% 이상"},
		{"오래 안 본 카드", smartrules.Rule{Type: smartrules.Stale, NotReviewedDays: 30}, "30일 이상 안 본 카드"},
		{"오래 안 본 카드 기본값", smartrules.Rule{Type: smartrules.Stale}, "7일 이상 안 본 카드"},
		{"태그 하나", smartrules.Rule{Type: smartrules.Tag, Tags: []string{"동사"}}, "태그: 동사"},
		{"태그 여럿", smartrules.Rule{Type: smartrules.Tag, Tags: []string{"동사", "기초"}}, "태그: 동사, 기초"},
		{"최근 추가", smartrules.Rule{Type: smartrules.Recent, AddedWithinDays: 3}, "최근 3일 추가"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ruleLabel(tt.rule); got != tt.want {
				t.Errorf("ruleLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

// 템플릿은 저장된 규칙 JSON을 그대로 넘긴다. 읽을 수 없는 값이 와도 화면이
// 무너지지 않고 빈 이름표로 지나가야 한다.
func TestRuleLabelJSON(t *testing.T) {
	if got := ruleLabelJSON([]byte(`{"type":"stale","notReviewedDays":14}`)); got != "14일 이상 안 본 카드" {
		t.Errorf("ruleLabelJSON() = %q", got)
	}
	for _, broken := range []string{"", "{nope", `{"type":"nonsense"}`} {
		if got := ruleLabelJSON([]byte(broken)); got != "" {
			t.Errorf("ruleLabelJSON(%q) = %q, want empty", broken, got)
		}
	}
}

// 추천 타일은 실제로 학습하게 될 장수를 말해야 한다. 규칙의 한계보다 많이 걸려도
// 한 번에 나오는 것은 한계까지뿐이다.
func TestSuggestionTitleCountsWhatWillBeStudied(t *testing.T) {
	tests := []struct {
		name  string
		rule  smartrules.Rule
		count int
		want  string
	}{
		{"한계 안", smartrules.Rule{Type: smartrules.HighError, Limit: 20}, 7,
			"오답률 높은 카드 7개 복습하기"},
		{"한계 초과", smartrules.Rule{Type: smartrules.HighError, Limit: 20}, 500,
			"오답률 높은 카드 20개 복습하기"},
		{"오래 안 본 카드", smartrules.Rule{Type: smartrules.Stale, Limit: 20}, 3,
			"오래 안 본 카드 3개 복습하기"},
		{"고정 문구가 없는 종류는 이름표로", smartrules.Rule{Type: smartrules.Recent, AddedWithinDays: 7, Limit: 20}, 5,
			"최근 7일 추가 5개 학습하기"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := suggestionTitle(tt.rule, tt.count); got != tt.want {
				t.Errorf("suggestionTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPercentAndRatioPercent(t *testing.T) {
	tests := []struct {
		name        string
		part, whole int
		want        int
	}{
		{"0으로 나누지 않는다", 5, 0, 0},
		{"절반", 1, 2, 50},
		{"반올림", 1, 3, 33},
		{"올림 쪽 반올림", 2, 3, 67},
		{"전부", 3, 3, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := percent(tt.part, tt.whole); got != tt.want {
				t.Errorf("percent(%d, %d) = %d, want %d", tt.part, tt.whole, got, tt.want)
			}
		})
	}

	// ratioPercent는 이미 0~1로 저장된 값을 받는다.
	for _, tt := range []struct {
		ratio float64
		want  int
	}{{0, 0}, {0.5, 50}, {0.333, 33}, {1, 100}} {
		if got := ratioPercent(tt.ratio); got != tt.want {
			t.Errorf("ratioPercent(%v) = %d, want %d", tt.ratio, got, tt.want)
		}
	}
}

func TestKoDate(t *testing.T) {
	if got := koDate(time.Date(2026, 7, 5, 13, 0, 0, 0, time.UTC)); got != "2026. 7. 5." {
		t.Errorf("koDate() = %q, want %q", got, "2026. 7. 5.")
	}
}
