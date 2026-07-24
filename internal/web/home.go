package web

import (
	"encoding/json"
	"net/http"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/smartrules"
	"github.com/gin-gonic/gin"
)

// 홈 화면: 오늘 복습할 카드 수, 연속 학습일, 최근 덱, 추천 타일.

type suggestionView struct {
	Title string
	Count int
	// Rule is the raw rule JSON; the template's URL context escapes it into
	// the /study?rule= link.
	Rule string
}

func (w *Web) homePage(c *gin.Context) {
	userID := auth.UserID(c)
	ctx := c.Request.Context()
	tz, loc := clientTZ(c)

	due, err := w.store.DueCount(ctx, userID, endOfToday(loc))
	if err != nil {
		w.failPage(c, err)
		return
	}
	summary, err := w.store.StatsSummary(ctx, userID, tz, loc)
	if err != nil {
		w.failPage(c, err)
		return
	}
	decks, err := w.store.ListDecks(ctx, userID)
	if err != nil {
		w.failPage(c, err)
		return
	}
	if len(decks) > 3 {
		decks = decks[:3]
	}

	// 홈 화면 추천 타일: 지금 카드가 있는 고정 규칙만 노출한다.
	var suggestions []suggestionView
	for _, rule := range smartrules.Suggested() {
		n, err := w.store.CountByRule(ctx, userID, rule)
		if err != nil {
			w.failPage(c, err)
			return
		}
		if n > 0 {
			raw, _ := json.Marshal(rule)
			suggestions = append(suggestions, suggestionView{
				Title: suggestionTitle(rule, n),
				Count: n,
				Rule:  string(raw),
			})
		}
	}

	w.render(c, http.StatusOK, "home", "Flashcard", gin.H{
		"Due":         due,
		"Streak":      summary.Streak,
		"Decks":       decks,
		"Suggestions": suggestions,
	})
}
