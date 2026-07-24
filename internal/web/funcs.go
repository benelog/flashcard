package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"math"
	"strings"
	"time"

	"github.com/benelog/flashcard/internal/smartrules"
	"github.com/benelog/flashcard/internal/store"
)

func isNotFound(err error) bool {
	return errors.Is(err, store.ErrNotFound)
}

// funcMap은 템플릿 안에서 부를 수 있는 함수 목록이다. 왼쪽 이름이 템플릿 표기,
// 오른쪽이 실제 함수다.
// tag::func-map[]
var funcMap = template.FuncMap{
	"icon":         icon,
	"asset":        asset,
	"hasPrefix":    strings.HasPrefix,
	"orEmpty":      orEmpty,
	"percent":      percent,
	"ratioPercent": ratioPercent,
	"add":          func(a, b int) int { return a + b },
	"koDate":       koDate,
	"ruleLabel":    ruleLabelJSON,
	"ruleRaw":      func(raw json.RawMessage) string { return string(raw) },
	"typeLabel":    typeLabel,
}

// end::func-map[]

// orEmpty prints an optional (nullable) field: 값이 없으면 빈칸이다. 템플릿에서
// nil 포인터를 그대로 쓰면 "<nil>"이 찍히기 때문에 반드시 거쳐야 한다.
func orEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// tag::percent[]
// percent returns round(part/whole*100), guarding the empty case.
func percent(part, whole int) int {
	if whole == 0 {
		return 0
	}
	return int(math.Round(float64(part) / float64(whole) * 100))
}

// end::percent[]

// ratioPercent turns a stored 0~1 ratio (오답률 같은 값) into a percentage.
func ratioPercent(ratio float64) int {
	return int(math.Round(ratio * 100))
}

// koDate formats a timestamp the way the shared-deck gallery shows it.
func koDate(t time.Time) string {
	return fmt.Sprintf("%d. %d. %d.", t.Year(), int(t.Month()), t.Day())
}

func typeLabel(t string) string {
	switch t {
	case "word":
		return "단어"
	case "sentence":
		return "문장"
	case "idiom":
		return "숙어"
	case "concept":
		return "개념"
	}
	return t
}

// ruleLabel renders a smart rule's human-readable description.
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

// suggestionTitle labels a home-screen recommendation tile.
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

