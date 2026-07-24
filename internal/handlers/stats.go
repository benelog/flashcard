package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/benelog/flashcard/internal/auth"
)

// clientLocation validates the tz query param against the IANA database so it
// can be passed into SQL safely; falls back to UTC.
// Location resolves an IANA timezone name the visitor's browser reported.
// 방문자가 어디 있는지는 HTML 화면은 쿠키로, JSON API는 질의 문자열로 알려
// 오는데, 값을 해석하는 규칙은 하나여야 통계의 "오늘"이 두 곳에서 같아진다.
// 비어 있거나 알아볼 수 없는 이름이면 UTC로 떨어진다.
func Location(tz string) (string, *time.Location) {
	if tz == "" {
		return "UTC", time.UTC
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return "UTC", time.UTC
	}
	return tz, loc
}

func clientLocation(c *gin.Context) (string, *time.Location) {
	return Location(c.Query("tz"))
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
