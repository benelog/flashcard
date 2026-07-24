// Package web renders the HTML pages of Flashcard. It sits on the same
// model.Store the JSON API uses; templates live in templates/ and are
// embedded into the binary, so one Go binary (or one Vercel function) serves
// pages, fragments and static assets alike.
//
// 화면 하나가 파일 하나다(home.go, decks.go, cards.go, …). 그 화면들이 공통으로
// 쓰는 것은 셋으로 나눠 두었다.
//
//	web.go     이 파일. 바이너리에 싣는 자산, Web 타입, 요청에서 값을 꺼내는 헬퍼.
//	render.go  템플릿 파싱과 화면 그리기(페이지 한 장, 조각 하나, 오류 화면).
//	routes.go  어떤 주소가 어느 화면으로 가는지, 그리고 정적 자원 서빙.
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
	// partials holds the shared fragments (study body, form fields, …) that
	// htmx endpoints render standalone.
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

// isHTMX reports whether htmx sent this request. htmx로 온 요청에는 페이지
// 한 장이 아니라 바꿔 끼울 조각만 돌려준다.
func isHTMX(c *gin.Context) bool {
	return c.GetHeader("HX-Request") != ""
}

// deckIDFromPath resolves the :slug path parameter to the visitor's deck id.
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

// uuidFromPath reads a UUID path parameter, answering 404 with notFound when it
// is malformed. 주소를 손으로 고쳐 넣은 경우라 "없는 것"과 구별할 이유가 없다.
func (w *Web) uuidFromPath(c *gin.Context, name, notFound string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		w.renderError(c, http.StatusNotFound, notFound)
		return uuid.Nil, false
	}
	return id, true
}

// clientTZ returns the visitor's IANA timezone, reported by app.js in a
// cookie. Before the first page load (or with JS off) it falls back to UTC.
func clientTZ(c *gin.Context) (string, *time.Location) {
	return model.Location(cookieValue(c, tzCookie))
}

// endOfToday bounds the due-card queue: the last moment of now's day in the
// visitor's timezone. now를 인자로 받아 날짜 경계 계산을 시계 없이 검증한다.
func endOfToday(now time.Time, loc *time.Location) time.Time {
	now = now.In(loc)
	return time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, loc)
}

// postFormValues parses the request body and hands back its form values, so
// the field-by-field readers next to each handler stay pure functions over
// url.Values. multipart와 urlencoded 모두 지원한다(ErrNotMultipart는 정상 경로).
func postFormValues(c *gin.Context) url.Values {
	_ = c.Request.ParseMultipartForm(32 << 20)
	return c.Request.PostForm
}
