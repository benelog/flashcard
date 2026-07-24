package web

import (
	"net/http"
	"strconv"
	"time"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/model"
	"github.com/gin-gonic/gin"
)

// 학습 통계 화면: 일별 막대 차트와 요약 수치.

// chartDays is how many days the stats bar chart shows.
const chartDays = 30

type chartDay struct {
	Date       string
	Total      int
	CorrectPct int // 차트 막대 높이(%), 최대값 기준
	WrongPct   int
	Title      string
}

// buildChart lays out the last chartDays days ending on today.
//
// 공부하지 않은 날은 DB에 행이 없다. 차트에 그 날을 빈칸으로 남기려면 날짜를
// 하루씩 세어 채워야 한다. 막대 높이는 여기서 %로 계산해 템플릿은 그리기만 한다.
func buildChart(daily []model.DailyStat, today time.Time) []chartDay {
	statOf := make(map[string]model.DailyStat, len(daily))
	busiestDay := 1 // 가장 많이 푼 날이 막대 100%의 기준이다 (0으로 나누지 않도록 1부터)
	for _, stat := range daily {
		statOf[stat.Date] = stat
		if stat.Total > busiestDay {
			busiestDay = stat.Total
		}
	}
	days := make([]chartDay, 0, chartDays)
	for daysAgo := chartDays - 1; daysAgo >= 0; daysAgo-- {
		date := today.AddDate(0, 0, -daysAgo).Format(time.DateOnly)
		stat := statOf[date]
		days = append(days, chartDay{
			Date:       date,
			Total:      stat.Total,
			CorrectPct: percent(stat.Correct, busiestDay),
			WrongPct:   percent(stat.Total-stat.Correct, busiestDay),
			Title:      date + ": " + strconv.Itoa(stat.Total) + "회 (정답 " + strconv.Itoa(stat.Correct) + ")",
		})
	}
	return days
}

// accuracy is the all-time correct percentage. 아직 한 번도 풀지 않았으면
// 정답률이라는 것이 없으므로, 템플릿이 0%와 구별하도록 -1로 알린다.
func accuracy(summary model.Summary) int {
	if summary.TotalReviews == 0 {
		return -1
	}
	return percent(summary.CorrectReviews, summary.TotalReviews)
}

func (w *Web) statsPage(c *gin.Context) {
	userID := auth.UserID(c)
	ctx := c.Request.Context()
	tz, loc := clientTZ(c)

	daily, err := w.store.DailyStats(ctx, userID, tz, chartDays)
	if err != nil {
		w.failPage(c, err)
		return
	}
	summary, err := w.store.StatsSummary(ctx, userID, tz, loc)
	if err != nil {
		w.failPage(c, err)
		return
	}

	w.render(c, http.StatusOK, "stats", "통계", gin.H{
		"Summary":  summary,
		"Accuracy": accuracy(summary),
		"Days":     buildChart(daily, time.Now().In(loc)),
	})
}
