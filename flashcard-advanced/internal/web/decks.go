package web

import (
	"net/http"
	"strings"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/model"
	"github.com/gin-gonic/gin"
)

// 덱 목록과 덱 상세 화면, 그리고 덱을 만들고 지우는 폼 처리.

func (w *Web) decksPage(c *gin.Context) {
	userID := auth.UserID(c)
	ctx := c.Request.Context()
	decks, err := w.store.ListDecks(ctx, userID)
	if err != nil {
		w.failPage(c, err)
		return
	}
	smartDecks, err := w.store.ListSmartDecks(ctx, userID)
	if err != nil {
		w.failPage(c, err)
		return
	}
	w.render(c, http.StatusOK, "decks", "덱", gin.H{
		"Decks":      decks,
		"SmartDecks": smartDecks,
	})
}

func (w *Web) createDeck(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		redirectWithFlash(c, flashError, "덱 이름을 입력해주세요", "/decks")
		return
	}
	deck, err := w.store.CreateDeck(c.Request.Context(), auth.UserID(c), name, nil)
	if err != nil {
		w.failPage(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/decks/"+deck.Slug)
}

func (w *Web) deckPage(c *gin.Context) {
	userID := auth.UserID(c)
	ctx := c.Request.Context()
	deck, err := w.store.GetDeckBySlug(ctx, userID, c.Param("slug"))
	if err != nil {
		w.failPage(c, err)
		return
	}
	cards, err := w.store.ListCards(ctx, userID, deck.ID)
	if err != nil {
		w.failPage(c, err)
		return
	}
	w.render(c, http.StatusOK, "deck", deck.Name, gin.H{
		"Deck":     deck,
		"Cards":    cards,
		"ShareURL": w.shareURL(c, deck),
	})
}

func (w *Web) shareURL(c *gin.Context, deck model.Deck) string {
	if deck.ShareSlug == nil {
		return ""
	}
	return origin(c) + "/shared/" + *deck.ShareSlug
}

func (w *Web) deleteDeck(c *gin.Context) {
	deckID, ok := w.deckIDFromPath(c)
	if !ok {
		return
	}
	if err := w.store.DeleteDeck(c.Request.Context(), auth.UserID(c), deckID); err != nil {
		w.failPage(c, err)
		return
	}
	setFlash(c, flashInfo, "덱을 삭제했어요")
	// 삭제한 덱의 화면에 머물 수 없으므로 목록으로 보낸다. htmx 요청은 조각만
	// 갈아 끼우므로 이동을 HX-Redirect 헤더로 지시해야 한다.
	if isHTMX(c) {
		c.Header("HX-Redirect", "/decks")
		c.Status(http.StatusOK)
		return
	}
	c.Redirect(http.StatusSeeOther, "/decks")
}
