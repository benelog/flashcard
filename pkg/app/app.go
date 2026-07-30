// Package app은 로컬 서버(cmd/server)와 Vercel 서버리스 진입점(api/index.go)이
// 함께 쓰는 Gin 엔진을 만든다.
//
// 이 패키지만 internal/ 이 아니라 pkg/ 에 있다. Vercel의 Go 빌더가 api/index.go를
// 모듈 바깥에서 컴파일해 internal/ 을 가져올 수 없기 때문이다. internal/app으로
// 옮기면 로컬 빌드와 테스트는 그대로 통과하고 배포만 깨지므로, 위치를 바꾸기
// 전에 api/index.go의 주석을 함께 읽는다.
//
// 이 패키지는 internal/litestore를 import하면 안 된다. Engine은 Vercel에서
// 돌고, 서버리스 바이너리에 SQLite가 링크되면 안 되기 때문이다. 예외는 _test
// 파일뿐이다(테스트는 배포 바이너리에 들어가지 않으므로 임시 SQLite로 앱을
// 통째로 띄운다).
package app

import (
	"context"
	"fmt"
	"sync"

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

// Engine은 프로세스 전역 라우터를 돌려준다. 서버리스 인스턴스가 따뜻할 때는
// 재사용된다.
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

// New는 아무 Store 구현 위에 라우터를 조립한다. 미들웨어는 cfg.AuthMode가
// 고른다. 운영은 Supabase 토큰 검증, 로컬 모드는 고정 사용자다.
func New(cfg *config.Config, s model.Store) *gin.Engine {
	h := handlers.New(s)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	required := auth.Middleware(cfg.JWKSURL, cfg.JWTSecret)
	optional := auth.OptionalMiddleware(cfg.JWKSURL, cfg.JWTSecret)
	if cfg.AuthMode == "local" {
		required = auth.LocalMiddleware()
		optional = auth.LocalMiddleware()
	}

	r.GET("/api/healthz", h.Healthz)

	// 공개 구간: 공유 덱 구경은 로그인이 필요 없다. 선택적 인증은 로그인한
	// 사용자에게 자기 공유 덱의 "내 것" 표시를 보여 주기 위한 것뿐이다.
	// 응답이 Authorization에 따라 달라지므로, 공유 캐시가 로그인 사용자의
	// 개인화된 본문을 익명 방문자에게 재사용하지 않도록 no-store를 붙인다.
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
		// 덱은 UUID가 아니라 짧은 Base36 슬러그로 가리킨다.
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

	// HTML 화면: 서버 렌더링 템플릿 + htmx, 쿠키 세션. 위의 API는 프로그램
	// 호출자를 위해 토큰 기반으로 남는다.
	web.New(cfg, s).Register(r)

	return r
}
