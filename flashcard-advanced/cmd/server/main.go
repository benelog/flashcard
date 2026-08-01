// 로컬 개발 서버다: go run ./cmd/server (환경 변수는 셸에서 읽는다).
// DATABASE_URL이 없으면 환경 변수 없이 SQLite 파일로 로컬 모드로 뜬다.
package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"github.com/benelog/flashcard/internal/config"
	"github.com/benelog/flashcard/internal/litestore"
	"github.com/benelog/flashcard/pkg/app"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	var engine *gin.Engine
	if cfg.Driver == "sqlite" {
		s, err := litestore.Open(cfg.SQLitePath)
		if err != nil {
			log.Fatal(err)
		}
		engine = app.New(cfg, s)
		log.Printf("flashcard api listening on :%s (local mode, sqlite: %s)", cfg.Port, cfg.SQLitePath)
	} else {
		engine, err = app.Engine()
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("flashcard api listening on :%s (postgres)", cfg.Port)
	}
	if err := engine.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
