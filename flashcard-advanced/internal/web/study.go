package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/model"
	"github.com/benelog/flashcard/internal/study"
)

// tag::study-state[]
// 학습 세션의 진행 상태는 서버에 저장하지 않는다. 카드 ID 큐·라운드·점수를
// hidden 필드로 폼에 실어 보내고, 채점(POST)마다 서버가 다음 상태를 계산해
// 다음 카드 조각(fragment)을 돌려준다. 서버리스(무상태)와 잘 맞는 구조다.
type studyState struct {
	SessionID        string
	Direction        string // text_to_meaning | meaning_to_text
	Title            string
	ReturnURL        string
	Queue            []string // 이번 라운드에 남은 카드 ID (첫 번째가 현재 카드)
	Missed           []string // 이번 라운드에서 틀린 카드 ID
	Round            int
	RoundCards       int // 이번 라운드 전체 카드 수 (진행률 표시용)
	FirstPassTotal   int // 1라운드 카드 수
	FirstPassCorrect int // 1라운드 정답 수
	TtsRate          float64
}

// end::study-state[]

// studyBodyView는 study_body 조각이 그리는 값이다. 단계(Phase) 중 정확히
// 하나만 그리며, 옛 React 상태 기계를 그대로 옮긴 모양이다.
type studyBodyView struct {
	Phase   string // studying | break | finished | empty
	State   studyState
	Card    *model.Card
	Index   int // 이번 라운드에서 몇 번째 카드인지 (0부터)
	TextTTS string
	BackTTS string
}

func (v studyBodyView) QueueJoined() string  { return strings.Join(v.State.Queue, ",") }
func (v studyBodyView) MissedJoined() string { return strings.Join(v.State.Missed, ",") }

// Accuracy는 완료 화면에 보여 줄 1라운드 정답률이다.
func (v studyBodyView) Accuracy() int {
	return percent(v.State.FirstPassCorrect, v.State.FirstPassTotal)
}

// ProgressPct는 진행 막대를 채운다.
func (v studyBodyView) ProgressPct() int { return percent(v.Index, v.State.RoundCards) }

// studyPage는 세션을 시작한다. ?direction=이 없으면 나머지 질의 문자열을
// 유지한 채 방향 선택 화면부터 그린다.
func (w *Web) studyPage(c *gin.Context) {
	if c.Query("direction") == "" {
		w.directionChooser(c)
		return
	}
	direction := model.NormalizeDirection(c.Query("direction"))
	setCookie(c, dirCookie, direction, dirCookieMaxAge)

	userID := auth.UserID(c)
	ctx := c.Request.Context()
	_, loc := clientTZ(c)

	profile, err := w.store.GetOrCreateProfile(ctx, userID, "")
	if err != nil {
		w.failPage(c, err)
		return
	}
	settings := settingsFrom(profile)

	plan, ok := w.planStudy(c, settings.DailyGoal, loc)
	if !ok {
		return
	}

	sess, err := w.store.CreateSession(ctx, userID, plan.Mode, direction, plan.DeckID, plan.Rule, len(plan.Cards))
	if err != nil {
		w.failPage(c, err)
		return
	}

	state := studyState{
		SessionID:      sess.ID.String(),
		Direction:      direction,
		Title:          plan.Title,
		ReturnURL:      plan.ReturnURL,
		Round:          1,
		RoundCards:     len(plan.Cards),
		FirstPassTotal: len(plan.Cards),
		TtsRate:        settings.TtsRate,
	}
	for _, card := range plan.Cards {
		state.Queue = append(state.Queue, card.ID.String())
	}

	body := w.studyBody(c, state)
	// 스마트 학습이면 "이 조건을 스마트 덱으로 저장" 버튼에 쓸 규칙을 넘긴다.
	saveRule := ""
	if plan.Mode == model.ModeSmart && len(plan.Cards) > 0 && c.Query("saved") == "" {
		saveRule = string(plan.Rule)
	}
	w.render(c, http.StatusOK, "study", plan.Title, gin.H{
		"Body":     body,
		"SaveRule": saveRule,
	})
}

// directionChooser는 세션을 시작하기 전에 학습 방향을 묻는다. 지난번에
// 고른 방향을 쿠키에서 꺼내 먼저 보여 주고, 두 링크 모두 나머지 질의 문자열
// (mode, deckId, rule …)을 그대로 물고 간다.
func (w *Web) directionChooser(c *gin.Context) {
	base := c.Request.URL.Query()
	w.render(c, http.StatusOK, "study_direction", "학습", gin.H{
		"Last":      model.NormalizeDirection(cookieValue(c, dirCookie)),
		"TextFirst": "/study?" + withParam(base, "direction", model.TextToMeaning),
		"TextLast":  "/study?" + withParam(base, "direction", model.MeaningToText),
	})
}

