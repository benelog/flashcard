// 화면을 그리는 일: 템플릿 파싱, 모든 페이지가 공유하는 view, 그리고 페이지
// 한 장·조각 하나·오류 화면을 써 내는 함수들.
package web

import (
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/auth"
)

// tag::parse-templates[]
func (w *Web) parseTemplates() {
	base := template.Must(template.New("").Funcs(funcMap).
		ParseFS(templateFS, "templates/layout.html", "templates/partials/*.html"))
	w.partials = base

	pageFiles, err := fs.Glob(templateFS, "templates/pages/*.html")
	if err != nil {
		panic(err)
	}
	w.pages = make(map[string]*template.Template, len(pageFiles))
	for _, f := range pageFiles {
		name := strings.TrimSuffix(f[strings.LastIndex(f, "/")+1:], ".html")
		w.pages[name] = template.Must(template.Must(base.Clone()).ParseFS(templateFS, f))
	}
}

// end::parse-templates[]

// view is the root object every template executes against.
type view struct {
	Title     string
	Path      string // request path, for the active tab in the bottom nav
	LoggedIn  bool
	LocalMode bool
	Email     string
	Flash     string
	FlashKind string
	Data      any
}

func (w *Web) newView(c *gin.Context, title string, data any) view {
	kind, msg := takeFlash(c)
	return view{
		Title:     title,
		Path:      c.Request.URL.Path,
		LoggedIn:  auth.OptionalUserID(c) != uuid.Nil,
		LocalMode: w.cfg.AuthMode == "local",
		Email:     userEmail(c),
		Flash:     msg,
		FlashKind: kind,
		Data:      data,
	}
}

// tag::render[]
func (w *Web) render(c *gin.Context, status int, page, title string, data any) {
	tpl, ok := w.pages[page]
	if !ok {
		panic("unknown page template: " + page)
	}
	c.Status(status)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Cache-Control", "no-store") // pages are per-user; never share
	if err := tpl.ExecuteTemplate(c.Writer, "layout", w.newView(c, title, data)); err != nil {
		_ = c.Error(err)
	}
}

// end::render[]

// renderPartial writes a single fragment — the response to an htmx request.
func (w *Web) renderPartial(c *gin.Context, name string, data any) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := w.partials.ExecuteTemplate(c.Writer, name, data); err != nil {
		_ = c.Error(err)
	}
}

func (w *Web) renderError(c *gin.Context, status int, message string) {
	w.render(c, status, "error", "문제가 생겼어요", message)
}

// failPage is the page-handler counterpart of the API's fail(): 404 for
// missing rows, 500 otherwise.
func (w *Web) failPage(c *gin.Context, err error) {
	if isNotFound(err) {
		w.renderError(c, http.StatusNotFound, "찾을 수 없는 페이지예요.")
		return
	}
	// 원문은 로그에만 남긴다. 방문자에게 보이는 화면은 아래 한 문장뿐이다.
	// internal/handlers의 fail()과 같은 처리로, 기록도 같은 곳으로 나간다.
	log.Printf("internal error: %v", err)
	w.renderError(c, http.StatusInternalServerError, "일시적인 오류가 발생했어요. 잠시 후 다시 시도해주세요.")
}
