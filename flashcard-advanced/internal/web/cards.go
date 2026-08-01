package web

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/model"
	"github.com/gin-gonic/gin"
)

// 카드 추가·수정·삭제 화면과 폼 처리.

// cardFormView는 카드 폼에 그릴 현재 값과 제출 대상이다. 새 카드 화면,
// 수정 화면, 사전 조회 조각이 함께 쓴다.
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

// cardInputFromForm은 제출된 카드 필드를 정규화한다. 두 번째 반환값은 이 폼을
// 저장할 수 있는지로, 원문과 뜻이 모두 있어야 참이다. url.Values만 보므로 HTTP
// 없이 검증할 수 있다.
func cardInputFromForm(form url.Values) (model.CardInput, bool) {
	in := model.CardInput{
		Text:     strings.TrimSpace(form.Get("text")),
		Meaning:  strings.TrimSpace(form.Get("meaning")),
		CardType: model.NormalizeCardType(form.Get("card_type")),
		Tags:     splitAndTrim(form.Get("tags"), ","),
		Phonetic: model.NilIfBlank(form.Get("phonetic")),
		Example:  model.NilIfBlank(form.Get("example")),
		Notes:    model.NilIfBlank(form.Get("notes")),
	}
	return in, in.Text != "" && in.Meaning != ""
}

func (w *Web) createCard(c *gin.Context) {
	// newCardPage와 같은 순서로 덱부터 확인한다. 폼 오류를 먼저 알리면 남의 덱
	// 슬러그로도 응답이 갈라져 슬러그의 존재가 드러난다.
	deckID, ok := w.deckIDFromPath(c)
	if !ok {
		return
	}
	slug := c.Param("slug")
	in, ok := cardInputFromForm(postFormValues(c))
	if !ok {
		redirectWithFlash(c, flashError, "원문과 뜻을 모두 입력해주세요", "/decks/"+slug+"/cards/new")
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
	in, ok := cardInputFromForm(postFormValues(c))
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
