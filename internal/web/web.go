// Package web renders the HTML pages of Flashcard. It sits on the same
// handlers.Store the JSON API uses; templates live in templates/ and are
// embedded into the binary, so one Go binary (or one Vercel function) serves
// pages, fragments and static assets alike.
package web

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/config"
	"github.com/benelog/flashcard/internal/handlers"
)

//go:embed templates
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// assetVersion fingerprints the embedded CSS/JS. The templates hang it off
// every asset URL as ?v=…, so a deploy changes the URL itself: the service
// worker (which serves static files stale-while-revalidate) can never pair a
// new page with last version's stylesheet. Filenames stay stable, which keeps
// them readable in the repo and in devtools.
var assetVersion = sync.OnceValue(func() string {
	h := sha256.New()
	for _, name := range []string{"static/app.css", "static/app.js", "static/htmx.min.js"} {
		b, err := staticFS.ReadFile(name)
		if err != nil {
			panic(err)
		}
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
})

// asset is the template func: {{asset "/static/app.css"}}.
func asset(path string) string { return path + "?v=" + assetVersion() }

type Web struct {
	store  handlers.Store
	cfg    *config.Config
	goTrue *goTrue
	pages  map[string]*template.Template
	// partials holds the shared fragments (study body, form fields, …) that
	// htmx endpoints render standalone.
	partials *template.Template
}

func New(cfg *config.Config, s handlers.Store) *Web {
	w := &Web{store: s, cfg: cfg}
	if cfg.AuthMode != "local" {
		w.goTrue = newGoTrue(cfg.SupabaseURL, cfg.SupabaseAnonKey)
	}
	w.parseTemplates()
	return w
}

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
	fmt.Println("web internal error:", err)
	w.renderError(c, http.StatusInternalServerError, "일시적인 오류가 발생했어요. 잠시 후 다시 시도해주세요.")
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

// Register wires every HTML route into the shared Gin engine. The JSON API
// under /api is registered separately and stays untouched.
func (w *Web) Register(r *gin.Engine) {
	w.registerStatic(r)

	h := handlers.New(w.store)

	// Public pages: shared-deck browsing and the login flow.
	pub := r.Group("/", w.withUser())
	{
		pub.GET("/login", w.loginPage)
		pub.GET("/auth/login/:provider", w.startOAuth)
		pub.GET("/auth/callback", w.oauthCallback)
		pub.POST("/logout", w.logout)
		pub.GET("/shared", w.sharedGalleryPage)
		pub.GET("/shared/:slug", w.sharedDeckPage)
	}

	// Signed-in pages and their form/htmx endpoints.
	app := r.Group("/", w.withUser(), w.requireUser(), h.EnsureProfile())
	{
		app.GET("/", w.homePage)

		app.GET("/decks", w.decksPage)
		app.POST("/decks", w.createDeck)
		app.GET("/decks/:slug", w.deckPage)
		app.POST("/decks/:slug/delete", w.deleteDeck)
		app.GET("/decks/:slug/cards/new", w.newCardPage)
		app.POST("/decks/:slug/cards", w.createCard)
		app.POST("/decks/:slug/import", w.importCSV)
		app.GET("/decks/:slug/export", w.exportCSV)
		app.POST("/decks/:slug/share", w.shareDeck)
		app.POST("/decks/:slug/unshare", w.unshareDeck)

		app.GET("/cards/:id", w.editCardPage)
		app.POST("/cards/:id", w.updateCard)
		app.POST("/cards/:id/delete", w.deleteCard)
		app.POST("/cards/lookup", w.dictionaryLookup)

		app.POST("/shared/:slug/import", w.importSharedDeck)

		app.GET("/study", w.studyPage)
		app.POST("/study/grade", w.gradeCard)
		app.POST("/study/next-round", w.nextRound)
		app.POST("/study/quit", w.quitStudy)

		app.POST("/smart-decks", w.saveSmartDeck)
		app.POST("/smart-decks/:id/delete", w.deleteSmartDeck)

		app.GET("/stats", w.statsPage)
		app.GET("/settings", w.settingsPage)
		app.POST("/settings", w.saveSettings)
	}

	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		w.withUser()(c)
		w.renderError(c, http.StatusNotFound, "찾을 수 없는 페이지예요.")
	})
}

// registerStatic serves the embedded assets. Filenames carry no hash, so a URL
// is only immutable when the template stamped ?v=<assetVersion> on it; bare
// URLs get a short TTL. The service worker adds stale-while-revalidate on top.
func (w *Web) registerStatic(r *gin.Engine) {
	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	server := http.FileServer(http.FS(static))
	cached := func(path string) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Header("Cache-Control", "public, max-age=3600")
			c.Request.URL.Path = path
			server.ServeHTTP(c.Writer, c.Request)
		}
	}
	r.GET("/static/*filepath", func(c *gin.Context) {
		if c.Query("v") == assetVersion() {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			c.Header("Cache-Control", "public, max-age=3600")
		}
		c.Request.URL.Path = "/" + strings.TrimPrefix(c.Param("filepath"), "/")
		server.ServeHTTP(c.Writer, c.Request)
	})
	r.GET("/icons/*filepath", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=86400")
		c.Request.URL.Path = "/icons/" + strings.TrimPrefix(c.Param("filepath"), "/")
		server.ServeHTTP(c.Writer, c.Request)
	})
	// The service worker must load from the root scope it controls.
	r.GET("/sw.js", cached("/sw.js"))
	r.GET("/manifest.webmanifest", cached("/manifest.webmanifest"))
	r.GET("/favicon.ico", cached("/favicon.ico"))
	// The offline fallback the service worker precaches and serves when a page
	// is requested with no network and no cached copy.
	r.GET("/offline.html", cached("/offline.html"))
}

// clientTZ returns the visitor's IANA timezone, reported by app.js in a
// cookie. Before the first page load (or with JS off) it falls back to UTC.
func clientTZ(c *gin.Context) (string, *time.Location) {
	return handlers.Location(cookieValue(c, tzCookie))
}

// endOfToday bounds the due-card queue, in the visitor's local day.
func endOfToday(loc *time.Location) time.Time {
	now := time.Now().In(loc)
	return time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, loc)
}
