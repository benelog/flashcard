package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/model"
	"github.com/benelog/flashcard/internal/study"
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
	if body.Limit <= 0 || body.Limit > study.MaxDailyGoal {
		body.Limit = study.DefaultDailyGoal
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
	dueBefore := time.Now()
	if body.DueBefore != nil {
		dueBefore = *body.DueBefore
	}

	userID := auth.UserID(c)
	ctx := c.Request.Context()

	// 어느 카드를 낼지는 internal/study가 정한다. 화면 쪽 학습도 같은 함수를
	// 부르므로 두 입구가 같은 카드를 같은 순서로 낸다.
	plan, err := study.Pick(ctx, h.Store, userID, study.Request{
		Mode:      body.Mode,
		DeckID:    body.DeckID,
		Rule:      body.Rule,
		DueBefore: dueBefore,
		Limit:     body.Limit,
	})
	if err != nil {
		failStudyRequest(c, err)
		return
	}

	sess, err := h.Store.CreateSession(ctx, userID, plan.Mode, body.Direction, plan.DeckID, plan.Rule, len(plan.Cards))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"session": sess, "cards": plan.Cards})
}

// failStudyRequest turns a study.Pick failure into this API's answer: a bad ask
// is 400 with what was wrong, anything else is the usual 404/500.
func failStudyRequest(c *gin.Context, err error) {
	switch {
	case errors.Is(err, study.ErrUnknownMode):
		badRequest(c, "mode must be deck, due or smart")
	case errors.Is(err, study.ErrDeckRequired):
		badRequest(c, "deckId is required for deck mode")
	case errors.Is(err, study.ErrRuleRequired):
		badRequest(c, "rule is required for smart mode")
	case errors.Is(err, study.ErrInvalidRule):
		badRequest(c, err.Error())
	default:
		fail(c, err)
	}
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
