package web

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/study"
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

	due, err := w.store.DueCount(ctx, userID, endOfToday(time.Now(), loc))
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

	// 홈 화면 추천 타일: 지금 카드가 있는 고정 규칙만 노출한다. 어느 규칙이
	// 남는지는 JSON API의 /suggestions와 같고, 이름표만 여기서 붙인다.
	found, err := study.Suggestions(ctx, w.store, userID)
	if err != nil {
		w.failPage(c, err)
		return
	}
	suggestions := make([]suggestionView, 0, len(found))
	for _, s := range found {
		raw, _ := json.Marshal(s.Rule)
		suggestions = append(suggestions, suggestionView{
			Title: suggestionTitle(s.Rule, s.Count),
			Count: s.Count,
			Rule:  string(raw),
		})
	}

	w.render(c, http.StatusOK, "home", "Flashcard", gin.H{
		"Due":         due,
		"Streak":      summary.Streak,
		"Decks":       decks,
		"Suggestions": suggestions,
	})
}
