package web

import (
	"net/http"
	"strings"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/model"
	"github.com/gin-gonic/gin"
)

// 덱 목록과 덱 상세 화면, 그리고 덱을 만들고 지우는 폼 처리.

func (w *Web) decksPage(c *gin.Context) {
	userID := auth.UserID(c)
	ctx := c.Request.Context()
	decks, err := w.store.ListDecks(ctx, userID)
	if err != nil {
		w.failPage(c, err)
		return
	}
	smartDecks, err := w.store.ListSmartDecks(ctx, userID)
	if err != nil {
		w.failPage(c, err)
		return
	}
	w.render(c, http.StatusOK, "decks", "덱", gin.H{
		"Decks":      decks,
		"SmartDecks": smartDecks,
	})
}

func (w *Web) createDeck(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		redirectWithFlash(c, flashError, "덱 이름을 입력해주세요", "/decks")
		return
	}
	deck, err := w.store.CreateDeck(c.Request.Context(), auth.UserID(c), name, nil)
	if err != nil {
		w.failPage(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/decks/"+deck.Slug)
}

// renameDeck은 덱 이름만 바꾼다. URL 슬러그는 덱의 일련번호에서 나오므로
// 이름을 바꿔도 그대로고, 공유 링크(share_slug)도 따로 있어 영향을 받지 않는다.
func (w *Web) renameDeck(c *gin.Context) {
	deckID, ok := w.deckIDFromPath(c)
	if !ok {
		return
	}
	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		redirectWithFlash(c, flashError, "덱 이름을 입력해주세요", "/decks/"+c.Param("slug"))
		return
	}
	// description에 nil을 주면 저장소가 기존 값을 그대로 둔다.
	if _, err := w.store.UpdateDeck(c.Request.Context(), auth.UserID(c), deckID, &name, nil); err != nil {
		w.failPage(c, err)
		return
	}
	redirectWithFlash(c, flashInfo, "덱 이름을 바꿨어요", "/decks/"+c.Param("slug"))
}

func (w *Web) deckPage(c *gin.Context) {
	userID := auth.UserID(c)
	ctx := c.Request.Context()
	deck, err := w.store.GetDeckBySlug(ctx, userID, c.Param("slug"))
	if err != nil {
		w.failPage(c, err)
		return
	}
	cards, err := w.store.ListCards(ctx, userID, deck.ID)
	if err != nil {
		w.failPage(c, err)
		return
	}
	story, err := w.store.DeckStory(ctx, userID, deck.ID)
	if err != nil {
		w.failPage(c, err)
		return
	}
	// 읽기 속도는 스토리를 읽어 줄 때만 쓰므로, 스토리가 있을 때만 프로필을 읽는다.
	ttsRate := defaultTtsRate
	if story != nil {
		profile, err := w.store.GetOrCreateProfile(ctx, userID, "")
		if err != nil {
			w.failPage(c, err)
			return
		}
		ttsRate = settingsFrom(profile).TtsRate
	}
	shareURL := w.shareURL(c, deck)
	// 스토리 링크는 같은 공유 페이지를 스토리가 펼쳐진 상태로 연다. 스토리가
	// 없으면 펼칠 것이 없으므로 만들지 않는다.
	storyShareURL := ""
	if shareURL != "" && story != nil {
		storyShareURL = shareURL + "?story=1"
	}
	w.render(c, http.StatusOK, "deck", deck.Name, gin.H{
		"Deck":          deck,
		"Cards":         cards,
		"ShareURL":      shareURL,
		"StoryShareURL": storyShareURL,
		"StoryHTML":     markdownHTML(model.OrEmpty(story)),
		"TtsRate":       ttsRate,
	})
}

func (w *Web) shareURL(c *gin.Context, deck model.Deck) string {
	if deck.ShareSlug == nil {
		return ""
	}
	return origin(c) + "/shared/" + *deck.ShareSlug
}

func (w *Web) deleteDeck(c *gin.Context) {
	deckID, ok := w.deckIDFromPath(c)
	if !ok {
		return
	}
	if err := w.store.DeleteDeck(c.Request.Context(), auth.UserID(c), deckID); err != nil {
		w.failPage(c, err)
		return
	}
	setFlash(c, flashInfo, "덱을 삭제했어요")
	// 삭제한 덱의 화면에 머물 수 없으므로 목록으로 보낸다. htmx 요청은 조각만
	// 갈아 끼우므로 이동을 HX-Redirect 헤더로 지시해야 한다.
	if isHTMX(c) {
		c.Header("HX-Redirect", "/decks")
		c.Status(http.StatusOK)
		return
	}
	c.Redirect(http.StatusSeeOther, "/decks")
}
