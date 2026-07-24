package handlers

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/smartrules"
	"github.com/benelog/flashcard/internal/store"
)

// CreateSession starts a study session and returns its card queue.
func (h *Handlers) CreateSession(c *gin.Context) {
	var body struct {
		Mode      string          `json:"mode"`
		Direction string          `json:"direction"`
		DeckID    *uuid.UUID      `json:"deckId"`
		Rule      json.RawMessage `json:"rule"`
		DueBefore *time.Time      `json:"dueBefore"`
		Limit     int             `json:"limit"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, "invalid body")
		return
	}
	if body.Limit <= 0 || body.Limit > 200 {
		body.Limit = 50
	}
	if body.Direction == "" {
		body.Direction = "text_to_meaning"
	}
	if body.Direction != "text_to_meaning" && body.Direction != "meaning_to_text" {
		badRequest(c, "direction must be text_to_meaning or meaning_to_text")
		return
	}
	userID := auth.UserID(c)
	ctx := c.Request.Context()

	var cards []store.Card
	var ruleJSON json.RawMessage
	var err error

	switch body.Mode {
	case "deck":
		if body.DeckID == nil {
			badRequest(c, "deckId is required for deck mode")
			return
		}
		cards, err = h.Store.ListCards(ctx, userID, *body.DeckID)
		if err == nil {
			rand.Shuffle(len(cards), func(i, j int) { cards[i], cards[j] = cards[j], cards[i] })
		}
	case "due":
		dueBefore := time.Now()
		if body.DueBefore != nil {
			dueBefore = *body.DueBefore
		}
		cards, err = h.Store.DueCards(ctx, userID, dueBefore, body.Limit)
	case "smart":
		if body.Rule == nil {
			badRequest(c, "rule is required for smart mode")
			return
		}
		rule, perr := smartrules.Parse(body.Rule)
		if perr != nil {
			badRequest(c, perr.Error())
			return
		}
		ruleJSON, _ = json.Marshal(rule)
		cards, err = h.Store.CardsByRule(ctx, userID, rule)
	default:
		badRequest(c, "mode must be deck, due or smart")
		return
	}
	if err != nil {
		fail(c, err)
		return
	}

	sess, err := h.Store.CreateSession(ctx, userID, body.Mode, body.Direction, body.DeckID, ruleJSON, len(cards))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"session": sess, "cards": cards})
}

func (h *Handlers) RecordReview(c *gin.Context) {
	sessionID, ok := uuidFromPath(c, "id")
	if !ok {
		return
	}
	var body struct {
		CardID  uuid.UUID `json:"cardId"`
		Result  *bool     `json:"result"`
		IsRetry bool      `json:"isRetry"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Result == nil || body.CardID == uuid.Nil {
		badRequest(c, "cardId and result are required")
		return
	}
	out, err := h.Store.RecordReview(c.Request.Context(), auth.UserID(c), sessionID, body.CardID, *body.Result, body.IsRetry)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *Handlers) FinishSession(c *gin.Context) {
	sessionID, ok := uuidFromPath(c, "id")
	if !ok {
		return
	}
	var body struct {
		Completed bool `json:"completed"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, "invalid body")
		return
	}
	if err := h.Store.FinishSession(c.Request.Context(), auth.UserID(c), sessionID, body.Completed); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handlers) DueCount(c *gin.Context) {
	dueBefore := time.Now()
	if raw := c.Query("dueBefore"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			badRequest(c, "dueBefore must be RFC3339")
			return
		}
		dueBefore = t
	}
	n, err := h.Store.DueCount(c.Request.Context(), auth.UserID(c), dueBefore)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": n})
}
