package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/benelog/flashcard/internal/auth"
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
