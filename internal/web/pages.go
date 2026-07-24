package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/smartrules"
	"github.com/benelog/flashcard/internal/store"
)

// profileSettings mirrors the JSON blob stored in profiles.settings.
type profileSettings struct {
	TtsRate   float64 `json:"ttsRate,omitempty"`
	DailyGoal int     `json:"dailyGoal,omitempty"`
}

// 설정을 저장한 적 없는 사용자, 그리고 저장된 값이 범위를 벗어난 경우의 값이다.
const (
	defaultTtsRate   = 0.9
	defaultDailyGoal = 50
)

func defaultSettings() profileSettings {
	return profileSettings{TtsRate: defaultTtsRate, DailyGoal: defaultDailyGoal}
}

// settingsFrom reads the profile's settings blob, falling back to the defaults
// for anything missing or out of range. 설정 JSON은 사용자가 바꿔 온 것이 아니라
// 우리가 쓴 것이지만, 예전 버전이 쓴 값이 남아 있을 수 있어 늘 검사한다.
func settingsFrom(profile store.Profile) profileSettings {
	settings := defaultSettings()
	_ = json.Unmarshal(profile.Settings, &settings)
	if settings.TtsRate <= 0 {
		settings.TtsRate = defaultTtsRate
	}
	if settings.DailyGoal <= 0 {
		settings.DailyGoal = defaultDailyGoal
	}
	return settings
}

// ---------- 홈 ----------

type suggestionView struct {
	Title string
	Count int
	// Rule is the raw rule JSON; the template's URL context escapes it into
	// the /study?rule= link.
	Rule string
}

func (w *Web) homePage(c *gin.Context) {
	userID := auth.UserID(c)
	ctx := c.Request.Context()
	tz, loc := clientTZ(c)

	due, err := w.store.DueCount(ctx, userID, endOfToday(loc))
	if err != nil {
		w.failPage(c, err)
		return
	}
	summary, err := w.store.StatsSummary(ctx, userID, tz, loc)
	if err != nil {
		w.failPage(c, err)
		return
	}
	decks, err := w.store.ListDecks(ctx, userID)
	if err != nil {
		w.failPage(c, err)
		return
	}
	if len(decks) > 3 {
		decks = decks[:3]
	}

	// 홈 화면 추천 타일: 지금 카드가 있는 고정 규칙만 노출한다.
	canned := []smartrules.Rule{
		{Type: smartrules.HighError, MinAttempts: 3, MinErrorRate: 0.4, Limit: 20},
		{Type: smartrules.Stale, NotReviewedDays: 7, Limit: 20},
	}
	var suggestions []suggestionView
	for _, rule := range canned {
		n, err := w.store.CountByRule(ctx, userID, rule)
		if err != nil {
			w.failPage(c, err)
			return
		}
		if n > 0 {
			raw, _ := json.Marshal(rule)
			suggestions = append(suggestions, suggestionView{
				Title: suggestionTitle(rule, n),
				Count: n,
				Rule:  string(raw),
			})
		}
	}

	w.render(c, http.StatusOK, "home", "Flashcard", gin.H{
		"Due":         due,
		"Streak":      summary.Streak,
		"Decks":       decks,
		"Suggestions": suggestions,
	})
}

// ---------- 덱 목록 ----------

func (w *Web) decksPage(c *gin.Context) {
	userID := auth.UserID(c)
	ctx := c.Request.Context()
	decks, err := w.store.ListDecks(ctx, userID)
	if err != nil {
		w.failPage(c, err)
		return
	}
	smartDecks, err := w.store.ListSmartDecks(ctx, userID)
	if err != nil {
		w.failPage(c, err)
		return
	}
	w.render(c, http.StatusOK, "decks", "덱", gin.H{
		"Decks":      decks,
		"SmartDecks": smartDecks,
	})
}

