// Package app builds the Gin engine shared by the local dev server
// (cmd/server) and the Vercel serverless entrypoint (api/index.go).
//
// 이 패키지만 internal/ 이 아니라 pkg/ 에 있다. Vercel의 Go 빌더가 api/index.go를
// 모듈 바깥에서 컴파일해 internal/ 을 가져올 수 없기 때문이다. internal/app으로
// 옮기면 로컬 빌드와 테스트는 그대로 통과하고 배포만 깨지므로, 위치를 바꾸기
// 전에 api/index.go의 주석을 함께 읽는다.
//
// This package must never import internal/litestore: Engine serves Vercel,
// and the serverless binary must not link SQLite.
package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/config"
	"github.com/benelog/flashcard/internal/db"
	"github.com/benelog/flashcard/internal/handlers"
	"github.com/benelog/flashcard/internal/model"
	"github.com/benelog/flashcard/internal/pgstore"
	"github.com/benelog/flashcard/internal/web"
)

var (
	engine     *gin.Engine
	engineOnce sync.Once
	engineErr  error
)

// Engine returns the process-wide router; warm serverless instances reuse it.
func Engine() (*gin.Engine, error) {
	engineOnce.Do(func() {
		engine, engineErr = build()
	})
	return engine, engineErr
}

func build() (*gin.Engine, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if cfg.Driver != "postgres" {
		return nil, fmt.Errorf("app.Engine requires postgres, got driver %q", cfg.Driver)
	}
	pool, err := db.Pool(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	return New(cfg, pgstore.New(pool)), nil
}

// New assembles the router on top of any Store implementation. cfg.AuthMode
// picks the middleware: Supabase token validation in production, the fixed
// local user in local mode.
func New(cfg *config.Config, s model.Store) *gin.Engine {
	h := handlers.New(s)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	if len(cfg.AllowedOrigins) > 0 {
		r.Use(cors.New(cors.Config{
			AllowOrigins:     cfg.AllowedOrigins,
			AllowMethods:     []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Authorization", "Content-Type"},
			MaxAge:           12 * time.Hour,
			AllowCredentials: false,
		}))
	}

	required := auth.Middleware(cfg.JWKSURL, cfg.JWTSecret)
	optional := auth.OptionalMiddleware(cfg.JWKSURL, cfg.JWTSecret)
	if cfg.AuthMode == "local" {
		required = auth.LocalMiddleware()
		optional = auth.LocalMiddleware()
	}

	r.GET("/api/healthz", h.Healthz)

	// Public: browsing shared decks needs no login. Optional auth only lets a
	// signed-in caller see the "is mine" flag on their own shared decks.
	// Responses vary by Authorization, so keep shared caches from reusing a
	// signed-in caller's personalized body for anonymous visitors.
	pub := r.Group("/api", optional, func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
	})
	{
		pub.GET("/shared-decks", h.ListSharedDecks)
		pub.GET("/shared-decks/:slug", h.GetSharedDeck)
	}

	api := r.Group("/api", required, h.EnsureProfile())
	{
		api.GET("/me", h.GetMe)
		api.PATCH("/me", h.UpdateMe)

		api.GET("/decks", h.ListDecks)
		api.POST("/decks", h.CreateDeck)
		// Decks are addressed by their short Base36 slug, not the UUID.
		api.GET("/decks/:slug", h.GetDeck)
		api.PATCH("/decks/:slug", h.UpdateDeck)
		api.DELETE("/decks/:slug", h.DeleteDeck)
		api.GET("/decks/:slug/cards", h.ListDeckCards)
		api.POST("/decks/:slug/cards/bulk", h.BulkCreateCards)
		api.GET("/decks/:slug/export", h.ExportDeck)
		api.POST("/decks/:slug/share", h.ShareDeck)
		api.DELETE("/decks/:slug/share", h.UnshareDeck)

		api.POST("/shared-decks/:slug/import", h.ImportSharedDeck)

		api.POST("/cards", h.CreateCard)
		api.GET("/cards/:id", h.GetCard)
		api.PATCH("/cards/:id", h.UpdateCard)
		api.DELETE("/cards/:id", h.DeleteCard)

		api.POST("/sessions", h.CreateSession)
		api.POST("/sessions/:id/reviews", h.RecordReview)
		api.POST("/sessions/:id/finish", h.FinishSession)
		api.GET("/due-count", h.DueCount)

		api.GET("/suggestions", h.Suggestions)
		api.GET("/smart-decks", h.ListSmartDecks)
		api.POST("/smart-decks", h.CreateSmartDeck)
		api.DELETE("/smart-decks/:id", h.DeleteSmartDeck)

		api.GET("/stats/daily", h.DailyStats)
		api.GET("/stats/summary", h.StatsSummary)
	}

	// HTML pages: server-rendered templates + htmx, cookie sessions. The API
	// above stays token-based for programmatic clients.
	web.New(cfg, s).Register(r)

	return r
}
