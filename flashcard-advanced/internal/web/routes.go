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

	"github.com/benelog/flashcard/internal/auth"
)

// assetVersion은 embed된 CSS/JS의 내용 해시다. 템플릿이 모든 자산 URL에
// ?v=…로 붙이므로 배포하면 URL 자체가 바뀐다. 정적 파일을
// stale-while-revalidate로 내주는 서비스 워커가 새 페이지에 지난 버전의
// 스타일시트를 짝지을 수 없게 하기 위해서다. 파일 이름은 그대로라 저장소와
// devtools에서 읽기 좋다.
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

// asset은 템플릿 함수다: {{asset "/static/app.css"}}.
func asset(path string) string { return path + "?v=" + assetVersion() }

// Register는 모든 HTML 라우트를 공용 Gin 엔진에 잇는다. /api 아래 JSON API는
// 따로 등록되며 여기서 건드리지 않는다.
func (w *Web) Register(r *gin.Engine) {
	w.registerStatic(r)

	// 공개 페이지: 공유 덱 구경과 로그인 흐름.
	pub := r.Group("/", w.withUser())
	{
		pub.GET("/login", w.loginPage)
		pub.GET("/auth/login/:provider", w.startOAuth)
		pub.GET("/auth/callback", w.oauthCallback)
		pub.POST("/logout", w.logout)
		pub.GET("/shared", w.sharedGalleryPage)
		pub.GET("/shared/:slug", w.sharedDeckPage)
	}

	// 로그인해야 보는 페이지와 그 폼·htmx 끝점. 프로필 보장이 실패하면 JSON이
	// 아니라 오류 화면(failPage)으로 답한다.
	app := r.Group("/", w.withUser(), w.requireUser(), auth.EnsureProfile(w.store, w.failPage))
	{
		app.GET("/", w.homePage)

		app.GET("/decks", w.decksPage)
		app.POST("/decks", w.createDeck)
		app.GET("/decks/:slug", w.deckPage)
		app.POST("/decks/:slug/rename", w.renameDeck)
		app.POST("/decks/:slug/delete", w.deleteDeck)
		app.GET("/decks/:slug/story", w.storyEditPage)
		app.POST("/decks/:slug/story", w.saveStory)
		app.POST("/decks/:slug/story/preview", w.previewStory)
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

// registerStatic은 embed된 자산을 내준다. 파일 이름에 해시가 없으므로 템플릿이
// ?v=<assetVersion>을 찍은 URL만 immutable로 두고, 맨 URL에는 짧은 TTL을 준다.
// 서비스 워커가 그 위에 stale-while-revalidate를 얹는다.
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
	// 서비스 워커는 자기가 제어할 루트 스코프에서 로드되어야 한다.
	r.GET("/sw.js", cached("/sw.js"))
	r.GET("/manifest.webmanifest", cached("/manifest.webmanifest"))
	r.GET("/favicon.ico", cached("/favicon.ico"))
	// 네트워크도 캐시 사본도 없을 때 서비스 워커가 내주는 오프라인 대체 화면.
	// 서비스 워커가 미리 캐시해 둔다.
	r.GET("/offline.html", cached("/offline.html"))
}
