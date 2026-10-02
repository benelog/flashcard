package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/model"
)

func (h *Handlers) ListDecks(c *gin.Context) {
	decks, err := h.Store.ListDecks(c.Request.Context(), auth.UserID(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, decks)
}

// tag::get-deck[]
func (h *Handlers) GetDeck(c *gin.Context) {
	deck, err := h.Store.GetDeckBySlug(c.Request.Context(), auth.UserID(c), c.Param("slug"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, deck)
}

// end::get-deck[]

// tag::create-deck[]
func (h *Handlers) CreateDeck(c *gin.Context) {
	var body struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Name) == "" {
		badRequest(c, "name is required")
		return
	}
	deck, err := h.Store.CreateDeck(c.Request.Context(), auth.UserID(c), strings.TrimSpace(body.Name), body.Description)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, deck)
}

// end::create-deck[]

func (h *Handlers) UpdateDeck(c *gin.Context) {
	deckID, ok := h.deckIDFromPath(c)
	if !ok {
		return
	}
	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, "invalid body")
		return
	}
	if body.Name != nil && strings.TrimSpace(*body.Name) == "" {
		badRequest(c, "name cannot be empty")
		return
	}
	deck, err := h.Store.UpdateDeck(c.Request.Context(), auth.UserID(c), deckID, body.Name, body.Description)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, deck)
}

func (h *Handlers) DeleteDeck(c *gin.Context) {
	deckID, ok := h.deckIDFromPath(c)
	if !ok {
		return
	}
	if err := h.Store.DeleteDeck(c.Request.Context(), auth.UserID(c), deckID); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// deckStoryBody는 덱 스토리(마크다운 원문)를 주고받는 모양이다. 스토리가 없으면 null이다.
type deckStoryBody struct {
	Story *string `json:"story"`
}

func (h *Handlers) GetDeckStory(c *gin.Context) {
	deckID, ok := h.deckIDFromPath(c)
	if !ok {
		return
	}
	story, err := h.Store.DeckStory(c.Request.Context(), auth.UserID(c), deckID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, deckStoryBody{Story: story})
}

// PutDeckStory는 스토리를 통째로 바꾼다. 빈 문자열이나 null은 스토리를 지운다
// (웹 편집 화면에서 비워 저장한 것과 같다).
func (h *Handlers) PutDeckStory(c *gin.Context) {
	deckID, ok := h.deckIDFromPath(c)
	if !ok {
		return
	}
	var body deckStoryBody
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, "invalid body")
		return
	}
	story := model.NilIfBlank(model.OrEmpty(body.Story))
	if err := h.Store.UpdateDeckStory(c.Request.Context(), auth.UserID(c), deckID, story); err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, deckStoryBody{Story: story})
}