// studyPlan은 한 세션의 카드 목록이다. 고르는 일은 JSON API도 쓰는
// internal/study가 하고, 화면에만 필요한 둘(세션 제목, 끝나면 돌아갈 곳)을
// 얹는다.
type studyPlan struct {
	study.Plan
	Title     string
	ReturnURL string
}

// planStudy는 요청된 모드에 맞는 카드를 고른다. false를 받으면 응답은 이미
// 쓰인 상태이므로 부르는 쪽은 그대로 돌아가면 된다.
func (w *Web) planStudy(c *gin.Context, dailyGoal int, loc *time.Location) (studyPlan, bool) {
	req := study.Request{
		Mode:      c.Query("mode"),
		Rule:      json.RawMessage(c.Query("rule")),
		DueBefore: study.EndOfDay(time.Now(), loc),
		Limit:     dailyGoal,
	}
	if req.Mode == "" {
		req.Mode = model.DefaultMode
	}
	// 주소를 손으로 고쳐 넣어 덱 번호가 깨진 경우는 덱이 없는 것과 구별하지
	// 않는다. 아래에서 study.ErrDeckRequired로 같은 404가 된다.
	if deckID, err := uuid.Parse(c.Query("deckId")); err == nil {
		req.DeckID = &deckID
	}

	picked, err := study.Pick(c.Request.Context(), w.store, auth.UserID(c), req)
	if err != nil {
		w.failStudyRequest(c, err)
		return studyPlan{}, false
	}

	plan := studyPlan{
		Plan:      picked,
		Title:     studyTitle(c.Query("title"), picked.Mode),
		ReturnURL: "/",
	}
	if picked.Mode == model.ModeDeck {
		plan.ReturnURL = "/decks"
	}
	return plan, true
}

// failStudyRequest는 study.Pick의 실패를 화면으로 옮긴다. 잘못 만들어진 학습
// 링크는 전부 404다: 방문자가 고칠 수 있는 것이 없으므로 무엇이 잘못됐는지만
// 알려 준다.
func (w *Web) failStudyRequest(c *gin.Context, err error) {
	switch {
	case errors.Is(err, study.ErrDeckRequired):
		w.renderError(c, http.StatusNotFound, "찾을 수 없는 덱이에요.")
	case errors.Is(err, study.ErrRuleRequired), errors.Is(err, study.ErrInvalidRule):
		w.renderError(c, http.StatusNotFound, "잘못된 학습 규칙이에요.")
	case errors.Is(err, study.ErrUnknownMode):
		w.renderError(c, http.StatusNotFound, "잘못된 학습 모드예요.")
	default:
		w.failPage(c, err)
	}
}

// studyTitle은 학습 화면의 제목을 정한다. 추천 타일이나 스마트 덱에서 온
// 링크는 자기 이름을 실어 보내므로 그것을 쓰고, 없으면 모드에 맞는 기본 제목을
// 붙인다.
func studyTitle(requested, mode string) string {
	if requested != "" {
		return requested
	}
	switch mode {
	case model.ModeDue:
		return "오늘 복습"
	case model.ModeSmart:
		return "스마트 학습"
	}
	return "덱 학습"
}

func withParam(q url.Values, key, value string) string {
	copied := url.Values{}
	for k, v := range q {
		copied[k] = v
	}
	copied.Set(key, value)
	return copied.Encode()
}

// studyBody는 상태의 현재 단계에 맞는 조각을 만들고, 학습 중이면 현재 카드를
// 읽어 온다. 재귀가 아니라 루프인 이유: 카드가 그 사이 삭제된 극단적 경우 큐를
// 한 칸 줄여 다시 도는데, 대량 삭제라면 재귀 깊이가 큐 길이만큼 깊어진다.
func (w *Web) studyBody(c *gin.Context, state studyState) studyBodyView {
	for {
		v := studyBodyView{State: state}
		switch {
		case state.FirstPassTotal == 0:
			v.Phase = "empty"
		case len(state.Queue) > 0:
			v.Phase = "studying"
			v.Index = state.RoundCards - len(state.Queue)
			cardID, err := uuid.Parse(state.Queue[0])
			if err == nil {
				card, cerr := w.store.GetCard(c.Request.Context(), auth.UserID(c), cardID)
				err = cerr
				if cerr == nil {
					v.Card = &card
					v.TextTTS = card.Text
					v.BackTTS = card.Text
					if card.Example != nil {
						v.BackTTS = card.Text + ". " + *card.Example
					}
				}
			}
			if v.Card == nil {
				// 카드가 그 사이 삭제된 극단적 경우: 남은 큐로 계속한다.
				state.Queue = state.Queue[1:]
				continue
			}
		case len(state.Missed) > 0:
			v.Phase = "break"
		default:
			v.Phase = "finished"
		}
		return v
	}
}

