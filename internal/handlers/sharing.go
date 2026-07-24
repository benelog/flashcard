package handlers

import (
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"

	"github.com/benelog/flashcard/internal/auth"
)

var slugPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{4,64}$`)

func pathSlug(c *gin.Context) (string, bool) {
	slug := c.Param("slug")
	if !slugPattern.MatchString(slug) {
		badRequest(c, "invalid slug")
		return "", false
	}
	return slug, true
}

func (h *Handlers) ShareDeck(c *gin.Context) {
	deckID, ok := h.deckIDFromPath(c)
	if !ok {
		return
	}
	info, err := h.Store.ShareDeck(c.Request.Context(), auth.UserID(c), deckID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, info)
}

func (h *Handlers) UnshareDeck(c *gin.Context) {
	deckID, ok := h.deckIDFromPath(c)
	if !ok {
		return
	}
	if err := h.Store.UnshareDeck(c.Request.Context(), auth.UserID(c), deckID); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handlers) ListSharedDecks(c *gin.Context) {
	decks, err := h.Store.ListSharedDecks(c.Request.Context(), auth.OptionalUserID(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, decks)
}

// GetSharedDeck returns the shared deck's summary plus its full card content
// for preview.
func (h *Handlers) GetSharedDeck(c *gin.Context) {
	slug, ok := pathSlug(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	deck, err := h.Store.GetSharedDeck(ctx, auth.OptionalUserID(c), slug)
	if err != nil {
		fail(c, err)
		return
	}
	cards, err := h.Store.GetSharedDeckCards(ctx, slug)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deck": deck, "cards": cards})
}

func (h *Handlers) ImportSharedDeck(c *gin.Context) {
	slug, ok := pathSlug(c)
	if !ok {
		return
	}
	deck, err := h.Store.ImportSharedDeck(c.Request.Context(), auth.UserID(c), slug)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, deck)
}
