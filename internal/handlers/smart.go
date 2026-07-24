package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/smartrules"
)

// Suggestions returns the home-screen tiles: canned smart rules that
// currently match at least one card.
func (h *Handlers) Suggestions(c *gin.Context) {
	userID := auth.UserID(c)
	ctx := c.Request.Context()
	canned := []smartrules.Rule{
		{Type: smartrules.HighError, MinAttempts: 3, MinErrorRate: 0.4, Limit: 20},
		{Type: smartrules.Stale, NotReviewedDays: 7, Limit: 20},
	}
	type suggestion struct {
		Type  smartrules.RuleType `json:"type"`
		Count int                 `json:"count"`
		Rule  smartrules.Rule     `json:"rule"`
	}
	out := []suggestion{}
	for _, rule := range canned {
		n, err := h.Store.CountByRule(ctx, userID, rule)
		if err != nil {
			fail(c, err)
			return
		}
		if n > 0 {
			out = append(out, suggestion{Type: rule.Type, Count: n, Rule: rule})
		}
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handlers) ListSmartDecks(c *gin.Context) {
	decks, err := h.Store.ListSmartDecks(c.Request.Context(), auth.UserID(c))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, decks)
}

func (h *Handlers) CreateSmartDeck(c *gin.Context) {
	var body struct {
		Name string          `json:"name"`
		Rule json.RawMessage `json:"rule"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Name) == "" || body.Rule == nil {
		badRequest(c, "name and rule are required")
		return
	}
	// tag::normalize-rule[]
	rule, err := smartrules.Parse(body.Rule)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	normalized, _ := json.Marshal(rule)
	// end::normalize-rule[]
	deck, err := h.Store.CreateSmartDeck(c.Request.Context(), auth.UserID(c), strings.TrimSpace(body.Name), normalized)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, deck)
}

func (h *Handlers) DeleteSmartDeck(c *gin.Context) {
	id, ok := uuidFromPath(c, "id")
	if !ok {
		return
	}
	if err := h.Store.DeleteSmartDeck(c.Request.Context(), auth.UserID(c), id); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
