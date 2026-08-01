package main

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// tag::routes[]
func registerRoutes(r *gin.Engine, store *Store) {
	r.GET("/", showDecks(store))
	r.POST("/decks", createDeck(store))
	r.GET("/decks/:id", showCards(store))
	r.POST("/decks/:id/cards", createCard(store))
	r.DELETE("/cards/:id", deleteCard(store))
	r.GET("/decks/:id/study", showStudy(store))
	r.GET("/api/decks", listDecksJSON(store))
}

// end::routes[]

// tag::show-decks[]
func showDecks(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		decks, err := store.ListDecks()
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.HTML(http.StatusOK, "decks.html", gin.H{"Decks": decks})
	}
}

// end::show-decks[]

// tag::create-deck[]
func createDeck(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := strings.TrimSpace(c.PostForm("name"))
		if name == "" {
			c.String(http.StatusBadRequest, "덱 이름이 비어 있다")
			return
		}
		if _, err := store.CreateDeck(name); err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.Redirect(http.StatusSeeOther, "/")
	}
}

// end::create-deck[]

// tag::show-cards[]
func showCards(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.String(http.StatusBadRequest, "잘못된 덱 번호다")
			return
		}
		deck, err := store.GetDeck(id)
		if errors.Is(err, ErrNotFound) {
			c.String(http.StatusNotFound, "그런 덱이 없다")
			return
		}
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		cards, err := store.ListCards(id)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.HTML(http.StatusOK, "cards.html", gin.H{"Deck": deck, "Cards": cards})
	}
}

// end::show-cards[]

// tag::create-card[]
func createCard(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.String(http.StatusBadRequest, "잘못된 덱 번호다")
			return
		}
		front := strings.TrimSpace(c.PostForm("front"))
		back := strings.TrimSpace(c.PostForm("back"))
		if front == "" || back == "" {
			c.String(http.StatusBadRequest, "앞면과 뒷면을 모두 채운다")
			return
		}
		card, err := store.CreateCard(id, front, back)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		// htmx 요청이면 새 카드 한 조각만 돌려주고,
		// 일반 폼 제출이면 카드 목록 화면으로 돌려보낸다.
		if c.GetHeader("HX-Request") == "true" {
			c.HTML(http.StatusOK, "card-item", card)
			return
		}
		c.Redirect(http.StatusSeeOther, "/decks/"+c.Param("id"))
	}
}

// end::create-card[]

// tag::delete-card[]
func deleteCard(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.String(http.StatusBadRequest, "잘못된 카드 번호다")
			return
		}
		if err := store.DeleteCard(id); errors.Is(err, ErrNotFound) {
			c.String(http.StatusNotFound, "그런 카드가 없다")
			return
		} else if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		// 본문 없는 200 응답. htmx가 hx-target으로 지정한 요소를 빈 내용으로 바꾼다.
		c.Status(http.StatusOK)
	}
}

// end::delete-card[]

// tag::show-study[]
func showStudy(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.String(http.StatusBadRequest, "잘못된 덱 번호다")
			return
		}
		deck, err := store.GetDeck(id)
		if errors.Is(err, ErrNotFound) {
			c.String(http.StatusNotFound, "그런 덱이 없다")
			return
		}
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		cards, err := store.ListCards(id)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		data := gin.H{"Deck": deck, "Total": len(cards)}
		if len(cards) > 0 {
			// ?i= 파라미터가 없거나 범위를 벗어나면 첫 카드부터 시작한다.
			i, _ := strconv.Atoi(c.DefaultQuery("i", "0"))
			if i < 0 || i >= len(cards) {
				i = 0
			}
			data["Card"] = cards[i]
			data["Pos"] = i + 1
			data["Next"] = (i + 1) % len(cards)
		}
		c.HTML(http.StatusOK, "study.html", data)
	}
}

// end::show-study[]

// tag::list-decks-json[]
func listDecksJSON(store *Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		decks, err := store.ListDecks()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, decks)
	}
}

// end::list-decks-json[]
