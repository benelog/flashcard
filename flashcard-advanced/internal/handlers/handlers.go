package handlers

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/model"
)

type Handlers struct {
	Store model.Store
}

func New(s model.Store) *Handlers {
	return &Handlers{Store: s}
}

// tag::healthz[]
func (h *Handlers) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// end::healthz[]

// EnsureProfile은 auth의 공용 미들웨어에 이 API의 실패 응답(JSON)을 끼운 것이다.
// 화면 쪽은 internal/web이 오류 화면을 끼워 따로 만든다.
func (h *Handlers) EnsureProfile() gin.HandlerFunc {
	return auth.EnsureProfile(h.Store, fail)
}

// tag::error-helpers[]
func fail(c *gin.Context, err error) {
	if errors.Is(err, model.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	log.Printf("internal error: %v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}

func badRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, gin.H{"error": msg})
}

// end::error-helpers[]

// uuidFromPath는 경로의 UUID 파라미터를 읽는다. false를 받으면 응답은 이미 쓰인
// 상태이므로 부르는 쪽은 그대로 돌아가면 된다.
func uuidFromPath(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		badRequest(c, "invalid "+name)
		return uuid.Nil, false
	}
	return id, true
}

// tag::deck-id-from-path[]
// deckIDFromPath는 경로의 :slug(Base36 덱 슬러그)를 요청자의 덱 id로 푼다.
// 형식이 틀리거나 남의 슬러그면 404로 응답한다.
func (h *Handlers) deckIDFromPath(c *gin.Context) (uuid.UUID, bool) {
	return h.deckIDBySlug(c, c.Param("slug"))
}

// end::deck-id-from-path[]

// tag::deck-id-by-slug[]
// deckIDBySlug는 슬러그를 요청자 계정 안에서만 찾으므로, 남의 덱 슬러그는
// 존재하지 않는 슬러그와 구별되지 않는다.
func (h *Handlers) deckIDBySlug(c *gin.Context, slug string) (uuid.UUID, bool) {
	id, err := h.Store.DeckIDBySlug(c.Request.Context(), auth.UserID(c), slug)
	if err != nil {
		fail(c, err)
		return uuid.Nil, false
	}
	return id, true
}

// end::deck-id-by-slug[]
