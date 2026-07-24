package web

import (
	"net/http"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/gin-gonic/gin"
)

// 덱 공유: 공유 링크 켜고 끄기, 공유 갤러리, 남의 덱 가져오기.

func (w *Web) shareDeck(c *gin.Context)   { w.setDeckShared(c, true) }
func (w *Web) unshareDeck(c *gin.Context) { w.setDeckShared(c, false) }

// setDeckShared turns the deck's public link on or off and returns to the deck
// page either way.
func (w *Web) setDeckShared(c *gin.Context, shared bool) {
	deckID, ok := w.deckIDFromPath(c)
	if !ok {
		return
	}
	userID := auth.UserID(c)
	ctx := c.Request.Context()
	var err error
	message := "공유를 해제했어요"
	if shared {
		_, err = w.store.ShareDeck(ctx, userID, deckID)
		message = "덱을 공유했어요"
	} else {
		err = w.store.UnshareDeck(ctx, userID, deckID)
	}
	if err != nil {
		w.failPage(c, err)
		return
	}
	redirectWithFlash(c, flashInfo, message, "/decks/"+c.Param("slug"))
}

func (w *Web) sharedGalleryPage(c *gin.Context) {
	decks, err := w.store.ListSharedDecks(c.Request.Context(), auth.OptionalUserID(c))
	if err != nil {
		w.failPage(c, err)
		return
	}
	w.render(c, http.StatusOK, "shared", "공유 덱 둘러보기", gin.H{"Decks": decks})
}

func (w *Web) sharedDeckPage(c *gin.Context) {
	slug := c.Param("slug")
	ctx := c.Request.Context()
	deck, err := w.store.GetSharedDeck(ctx, auth.OptionalUserID(c), slug)
	if err != nil {
		if isNotFound(err) {
			w.renderError(c, http.StatusNotFound, "공유가 해제되었거나 존재하지 않는 덱이에요.")
			return
		}
		w.failPage(c, err)
		return
	}
	cards, err := w.store.GetSharedDeckCards(ctx, slug)
	if err != nil {
		w.failPage(c, err)
		return
	}
	w.render(c, http.StatusOK, "shared_deck", deck.Name, gin.H{
		"Slug":  slug,
		"Deck":  deck,
		"Cards": cards,
	})
}

func (w *Web) importSharedDeck(c *gin.Context) {
	deck, err := w.store.ImportSharedDeck(c.Request.Context(), auth.UserID(c), c.Param("slug"))
	if err != nil {
		w.failPage(c, err)
		return
	}
	redirectWithFlash(c, flashInfo, "'"+deck.Name+"' 덱을 가져왔어요", "/decks/"+deck.Slug)
}
