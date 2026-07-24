package web

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/smartrules"
	"github.com/gin-gonic/gin"
)

// 스마트 덱(규칙으로 카드를 고르는 가상 덱) 저장과 삭제.

func (w *Web) saveSmartDeck(c *gin.Context) {
	rule, err := smartrules.Parse([]byte(c.PostForm("rule")))
	if err != nil {
		setFlash(c, flashError, "잘못된 규칙이에요")
		redirectBack(c, "/decks")
		return
	}
	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		name = ruleLabel(rule)
	}
	normalized, _ := json.Marshal(rule)
	if _, err := w.store.CreateSmartDeck(c.Request.Context(), auth.UserID(c), name, normalized); err != nil {
		w.failPage(c, err)
		return
	}
	// 학습 화면의 저장 버튼(htmx)은 배지로 바꿔치기만 한다.
	if isHTMX(c) {
		w.renderPartial(c, "saved_badge", nil)
		return
	}
	setFlash(c, flashInfo, "스마트 덱으로 저장했어요")
	redirectBack(c, "/decks")
}

func (w *Web) deleteSmartDeck(c *gin.Context) {
	smartDeckID, ok := w.uuidFromPath(c, "id", "찾을 수 없는 스마트 덱이에요.")
	if !ok {
		return
	}
	if err := w.store.DeleteSmartDeck(c.Request.Context(), auth.UserID(c), smartDeckID); err != nil {
		w.failPage(c, err)
		return
	}
	// htmx가 목록의 해당 항목을 지우도록 빈 본문을 돌려준다.
	if isHTMX(c) {
		c.Status(http.StatusOK)
		return
	}
	redirectWithFlash(c, flashInfo, "스마트 덱을 삭제했어요", "/decks")
}
