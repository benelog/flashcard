// embed된 SQL 마이그레이션을 적용한다. 트랜잭션 풀링을 거치지 않는 직접 연결
// 문자열로 실행해야 한다. 예:
//
//	MIGRATE_DATABASE_URL=postgres://... go run ./cmd/migrate
//
// MIGRATE_DATABASE_URL이 없으면 DATABASE_URL을 대신 쓴다.
package main

import (
	"log"
	"os"

	"github.com/benelog/flashcard/internal/db"
)

func main() {
	url := os.Getenv("MIGRATE_DATABASE_URL")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		log.Fatal("MIGRATE_DATABASE_URL or DATABASE_URL is required")
	}
	if err := db.Migrate(url); err != nil {
		log.Fatal(err)
	}
	log.Println("migrations applied")
}
