package web

import (
	"net/http"
	"strconv"
	"time"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/model"
	"github.com/gin-gonic/gin"
)

// 학습 통계 화면: 요약 수치, 1년치 잔디(학습한 날), 일별 막대 차트.

// chartDays는 통계 막대 차트가 보여 주는 날 수다.
const chartDays = 30

type chartDay struct {
	Date       string
	Total      int
	CorrectPct int // 차트 막대 높이(%), 최대값 기준
	WrongPct   int
	Title      string
}

// buildChart는 오늘로 끝나는 최근 chartDays일을 늘어놓는다.
//
// 공부하지 않은 날은 DB에 행이 없다. 차트에 그 날을 빈칸으로 남기려면 날짜를
// 하루씩 세어 채워야 한다. 막대 높이는 여기서 %로 계산해 템플릿은 그리기만 한다.
// daily에 그보다 오래된 날이 섞여 있어도 막대의 기준은 차트에 보이는 날에서만 고른다.
func buildChart(daily []model.DailyStat, today time.Time) []chartDay {
	statOf := statsByDate(daily)
	busiestDay := 1 // 가장 많이 푼 날이 막대 100%의 기준이다 (0으로 나누지 않도록 1부터)
	for daysAgo := range chartDays {
		if total := statOf[today.AddDate(0, 0, -daysAgo).Format(time.DateOnly)].Total; total > busiestDay {
			busiestDay = total
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

func statsByDate(daily []model.DailyStat) map[string]model.DailyStat {
	statOf := make(map[string]model.DailyStat, len(daily))
	for _, stat := range daily {
		statOf[stat.Date] = stat
	}
	return statOf
}

// heatmapWeeks는 잔디가 보여 주는 주 수다. 이번 주까지 합쳐 53주라 1년이 다 들어간다.
const heatmapWeeks = 53

type heatmapCell struct {
	Level  int    // 0(안 함)~4(가장 많이). CSS 클래스 heat-0~heat-4로 그린다.
	Future bool   // 이번 주의 남은 날. 자리만 차지하고 그리지 않는다.
	Title  string // 마우스를 올리면 뜨는 설명
}

type heatmapWeek struct {
	Month string // 이 주에 어느 달의 1일이 들어 있으면 그 달 이름, 아니면 빈칸
	Days  [7]heatmapCell
}

type heatmap struct {
	Weeks       []heatmapWeek
	StudiedDays int  // 기간 안에 하루라도 공부한 날 수
	Longest     int  // 기간 안의 최장 연속 학습일
	TodayDone   bool // 오늘 이미 공부했는지(스트릭을 이어 가라는 안내에 쓴다)
}

// heatmapStart는 잔디의 첫 칸, 즉 heatmapWeeks-1주 전 일요일이다. 한 주를
// 일요일부터 세므로 오늘이 무슨 요일이든 맨 오른쪽 열이 이번 주가 된다.
func heatmapStart(today time.Time) time.Time {
	day := noon(today)
	return day.AddDate(0, 0, -int(day.Weekday())-(heatmapWeeks-1)*7)
}

// noon은 날짜만 남기고 시각을 정오에 고정한다. 자정에서 날을 더하고 빼면
// 서머타임이 바뀌는 날 없는 시각이 되어 날짜가 밀릴 수 있다(model.Streak과 같은 이유).
func noon(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 12, 0, 0, 0, t.Location())
}

// heatmapDays는 잔디를 채우려면 오늘부터 며칠을 거슬러 읽어야 하는지다.
func heatmapDays(today time.Time) int {
	return int(noon(today).Sub(heatmapStart(today)).Hours()/24+0.5) + 1
}

// buildHeatmap은 GitHub 기여 그래프처럼 열은 주, 행은 요일(일~토)인 격자를 만든다.
//
// 칸의 진하기는 기간 안에서 가장 많이 푼 날을 기준으로 네 단계로 나눈다. 한 장만
// 풀어도 1단계라서, 하루를 빠뜨리지 않았다는 사실은 양과 상관없이 눈에 띈다.
func buildHeatmap(daily []model.DailyStat, today time.Time) heatmap {
	statOf := statsByDate(daily)
	start := heatmapStart(today)
	todayKey := noon(today).Format(time.DateOnly)

	busiestDay := 1
	for _, stat := range daily {
		if stat.Date >= start.Format(time.DateOnly) && stat.Total > busiestDay {
			busiestDay = stat.Total
		}
	}

	h := heatmap{Weeks: make([]heatmapWeek, heatmapWeeks)}
	run := 0
	for w := range h.Weeks {
		for d := range 7 {
			day := start.AddDate(0, 0, w*7+d)
			key := day.Format(time.DateOnly)
			if day.Day() == 1 {
				h.Weeks[w].Month = strconv.Itoa(int(day.Month())) + "월"
			}
			if key > todayKey {
				h.Weeks[w].Days[d] = heatmapCell{Future: true}
				continue
			}
			total := statOf[key].Total
			cell := heatmapCell{Title: key + " (" + weekdayNames[d] + "): "}
			if total == 0 {
				cell.Title += "학습 없음"
				run = 0
			} else {
				cell.Level = (total*4 + busiestDay - 1) / busiestDay // 1~4로 올림
				cell.Title += strconv.Itoa(total) + "회"
				h.StudiedDays++
				run++
				h.Longest = max(h.Longest, run)
			}
			h.Weeks[w].Days[d] = cell
			if key == todayKey {
				h.TodayDone = total > 0
			}
		}
	}
	return h
}

// weekdayNames는 time.Weekday 순서(일요일이 0)를 따른다.
var weekdayNames = [7]string{"일", "월", "화", "수", "목", "금", "토"}

// accuracy는 전체 기간 정답률(%)이다. 아직 한 번도 풀지 않았으면 정답률이라는
// 것이 없으므로, 템플릿이 0%와 구별하도록 -1로 알린다.
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

	// 잔디가 막대 차트보다 기간이 길어 한 번 읽은 것을 둘이 나눠 쓴다.
	today := time.Now().In(loc)
	daily, err := w.store.DailyStats(ctx, userID, tz, max(chartDays, heatmapDays(today)))
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
		"Days":     buildChart(daily, today),
		"Heatmap":  buildHeatmap(daily, today),
	})
}
