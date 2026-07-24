package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/smartrules"
	"github.com/benelog/flashcard/internal/store"
)

// Store is the persistence contract the handlers consume. *store.Store (pgx,
// production) and *litestore.Store (SQLite, local mode) both satisfy it; the
// row types and the ErrNotFound sentinel stay shared from internal/store.
type Store interface {
	GetOrCreateProfile(ctx context.Context, userID uuid.UUID, displayName string) (store.Profile, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, displayName *string, settings json.RawMessage) (store.Profile, error)

	ListDecks(ctx context.Context, userID uuid.UUID) ([]store.Deck, error)
	GetDeckBySlug(ctx context.Context, userID uuid.UUID, slug string) (store.Deck, error)
	DeckIDBySlug(ctx context.Context, userID uuid.UUID, slug string) (uuid.UUID, error)
	CreateDeck(ctx context.Context, userID uuid.UUID, name string, description *string) (store.Deck, error)
	UpdateDeck(ctx context.Context, userID, deckID uuid.UUID, name, description *string) (store.Deck, error)
	DeleteDeck(ctx context.Context, userID, deckID uuid.UUID) error

	ListCards(ctx context.Context, userID, deckID uuid.UUID) ([]store.Card, error)
	GetCard(ctx context.Context, userID, cardID uuid.UUID) (store.Card, error)
	CreateCard(ctx context.Context, userID uuid.UUID, in store.CardInput) (store.Card, error)
	UpdateCard(ctx context.Context, userID, cardID uuid.UUID, in store.CardInput) (store.Card, error)
	DeleteCard(ctx context.Context, userID, cardID uuid.UUID) error
	BulkCreateCards(ctx context.Context, userID, deckID uuid.UUID, inputs []store.CardInput) (store.BulkResult, error)
	DueCards(ctx context.Context, userID uuid.UUID, dueBefore time.Time, limit int) ([]store.Card, error)
	DueCount(ctx context.Context, userID uuid.UUID, dueBefore time.Time) (int, error)
	CardsByRule(ctx context.Context, userID uuid.UUID, rule smartrules.Rule) ([]store.Card, error)
	CountByRule(ctx context.Context, userID uuid.UUID, rule smartrules.Rule) (int, error)

	CreateSession(ctx context.Context, userID uuid.UUID, mode, direction string, deckID *uuid.UUID, rule json.RawMessage, totalCards int) (store.Session, error)
	RecordReview(ctx context.Context, userID, sessionID, cardID uuid.UUID, result, isRetry bool) (store.ReviewOutcome, error)
	FinishSession(ctx context.Context, userID, sessionID uuid.UUID, completed bool) error

	ShareDeck(ctx context.Context, userID, deckID uuid.UUID) (store.ShareInfo, error)
	UnshareDeck(ctx context.Context, userID, deckID uuid.UUID) error
	ListSharedDecks(ctx context.Context, viewerID uuid.UUID) ([]store.SharedDeckSummary, error)
	GetSharedDeck(ctx context.Context, viewerID uuid.UUID, slug string) (store.SharedDeckSummary, error)
	GetSharedDeckCards(ctx context.Context, slug string) ([]store.SharedCard, error)
	ImportSharedDeck(ctx context.Context, viewerID uuid.UUID, slug string) (store.Deck, error)

	ListSmartDecks(ctx context.Context, userID uuid.UUID) ([]store.SmartDeck, error)
	CreateSmartDeck(ctx context.Context, userID uuid.UUID, name string, rule json.RawMessage) (store.SmartDeck, error)
	DeleteSmartDeck(ctx context.Context, userID, id uuid.UUID) error

	DailyStats(ctx context.Context, userID uuid.UUID, tz string, days int) ([]store.DailyStat, error)
	StatsSummary(ctx context.Context, userID uuid.UUID, tz string, loc *time.Location) (store.Summary, error)
}

type Handlers struct {
	Store Store
}

func New(s Store) *Handlers {
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
	if errors.Is(err, store.ErrNotFound) {
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
