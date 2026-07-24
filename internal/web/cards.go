package web

import (
	"net/http"
	"strings"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/model"
	"github.com/gin-gonic/gin"
)

// 카드 추가·수정·삭제 화면과 폼 처리.

// cardFormView carries the (re)rendered card form: current values plus the
// submit target, shared by the new-card page, the edit page and the
// dictionary-lookup fragment.
type cardFormView struct {
	Action   string
	BackURL  string
	Editing  bool
	Text     string
	Meaning  string
	CardType string
	Tags     string
	Phonetic string
	Example  string
	Notes    string
	Status   string // 사전 조회 결과 메시지
}

func (w *Web) newCardPage(c *gin.Context) {
	// 존재하지 않는 덱이면 빈 폼을 보여 주기 전에 404를 낸다.
	if _, ok := w.deckIDFromPath(c); !ok {
		return
	}
	slug := c.Param("slug")
	w.render(c, http.StatusOK, "card_form", "새 카드", cardFormView{
		Action:   "/decks/" + slug + "/cards",
		BackURL:  "/decks/" + slug,
		CardType: model.DefaultCardType,
	})
}

func (w *Web) editCardPage(c *gin.Context) {
	cardID, ok := w.uuidFromPath(c, "id", "찾을 수 없는 카드예요.")
	if !ok {
		return
	}
	card, err := w.store.GetCard(c.Request.Context(), auth.UserID(c), cardID)
	if err != nil {
		w.failPage(c, err)
		return
	}
	w.render(c, http.StatusOK, "card_form", "카드 수정", cardFormView{
		Action:   "/cards/" + card.ID.String(),
		BackURL:  "/decks/" + card.DeckSlug,
		Editing:  true,
		Text:     card.Text,
		Meaning:  card.Meaning,
		CardType: card.CardType,
		Tags:     strings.Join(card.Tags, ", "),
		Phonetic: model.OrEmpty(card.Phonetic),
		Example:  model.OrEmpty(card.Example),
		Notes:    model.OrEmpty(card.Notes),
	})
}

// cardInputFromForm normalizes the posted card fields. 두 번째 반환값은 이 폼을
// 저장할 수 있는지로, 원문과 뜻이 모두 있어야 참이다.
func cardInputFromForm(c *gin.Context) (model.CardInput, bool) {
	in := model.CardInput{
		Text:     strings.TrimSpace(c.PostForm("text")),
		Meaning:  strings.TrimSpace(c.PostForm("meaning")),
		CardType: model.NormalizeCardType(c.PostForm("card_type")),
		Tags:     splitAndTrim(c.PostForm("tags"), ","),
		Phonetic: model.NilIfBlank(c.PostForm("phonetic")),
		Example:  model.NilIfBlank(c.PostForm("example")),
		Notes:    model.NilIfBlank(c.PostForm("notes")),
	}
	return in, in.Text != "" && in.Meaning != ""
}
func (w *Web) createCard(c *gin.Context) {
	slug := c.Param("slug")
	in, ok := cardInputFromForm(c)
	if !ok {
		redirectWithFlash(c, flashError, "원문과 뜻을 모두 입력해주세요", "/decks/"+slug+"/cards/new")
		return
	}
	deckID, ok := w.deckIDFromPath(c)
	if !ok {
		return
	}
	in.DeckID = deckID
	if _, err := w.store.CreateCard(c.Request.Context(), auth.UserID(c), in); err != nil {
		w.failPage(c, err)
		return
	}
	redirectWithFlash(c, flashInfo, "카드를 추가했어요", "/decks/"+slug)
}

func (w *Web) updateCard(c *gin.Context) {
	cardID, ok := w.uuidFromPath(c, "id", "찾을 수 없는 카드예요.")
	if !ok {
		return
	}
	in, ok := cardInputFromForm(c)
	if !ok {
		redirectWithFlash(c, flashError, "원문과 뜻을 모두 입력해주세요", "/cards/"+cardID.String())
		return
	}
	card, err := w.store.UpdateCard(c.Request.Context(), auth.UserID(c), cardID, in)
	if err != nil {
		w.failPage(c, err)
		return
	}
	redirectWithFlash(c, flashInfo, "카드를 수정했어요", "/decks/"+card.DeckSlug)
}

func (w *Web) deleteCard(c *gin.Context) {
	cardID, ok := w.uuidFromPath(c, "id", "찾을 수 없는 카드예요.")
	if !ok {
		return
	}
	if err := w.store.DeleteCard(c.Request.Context(), auth.UserID(c), cardID); err != nil {
		w.failPage(c, err)
		return
	}
	// htmx가 목록의 해당 <li>를 지우도록 빈 본문을 돌려준다.
	if isHTMX(c) {
		c.Status(http.StatusOK)
		return
	}
	setFlash(c, flashInfo, "카드를 삭제했어요")
	redirectBack(c, "/decks")
}
