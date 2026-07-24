// 라우트 등록과 정적 자원 서빙. 어떤 주소가 어느 화면으로 가는지 한눈에
// 보이도록 이 파일 하나에 모아 둔다.
package web

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/benelog/flashcard/internal/handlers"
)

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