func (w *Web) createDeck(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		redirectWithFlash(c, flashError, "덱 이름을 입력해주세요", "/decks")
		return
	}
	deck, err := w.store.CreateDeck(c.Request.Context(), auth.UserID(c), name, nil)
	if err != nil {
		w.failPage(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/decks/"+deck.Slug)
}

// ---------- 덱 상세 ----------

func (w *Web) deckPage(c *gin.Context) {
	userID := auth.UserID(c)
	ctx := c.Request.Context()
	deck, err := w.store.GetDeckBySlug(ctx, userID, c.Param("slug"))
	if err != nil {
		w.failPage(c, err)
		return
	}
	cards, err := w.store.ListCards(ctx, userID, deck.ID)
	if err != nil {
		w.failPage(c, err)
		return
	}
	w.render(c, http.StatusOK, "deck", deck.Name, gin.H{
		"Deck":     deck,
		"Cards":    cards,
		"ShareURL": w.shareURL(c, deck),
	})
}

func (w *Web) shareURL(c *gin.Context, deck store.Deck) string {
	if deck.ShareSlug == nil {
		return ""
	}
	return origin(c) + "/shared/" + *deck.ShareSlug
}

func (w *Web) deleteDeck(c *gin.Context) {
	deckID, ok := w.deckIDFromPath(c)
	if !ok {
		return
	}
	if err := w.store.DeleteDeck(c.Request.Context(), auth.UserID(c), deckID); err != nil {
		w.failPage(c, err)
		return
	}
	setFlash(c, flashInfo, "덱을 삭제했어요")
	// 삭제한 덱의 화면에 머물 수 없으므로 목록으로 보낸다. htmx 요청은 조각만
	// 갈아 끼우므로 이동을 HX-Redirect 헤더로 지시해야 한다.
	if isHTMX(c) {
		c.Header("HX-Redirect", "/decks")
		c.Status(http.StatusOK)
		return
	}
	c.Redirect(http.StatusSeeOther, "/decks")
}

// ---------- 공유 ----------

func (w *Web) shareDeck(c *gin.Context)   { w.setDeckShared(c, true) }
func (w *Web) unshareDeck(c *gin.Context) { w.setDeckShared(c, false) }

// setDeckShared turns the deck's public link on or off and returns to the deck
// page either way.
func (w *Web) setDeckShared(c *gin.Context, shared bool) {
	deckID, ok := w.deckIDFromPath(c)
	if !ok {
		return
	}
	userID := auth.UserID(c)
	ctx := c.Request.Context()
	var err error
	message := "공유를 해제했어요"
	if shared {
		_, err = w.store.ShareDeck(ctx, userID, deckID)
		message = "덱을 공유했어요"
	} else {
		err = w.store.UnshareDeck(ctx, userID, deckID)
	}
	if err != nil {
		w.failPage(c, err)
		return
	}
	redirectWithFlash(c, flashInfo, message, "/decks/"+c.Param("slug"))
}

func (w *Web) sharedGalleryPage(c *gin.Context) {
	decks, err := w.store.ListSharedDecks(c.Request.Context(), auth.OptionalUserID(c))
	if err != nil {
		w.failPage(c, err)
		return
	}
	w.render(c, http.StatusOK, "shared", "공유 덱 둘러보기", gin.H{"Decks": decks})
}

func (w *Web) sharedDeckPage(c *gin.Context) {
	slug := c.Param("slug")
	ctx := c.Request.Context()
	deck, err := w.store.GetSharedDeck(ctx, auth.OptionalUserID(c), slug)
	if err != nil {
		if isNotFound(err) {
			w.renderError(c, http.StatusNotFound, "공유가 해제되었거나 존재하지 않는 덱이에요.")
			return
		}
		w.failPage(c, err)
		return
	}
	cards, err := w.store.GetSharedDeckCards(ctx, slug)
	if err != nil {
		w.failPage(c, err)
		return
	}
	w.render(c, http.StatusOK, "shared_deck", deck.Name, gin.H{
		"Slug":  slug,
		"Deck":  deck,
		"Cards": cards,
	})
}

func (w *Web) importSharedDeck(c *gin.Context) {
	deck, err := w.store.ImportSharedDeck(c.Request.Context(), auth.UserID(c), c.Param("slug"))
	if err != nil {
		w.failPage(c, err)
		return
	}
	redirectWithFlash(c, flashInfo, "'"+deck.Name+"' 덱을 가져왔어요", "/decks/"+deck.Slug)
}

// ---------- 카드 편집 ----------

// cardFormView carries the (re)rendered card form: current values plus the
// submit target, shared by the new-card page, the edit page and the
// dictionary-lookup fragment.
type cardFormView struct {
	Action   string
	BackURL  string
	Editing  bool
	Text     string
	Meaning  string
	CardType string
	Tags     string
	Phonetic string
	Example  string
	Notes    string
	Status   string // 사전 조회 결과 메시지
}

func (w *Web) newCardPage(c *gin.Context) {
	// 존재하지 않는 덱이면 빈 폼을 보여 주기 전에 404를 낸다.
	if _, ok := w.deckIDFromPath(c); !ok {
		return
	}
	slug := c.Param("slug")
	w.render(c, http.StatusOK, "card_form", "새 카드", cardFormView{
		Action:   "/decks/" + slug + "/cards",
		BackURL:  "/decks/" + slug,
		CardType: store.DefaultCardType,
	})
}

func (w *Web) editCardPage(c *gin.Context) {
	cardID, ok := w.uuidFromPath(c, "id", "찾을 수 없는 카드예요.")
	if !ok {
		return
	}
	card, err := w.store.GetCard(c.Request.Context(), auth.UserID(c), cardID)
	if err != nil {
		w.failPage(c, err)
		return
	}
	w.render(c, http.StatusOK, "card_form", "카드 수정", cardFormView{
		Action:   "/cards/" + card.ID.String(),
		BackURL:  "/decks/" + card.DeckSlug,
		Editing:  true,
		Text:     card.Text,
		Meaning:  card.Meaning,
		CardType: card.CardType,
		Tags:     strings.Join(card.Tags, ", "),
		Phonetic: orEmpty(card.Phonetic),
		Example:  orEmpty(card.Example),
		Notes:    orEmpty(card.Notes),
	})
}

// cardInputFromForm normalizes the posted card fields. 두 번째 반환값은 이 폼을
// 저장할 수 있는지로, 원문과 뜻이 모두 있어야 참이다.
func cardInputFromForm(c *gin.Context) (store.CardInput, bool) {
	in := store.CardInput{
		Text:     strings.TrimSpace(c.PostForm("text")),
		Meaning:  strings.TrimSpace(c.PostForm("meaning")),
		CardType: store.NormalizeCardType(c.PostForm("card_type")),
		Tags:     splitAndTrim(c.PostForm("tags"), ","),
		Phonetic: nilIfBlank(c.PostForm("phonetic")),
		Example:  nilIfBlank(c.PostForm("example")),
		Notes:    nilIfBlank(c.PostForm("notes")),
	}
	return in, in.Text != "" && in.Meaning != ""
}

// nilIfBlank keeps an empty optional field out of the database as NULL rather
// than as an empty string, so "적지 않았다"와 "빈 값을 적었다"가 섞이지 않는다.
func nilIfBlank(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}

// splitAndTrim cuts a separated list, dropping padding and empty items. 태그
// 입력("a, b,")과 학습 큐의 카드 ID 목록이 모두 이 모양이다.
func splitAndTrim(raw, separator string) []string {
	items := []string{}
	for _, item := range strings.Split(raw, separator) {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}

func (w *Web) createCard(c *gin.Context) {
	slug := c.Param("slug")
	in, ok := cardInputFromForm(c)
	if !ok {
		redirectWithFlash(c, flashError, "원문과 뜻을 모두 입력해주세요", "/decks/"+slug+"/cards/new")
		return
	}
	deckID, ok := w.deckIDFromPath(c)
	if !ok {
		return
	}
	in.DeckID = deckID
	if _, err := w.store.CreateCard(c.Request.Context(), auth.UserID(c), in); err != nil {
		w.failPage(c, err)
		return
	}
	redirectWithFlash(c, flashInfo, "카드를 추가했어요", "/decks/"+slug)
}

func (w *Web) updateCard(c *gin.Context) {
	cardID, ok := w.uuidFromPath(c, "id", "찾을 수 없는 카드예요.")
	if !ok {
		return
	}
	in, ok := cardInputFromForm(c)
	if !ok {
		redirectWithFlash(c, flashError, "원문과 뜻을 모두 입력해주세요", "/cards/"+cardID.String())
		return
	}
	card, err := w.store.UpdateCard(c.Request.Context(), auth.UserID(c), cardID, in)
	if err != nil {
		w.failPage(c, err)
		return
	}
	redirectWithFlash(c, flashInfo, "카드를 수정했어요", "/decks/"+card.DeckSlug)
}

func (w *Web) deleteCard(c *gin.Context) {
	cardID, ok := w.uuidFromPath(c, "id", "찾을 수 없는 카드예요.")
	if !ok {
		return
	}
	if err := w.store.DeleteCard(c.Request.Context(), auth.UserID(c), cardID); err != nil {
		w.failPage(c, err)
		return
	}
	// htmx가 목록의 해당 <li>를 지우도록 빈 본문을 돌려준다.
	if isHTMX(c) {
		c.Status(http.StatusOK)
		return
	}
	setFlash(c, flashInfo, "카드를 삭제했어요")
	redirectBack(c, "/decks")
}

// ---------- 스마트 덱 ----------

func (w *Web) saveSmartDeck(c *gin.Context) {
	rule, err := smartrules.Parse([]byte(c.PostForm("rule")))
	if err != nil {
		setFlash(c, flashError, "잘못된 규칙이에요")
		redirectBack(c, "/decks")
		return
	}
	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		name = ruleLabel(rule)
	}
	normalized, _ := json.Marshal(rule)
	if _, err := w.store.CreateSmartDeck(c.Request.Context(), auth.UserID(c), name, normalized); err != nil {
		w.failPage(c, err)
		return
	}
	// 학습 화면의 저장 버튼(htmx)은 배지로 바꿔치기만 한다.
	if isHTMX(c) {
		w.renderPartial(c, "saved_badge", nil)
		return
	}
	setFlash(c, flashInfo, "스마트 덱으로 저장했어요")
	redirectBack(c, "/decks")
}

func (w *Web) deleteSmartDeck(c *gin.Context) {
	smartDeckID, ok := w.uuidFromPath(c, "id", "찾을 수 없는 스마트 덱이에요.")
	if !ok {
		return
	}
	if err := w.store.DeleteSmartDeck(c.Request.Context(), auth.UserID(c), smartDeckID); err != nil {
		w.failPage(c, err)
		return
	}
	// htmx가 목록의 해당 항목을 지우도록 빈 본문을 돌려준다.
	if isHTMX(c) {
		c.Status(http.StatusOK)
		return
	}
	redirectWithFlash(c, flashInfo, "스마트 덱을 삭제했어요", "/decks")
}

// ---------- 통계 ----------

// chartDays is how many days the stats bar chart shows.
const chartDays = 30

type chartDay struct {
	Date       string
	Total      int
	CorrectPct int // 차트 막대 높이(%), 최대값 기준
	WrongPct   int
	Title      string
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

	// 공부하지 않은 날은 DB에 행이 없다. 차트에 그 날을 빈칸으로 남기려면 날짜를
	// 하루씩 세어 채워야 한다. 막대 높이는 서버에서 %로 계산해 템플릿은 그리기만
	// 한다.
	statOf := map[string]store.DailyStat{}
	busiestDay := 1 // 가장 많이 푼 날이 막대 100%의 기준이다 (0으로 나누지 않도록 1부터)
	for _, stat := range daily {
		statOf[stat.Date] = stat
		if stat.Total > busiestDay {
			busiestDay = stat.Total
		}
	}
	days := make([]chartDay, 0, chartDays)
	today := time.Now().In(loc)
	for daysAgo := chartDays - 1; daysAgo >= 0; daysAgo-- {
		date := today.AddDate(0, 0, -daysAgo).Format("2006-01-02")
		stat := statOf[date]
		days = append(days, chartDay{
			Date:       date,
			Total:      stat.Total,
			CorrectPct: percent(stat.Correct, busiestDay),
			WrongPct:   percent(stat.Total-stat.Correct, busiestDay),
			Title:      date + ": " + strconv.Itoa(stat.Total) + "회 (정답 " + strconv.Itoa(stat.Correct) + ")",
		})
	}

	// 아직 한 번도 풀지 않았으면 정답률이라는 것이 없다. 템플릿이 0%와 구별하도록
	// -1로 표시한다.
	accuracy := -1
	if summary.TotalReviews > 0 {
		accuracy = percent(summary.CorrectReviews, summary.TotalReviews)
	}

	w.render(c, http.StatusOK, "stats", "통계", gin.H{
		"Summary":  summary,
		"Accuracy": accuracy,
		"Days":     days,
	})
}

// ---------- 설정 ----------

func (w *Web) settingsPage(c *gin.Context) {
	profile, err := w.store.GetOrCreateProfile(c.Request.Context(), auth.UserID(c), "")
	if err != nil {
		w.failPage(c, err)
		return
	}
	w.render(c, http.StatusOK, "settings", "설정", gin.H{
		"Profile":  profile,
		"Settings": settingsFrom(profile),
	})
}

// 설정값이 머무를 수 있는 범위. 벗어난 값이 오면 기본값을 그대로 둔다.
const (
	minTtsRate, maxTtsRate     = 0.5, 1.5
	minDailyGoal, maxDailyGoal = 5, 200
)

func (w *Web) saveSettings(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("display_name"))
	settings := defaultSettings()
	if rate, err := strconv.ParseFloat(c.PostForm("tts_rate"), 64); err == nil &&
		rate >= minTtsRate && rate <= maxTtsRate {
		settings.TtsRate = rate
	}
	if goal, err := strconv.Atoi(c.PostForm("daily_goal")); err == nil &&
		goal >= minDailyGoal && goal <= maxDailyGoal {
		settings.DailyGoal = goal
	}
	raw, _ := json.Marshal(settings)
	if _, err := w.store.UpdateProfile(c.Request.Context(), auth.UserID(c), &name, raw); err != nil {
		w.failPage(c, err)
		return
	}
	redirectWithFlash(c, flashInfo, "저장했어요", "/settings")
}
