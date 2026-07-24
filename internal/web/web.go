// Package web은 Flashcard의 HTML 화면을 그린다. JSON API와 같은 model.Store
// 위에 서고, 템플릿과 정적 자원은 전부 바이너리에 embed되므로 바이너리 하나
// (또는 Vercel 함수 하나)가 페이지·조각·정적 자원을 다 낸다.
//
// 화면 하나가 파일 하나다(home.go, decks.go, cards.go, …). 화면들이 공통으로
// 쓰는 것은 역할별 일곱 파일로 나뉜다.
//
//	web.go      이 파일. embed 자산, Web 타입, 요청에서 값을 꺼내는 헬퍼.
//	render.go   템플릿 파싱과 그리기(페이지, 조각, 오류 화면).
//	routes.go   라우트 등록과 정적 자원 서빙.
//	funcs.go    템플릿 함수(funcMap)와 화면에 붙는 이름표.
//	cookies.go  쿠키 전부: 로그인 세션, 학습 방향, 플래시, 시간대.
//	gotrue.go   Supabase Auth REST 클라이언트.
//	icons.go    인라인 SVG 아이콘.
package web

import (
	"embed"
	"html/template"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/config"
	"github.com/benelog/flashcard/internal/model"
)

//go:embed templates
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

type Web struct {
	store  model.Store
	cfg    *config.Config
	goTrue *goTrue
	pages  map[string]*template.Template
	// partials는 htmx 끝점이 단독으로 그리는 공용 조각(학습 본문, 폼 필드 …)
	// 이다.
	partials *template.Template
}

func New(cfg *config.Config, s model.Store) *Web {
	w := &Web{store: s, cfg: cfg}
	if cfg.AuthMode != "local" {
		w.goTrue = newGoTrue(cfg.SupabaseURL, cfg.SupabaseAnonKey)
	}
	w.parseTemplates()
	return w
}

// isHTMX는 이 요청을 htmx가 보냈는지 알린다. htmx로 온 요청에는 페이지
// 한 장이 아니라 바꿔 끼울 조각만 돌려준다.
func isHTMX(c *gin.Context) bool {
	return c.GetHeader("HX-Request") != ""
}

// deckIDFromPath는 :slug 경로 매개변수를 방문자의 덱 id로 푼다.
// 남의 덱이나 없는 덱은 여기서 404로 끝나므로, 부르는 쪽은 소유권을 다시 확인하지
// 않아도 된다. false를 받으면 응답은 이미 쓰인 상태다.
func (w *Web) deckIDFromPath(c *gin.Context) (uuid.UUID, bool) {
	deckID, err := w.store.DeckIDBySlug(c.Request.Context(), auth.UserID(c), c.Param("slug"))
	if err != nil {
		w.failPage(c, err)
		return uuid.Nil, false
	}
	return deckID, true
}

// uuidFromPath는 UUID 경로 매개변수를 읽고, 형식이 틀리면 notFound 문구로
// 404를 답한다. 주소를 손으로 고쳐 넣은 경우라 "없는 것"과 구별할 이유가 없다.
func (w *Web) uuidFromPath(c *gin.Context, name, notFound string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		w.renderError(c, http.StatusNotFound, notFound)
		return uuid.Nil, false
	}
	return id, true
}

// clientTZ는 app.js가 쿠키로 알려 준 방문자의 IANA 시간대를 돌려준다.
// 첫 페이지를 열기 전이거나 JS가 꺼져 있으면 UTC다.
func clientTZ(c *gin.Context) (string, *time.Location) {
	return model.Location(cookieValue(c, tzCookie))
}

// endOfToday는 복습 큐의 만기 상한으로, 방문자 시간대에서 now가 속한 날의
// 마지막 순간이다. now를 인자로 받아 날짜 경계 계산을 시계 없이 검증한다.
func endOfToday(now time.Time, loc *time.Location) time.Time {
	now = now.In(loc)
	return time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, loc)
}

// postFormValues는 요청 본문을 해석해 폼 값을 돌려준다. 핸들러 옆의 필드
// 해석 함수들이 url.Values만 보는 순수 함수로 남게 하기 위해서다.
// multipart와 urlencoded 모두 지원한다(ErrNotMultipart는 정상 경로).
func postFormValues(c *gin.Context) url.Values {
	_ = c.Request.ParseMultipartForm(32 << 20)
	return c.Request.PostForm
}
