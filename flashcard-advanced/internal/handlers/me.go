package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/benelog/flashcard/internal/auth"
)

func (h *Handlers) GetMe(c *gin.Context) {
	p, err := h.Store.GetOrCreateProfile(c.Request.Context(), auth.UserID(c), "")
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *Handlers) UpdateMe(c *gin.Context) {
	var body struct {
		DisplayName *string         `json:"displayName"`
		Settings    json.RawMessage `json:"settings"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, "invalid body")
		return
	}
	if body.Settings != nil && !json.Valid(body.Settings) {
		badRequest(c, "settings must be valid json")
		return
	}
	p, err := h.Store.UpdateProfile(c.Request.Context(), auth.UserID(c), body.DisplayName, body.Settings)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}
