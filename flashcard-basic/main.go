// flashcard-basic: 책 2부에서 다루는 최소 플래시카드 앱.
// 덱과 카드 두 테이블만 두고, Gin + html/template + SQLite로 만든다.
package main

import (
	"log"

	"github.com/gin-gonic/gin"
)

// tag::main[]
func main() {
	store, err := OpenStore("flashcard.db")
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	r := gin.Default()
	r.LoadHTMLGlob("templates/*.html")
	r.Static("/static", "./static")
	registerRoutes(r, store)

	log.Println("flashcard-basic listening on http://localhost:8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}

// end::main[]
