package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/model"
)

// clientLocation reads the tz query param, resolved by model.Location — the
// same rule the HTML pages apply to their tz cookie, so the two entrances
// agree on the visitor's "today".
func clientLocation(c *gin.Context) (string, *time.Location) {
	return model.Location(c.Query("tz"))
}

func (h *Handlers) DailyStats(c *gin.Context) {
	days, err := strconv.Atoi(c.DefaultQuery("days", "30"))
	if err != nil || days < 1 || days > 365 {
		days = 30
	}
	tz, _ := clientLocation(c)
	stats, err := h.Store.DailyStats(c.Request.Context(), auth.UserID(c), tz, days)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (h *Handlers) StatsSummary(c *gin.Context) {
	tz, loc := clientLocation(c)
	sum, err := h.Store.StatsSummary(c.Request.Context(), auth.UserID(c), tz, loc)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, sum)
}
