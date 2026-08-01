package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/model"
)

// clientLocation은 tz 쿼리 파라미터를 model.Location으로 푼다. HTML 화면이 tz
// 쿠키에 적용하는 규칙과 같아서 두 입구가 방문자의 '오늘'을 같게 본다.
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