// stateFromForm은 이전 조각이 실어 보낸 학습 상태를 되살린다.
func stateFromForm(c *gin.Context) studyState {
	return stateFromValues(postFormValues(c))
}

// stateFromValues는 stateFromForm의 해석 절반이다. 폼 값은 브라우저가
// 실어 오는 것이라 그대로 믿지 않는다: 범위를 벗어난 숫자는 보정하고 돌아갈
// 주소는 safeNext로 거른다. url.Values만 보므로 HTTP 없이 검증할 수 있다.
func stateFromValues(form url.Values) studyState {
	round, _ := strconv.Atoi(form.Get("round"))
	if round < 1 {
		round = 1
	}
	roundCards, _ := strconv.Atoi(form.Get("round_len"))
	firstPassTotal, _ := strconv.Atoi(form.Get("fp_total"))
	firstPassCorrect, _ := strconv.Atoi(form.Get("fp_correct"))
	rate, _ := strconv.ParseFloat(form.Get("tts_rate"), 64)
	if rate <= 0 {
		rate = defaultTtsRate
	}
	return studyState{
		SessionID:        form.Get("session"),
		Direction:        model.NormalizeDirection(form.Get("direction")),
		Title:            form.Get("title"),
		ReturnURL:        safeNext(form.Get("return_url")),
		Queue:            splitAndTrim(form.Get("queue"), ","),
		Missed:           splitAndTrim(form.Get("missed"), ","),
		Round:            round,
		RoundCards:       roundCards,
		FirstPassTotal:   firstPassTotal,
		FirstPassCorrect: firstPassCorrect,
		TtsRate:          rate,
	}
}

// tag::grade-head[]
// gradeCard: 채점 한 번 = 리뷰 기록 + 다음 상태 계산 + 다음 화면 조각 응답.
func (w *Web) gradeCard(c *gin.Context) {
	state := stateFromForm(c)
	correct := c.PostForm("correct") == "true"
	// end::grade-head[]
	if len(state.Queue) == 0 {
		w.renderPartial(c, "study_body", w.studyBody(c, state))
		return
	}

	// tag::grade-record[]
	current := state.Queue[0]
	state.Queue = state.Queue[1:]

	sessionID, err1 := uuid.Parse(state.SessionID)
	cardID, err2 := uuid.Parse(current)
	if err1 == nil && err2 == nil {
		// 채점 기록 실패는 학습 흐름을 끊을 만큼 치명적이지 않다: 이번
		// 판정 하나가 통계에서 빠질 뿐이므로 세션은 계속 진행한다.
		// end::grade-record[]
		if _, err := w.store.RecordReview(c.Request.Context(), auth.UserID(c),
			sessionID, cardID, correct, state.Round > 1); err != nil && !isNotFound(err) {
			_ = c.Error(err)
		}
	}

	// tag::grade-tally[]
	if correct {
		if state.Round == 1 {
			state.FirstPassCorrect++
		}
	} else {
		state.Missed = append(state.Missed, current)
	}

	// 마지막 카드까지 전부 맞혔으면 세션 완료를 기록한다.
	if len(state.Queue) == 0 && len(state.Missed) == 0 && err1 == nil {
		_ = w.store.FinishSession(c.Request.Context(), auth.UserID(c), sessionID, true)
	}

	w.renderPartial(c, "study_body", w.studyBody(c, state))
}

// end::grade-tally[]

// nextRound는 틀린 카드만으로 다시 시작한다.
func (w *Web) nextRound(c *gin.Context) {
	state := stateFromForm(c)
	state.Queue = state.Missed
	state.Missed = nil
	state.Round++
	state.RoundCards = len(state.Queue)
	w.renderPartial(c, "study_body", w.studyBody(c, state))
}

// quitStudy는 세션을 미완료로 기록하고 화면을 떠난다.
func (w *Web) quitStudy(c *gin.Context) {
	state := stateFromForm(c)
	if sessionID, err := uuid.Parse(state.SessionID); err == nil {
		_ = w.store.FinishSession(c.Request.Context(), auth.UserID(c), sessionID, false)
	}
	c.Redirect(http.StatusSeeOther, state.ReturnURL)
}
