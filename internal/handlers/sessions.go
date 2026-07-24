package handlers

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/model"
	"github.com/benelog/flashcard/internal/smartrules"
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
		body.Direction = model.DefaultDirection
	}
	// API는 폼·쿠키와 달리 모르는 방향을 조용히 고치지 않는다. 프로그램이 보내는
	// 값이라 오타라면 알려 주는 편이 낫다.
	if !model.IsDirection(body.Direction) {
		badRequest(c, "direction must be "+strings.Join(model.Directions, " or "))
		return
	}
	userID := auth.UserID(c)
	ctx := c.Request.Context()

	var cards []model.Card
	var ruleJSON json.RawMessage
	var err error

	switch body.Mode {
	case model.ModeDeck:
		if body.DeckID == nil {
			badRequest(c, "deckId is required for deck mode")
			return
		}
		cards, err = h.Store.ListCards(ctx, userID, *body.DeckID)
		if err == nil {
			rand.Shuffle(len(cards), func(i, j int) { cards[i], cards[j] = cards[j], cards[i] })
		}
	case model.ModeDue:
		dueBefore := time.Now()
		if body.DueBefore != nil {
			dueBefore = *body.DueBefore
		}
		cards, err = h.Store.DueCards(ctx, userID, dueBefore, body.Limit)
	case model.ModeSmart:
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
