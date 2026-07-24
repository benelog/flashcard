package handlers

import (
	"errors"
	"log"
	"net/http"
	"sync"

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

// EnsureProfile lazily creates the caller's profile row so that any first
// write (deck create, import, …) satisfies the profiles(id) foreign keys.
// Runs once per user per warm instance.
func (h *Handlers) EnsureProfile() gin.HandlerFunc {
	var seen sync.Map
	return func(c *gin.Context) {
		userID := auth.UserID(c)
		if _, ok := seen.Load(userID); !ok {
			if _, err := h.Store.GetOrCreateProfile(c.Request.Context(), userID, ""); err != nil {
				fail(c, err)
				c.Abort()
				return
			}
			seen.Store(userID, struct{}{})
		}
		c.Next()
	}
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

// uuidFromPath reads a UUID path parameter. false를 받으면 응답은 이미 쓰인
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
// deckIDFromPath resolves the :slug path param (Base36 deck slug) to the
// caller's deck id; a malformed or foreign slug responds 404.
func (h *Handlers) deckIDFromPath(c *gin.Context) (uuid.UUID, bool) {
	return h.deckIDBySlug(c, c.Param("slug"))
}

// end::deck-id-from-path[]

// tag::deck-id-by-slug[]
// deckIDBySlug looks the slug up under the caller's account, so a slug from
// another user's deck is indistinguishable from one that does not exist.
func (h *Handlers) deckIDBySlug(c *gin.Context, slug string) (uuid.UUID, bool) {
	id, err := h.Store.DeckIDBySlug(c.Request.Context(), auth.UserID(c), slug)
	if err != nil {
		fail(c, err)
		return uuid.Nil, false
	}
	return id, true
}

// end::deck-id-by-slug[]