// icons holds the inline SVG bodies (24×24 stroke drawings) used across the
// pages, so no icon font or JS icon library is needed.
var icons = map[string]string{
	"home":       `<path d="M3 11l9-8 9 8"/><path d="M5 10v10h14V10"/>`,
	"layers":     `<path d="M12 3l9 5-9 5-9-5z"/><path d="M3 13l9 5 9-5"/>`,
	"chart":      `<line x1="3" y1="20" x2="21" y2="20"/><line x1="7" y1="20" x2="7" y2="12"/><line x1="12" y1="20" x2="12" y2="6"/><line x1="17" y1="20" x2="17" y2="15"/>`,
	"settings":   `<line x1="4" y1="7" x2="20" y2="7"/><circle cx="9" cy="7" r="2.5"/><line x1="4" y1="17" x2="20" y2="17"/><circle cx="15" cy="17" r="2.5"/>`,
	"flame":      `<path d="M12 3c2 4-4 5.5-4 9.5a4 4 0 0 0 8 0c0-1.8-.9-3.2-1.8-4.2 0 1.4-.9 2.2-1.7 2.4C13.6 8.6 14 5.4 12 3z"/>`,
	"cap":        `<path d="M2 9l10-4 10 4-10 4z"/><path d="M6 11.5V16c0 1.5 3 3 6 3s6-1.5 6-3v-4.5"/>`,
	"plus":       `<line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>`,
	"sparkles":   `<path d="M12 4l1.8 4.7L18.5 10l-4.7 1.8L12 16.5l-1.8-4.7L5.5 10l4.7-1.8z"/><path d="M18.5 15.5l.8 1.7 1.7.8-1.7.8-.8 1.7-.8-1.7-1.7-.8 1.7-.8z"/>`,
	"globe":      `<circle cx="12" cy="12" r="9"/><path d="M3 12h18"/><path d="M12 3a13 13 0 0 1 0 18M12 3a13 13 0 0 0 0 18"/>`,
	"chev-right": `<path d="M9 6l6 6-6 6"/>`,
	"chev-left":  `<path d="M15 6l-6 6 6 6"/>`,
	"trash":      `<path d="M4 7h16"/><path d="M9 7V5h6v2"/><path d="M6 7l1 13h10l1-13"/>`,
	"pencil":     `<path d="M4 20l1-4L16 5l3 3-11 11z"/><path d="M14 7l3 3"/>`,
	"download":   `<path d="M12 4v10"/><path d="M8 10l4 4 4-4"/><path d="M5 20h14"/>`,
	"upload":     `<path d="M12 14V4"/><path d="M8 8l4-4 4 4"/><path d="M5 20h14"/>`,
	"share":      `<circle cx="6" cy="12" r="2.5"/><circle cx="17" cy="6" r="2.5"/><circle cx="17" cy="18" r="2.5"/><path d="M8.3 10.8l6.4-3.6M8.3 13.2l6.4 3.6"/>`,
	"link":       `<path d="M10 14a4.5 4.5 0 0 0 6.4 0l2.3-2.3a4.5 4.5 0 0 0-6.4-6.4l-1.1 1.1"/><path d="M14 10a4.5 4.5 0 0 0-6.4 0l-2.3 2.3a4.5 4.5 0 0 0 6.4 6.4l1.1-1.1"/>`,
	"link-off":   `<path d="M15 9l5-5"/><path d="M9 15l-5 5"/><path d="M13 6l1-1a4.2 4.2 0 0 1 6 6l-1 1"/><path d="M11 18l-1 1a4.2 4.2 0 0 1-6-6l1-1"/>`,
	"login":      `<path d="M10 17l5-5-5-5"/><path d="M15 12H3"/><path d="M12 3h7a1 1 0 0 1 1 1v16a1 1 0 0 1-1 1h-7"/>`,
	"logout":     `<path d="M16 17l5-5-5-5"/><path d="M21 12H9"/><path d="M12 3H5a1 1 0 0 0-1 1v16a1 1 0 0 0 1 1h7"/>`,
	"x":          `<path d="M6 6l12 12M18 6L6 18"/>`,
	"check":      `<path d="M5 13l4 4L19 7"/>`,
	"rotate":     `<path d="M3 12a9 9 0 1 0 3.2-6.9"/><path d="M3 4v5h5"/>`,
	"party":      `<path d="M5 11l8 8-11 3z"/><path d="M13 6.5l1.5-2.5M17.5 9.5l2.5-1M15 13.5l2.5 1.5M11.5 3.5l.5 2"/>`,
	"swap":       `<path d="M8 3L4 7l4 4"/><path d="M4 7h16"/><path d="M16 21l4-4-4-4"/><path d="M20 17H4"/>`,
	"arrow":      `<path d="M5 12h14"/><path d="M13 6l6 6-6 6"/>`,
	"bookmark":   `<path d="M6 3h12v18l-6-4-6 4z"/><path d="M12 7v6M9 10h6"/>`,
	"book":       `<path d="M4 5a2 2 0 0 1 2-2h14v18H6a2 2 0 0 0-2 2z"/><path d="M20 17H6a2 2 0 0 0-2 2"/>`,
	"volume":     `<path d="M11 5L6 9H3v6h3l5 4z"/><path d="M15 9a4 4 0 0 1 0 6"/><path d="M17.5 6.5a8 8 0 0 1 0 11"/>`,
	"copy":       `<rect x="9" y="9" width="11" height="11" rx="2"/><path d="M5 15V5a2 2 0 0 1 2-2h10"/>`,
}

func icon(name string) template.HTML {
	body, ok := icons[name]
	if !ok {
		return ""
	}
	return template.HTML(`<svg class="icon" viewBox="0 0 24 24" aria-hidden="true">` + body + `</svg>`)
}
