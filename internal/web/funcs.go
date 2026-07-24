package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"math"
	"strings"
	"time"

	"github.com/benelog/flashcard/internal/model"
	"github.com/benelog/flashcard/internal/smartrules"
)

func isNotFound(err error) bool {
	return errors.Is(err, model.ErrNotFound)
}

// funcMap은 템플릿 안에서 부를 수 있는 함수 목록이다. 왼쪽 이름이 템플릿 표기,
// 오른쪽이 실제 함수다.
// tag::func-map[]
var funcMap = template.FuncMap{
	"icon":         icon,
	"asset":        asset,
	"hasPrefix":    strings.HasPrefix,
	"orEmpty":      model.OrEmpty,
	"percent":      percent,
	"ratioPercent": ratioPercent,
	"add":          func(a, b int) int { return a + b },
	"koDate":       koDate,
	"ruleLabel":    ruleLabelJSON,
	"ruleRaw":      func(raw json.RawMessage) string { return string(raw) },
	"cardTypes":    func() []string { return model.CardTypes },
	"typeLabel":    typeLabel,
}

// end::func-map[]

// tag::percent[]
// percent는 part/whole을 백분율로 반올림한다. whole이 0이면 0이다.
func percent(part, whole int) int {
	if whole == 0 {
		return 0
	}
	return int(math.Round(float64(part) / float64(whole) * 100))
}

// end::percent[]

// ratioPercent는 0~1로 저장된 비율(오답률 같은 값)을 백분율로 바꾼다.
func ratioPercent(ratio float64) int {
	return int(math.Round(ratio * 100))
}

// koDate는 공유 덱 갤러리에 보이는 형식으로 날짜를 만든다.
func koDate(t time.Time) string {
	return fmt.Sprintf("%d. %d. %d.", t.Year(), int(t.Month()), t.Day())
}

// cardTypeLabels는 model.CardType마다 한국어 이름표를 붙인다. 종류를 새로
// 만들면 목록은 store에, 이름표는 여기에 한 줄씩만 늘면 된다: 카드 폼의 라디오
// 버튼도 학습 화면의 배지도 이 둘을 돌려 만든다.
var cardTypeLabels = map[string]string{
	model.CardTypeWord:     "단어",
	model.CardTypeSentence: "문장",
	model.CardTypeIdiom:    "숙어",
	model.CardTypeConcept:  "개념",
}

func typeLabel(t string) string {
	if label, ok := cardTypeLabels[t]; ok {
		return label
	}
	return t
}

// ruleLabel은 스마트 규칙을 사람이 읽을 문구로 만든다.
func ruleLabel(r smartrules.Rule) string {
	switch r.Type {
	case smartrules.HighError:
		rate := r.MinErrorRate
		if rate == 0 {
			rate = 0.4
		}
		return fmt.Sprintf("오답률 %d%% 이상", int(math.Round(rate*100)))
	case smartrules.Stale:
		days := r.NotReviewedDays
		if days == 0 {
			days = 7
		}
		return fmt.Sprintf("%d일 이상 안 본 카드", days)
	case smartrules.Tag:
		label := ""
		for i, t := range r.Tags {
			if i > 0 {
				label += ", "
			}
			label += t
		}
		return "태그: " + label
	case smartrules.Recent:
		days := r.AddedWithinDays
		if days == 0 {
			days = 7
		}
		return fmt.Sprintf("최근 %d일 추가", days)
	}
	return string(r.Type)
}

func ruleLabelJSON(raw json.RawMessage) string {
	r, err := smartrules.Parse(raw)
	if err != nil {
		return ""
	}
	return ruleLabel(r)
}

// suggestionTitle은 홈 화면 추천 타일의 제목을 만든다.
func suggestionTitle(r smartrules.Rule, count int) string {
	n := count
	if r.Limit > 0 && n > r.Limit {
		n = r.Limit
	}
	switch r.Type {
	case smartrules.HighError:
		return fmt.Sprintf("오답률 높은 카드 %d개 복습하기", n)
	case smartrules.Stale:
		return fmt.Sprintf("오래 안 본 카드 %d개 복습하기", n)
	}
	return fmt.Sprintf("%s %d개 학습하기", ruleLabel(r), n)
}

// splitAndTrim은 구분자로 나눈 뒤 공백과 빈 항목을 버린다. 태그
// 입력("a, b,")과 학습 큐의 카드 ID 목록이 모두 이 모양이다.
func splitAndTrim(raw, separator string) []string {
	items := []string{}
	for _, item := range strings.Split(raw, separator) {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}
