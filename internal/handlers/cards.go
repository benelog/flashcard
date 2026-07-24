package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/cardcsv"
	"github.com/benelog/flashcard/internal/model"
)

// tag::card-body[]
type cardBody struct {
	DeckSlug string   `json:"deckSlug"`
	Text     string   `json:"text"`
	Meaning  string   `json:"meaning"`
	CardType string   `json:"cardType"`
	Tags     []string `json:"tags"`
	Phonetic *string  `json:"phonetic"`
	Example  *string  `json:"example"`
	Notes    *string  `json:"notes"`
}

// end::card-body[]

// tag::to-input[]
func (b *cardBody) toInput() (model.CardInput, string) {
	b.Text = strings.TrimSpace(b.Text)
	b.Meaning = strings.TrimSpace(b.Meaning)
	if b.Text == "" || b.Meaning == "" {
		return model.CardInput{}, "text and meaning are required"
	}
	if b.CardType == "" {
		b.CardType = model.DefaultCardType
	}
	// API는 폼·CSV와 달리 모르는 종류를 조용히 고치지 않는다. 프로그램이 보내는
	// 값이라 오타라면 알려 주는 편이 낫다.
	if !model.IsCardType(b.CardType) {
		return model.CardInput{}, "cardType must be " + strings.Join(model.CardTypes, ", ")
	}
	// end::to-input[]
	if b.Tags == nil {
		b.Tags = []string{}
	}
	return model.CardInput{
		Text: b.Text, Meaning: b.Meaning,
		CardType: b.CardType, Tags: b.Tags,
		Phonetic: b.Phonetic, Example: b.Example, Notes: b.Notes,
	}, ""
}

func (h *Handlers) ListDeckCards(c *gin.Context) {
	deckID, ok := h.deckIDFromPath(c)
	if !ok {
		return
	}
	cards, err := h.Store.ListCards(c.Request.Context(), auth.UserID(c), deckID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, cards)
}

func (h *Handlers) CreateCard(c *gin.Context) {
	var body cardBody
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, "invalid body")
		return
	}
	in, msg := body.toInput()
	if msg != "" {
		badRequest(c, msg)
		return
	}
	// 새 카드는 경로가 아니라 본문의 덱 슬러그로 어느 덱에 담길지 정한다.
	deckID, ok := h.deckIDBySlug(c, body.DeckSlug)
	if !ok {
		return
	}
	in.DeckID = deckID
	card, err := h.Store.CreateCard(c.Request.Context(), auth.UserID(c), in)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, card)
}

func (h *Handlers) GetCard(c *gin.Context) {
	cardID, ok := uuidFromPath(c, "id")
	if !ok {
		return
	}
	card, err := h.Store.GetCard(c.Request.Context(), auth.UserID(c), cardID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, card)
}

func (h *Handlers) UpdateCard(c *gin.Context) {
	cardID, ok := uuidFromPath(c, "id")
	if !ok {
		return
	}
	var body cardBody
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, "invalid body")
		return
	}
	in, msg := body.toInput()
	if msg != "" {
		badRequest(c, msg)
		return
	}
	// 수정은 내용만 바꾸고 카드를 원래 덱에 그대로 두므로 덱 슬러그가 필요
	// 없다. UUID만으로 카드를 특정한다.
	card, err := h.Store.UpdateCard(c.Request.Context(), auth.UserID(c), cardID, in)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, card)
}

func (h *Handlers) DeleteCard(c *gin.Context) {
	cardID, ok := uuidFromPath(c, "id")
	if !ok {
		return
	}
	if err := h.Store.DeleteCard(c.Request.Context(), auth.UserID(c), cardID); err != nil {
		fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// BulkCreateCards는 클라이언트가 CSV에서 파싱해 보낸 행들을 들여온다.
func (h *Handlers) BulkCreateCards(c *gin.Context) {
	deckID, ok := h.deckIDFromPath(c)
	if !ok {
		return
	}
	var body struct {
		Cards []cardBody `json:"cards"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, "invalid body")
		return
	}
	if len(body.Cards) == 0 || len(body.Cards) > model.MaxBulkCards {
		badRequest(c, fmt.Sprintf("cards must contain 1-%d rows", model.MaxBulkCards))
		return
	}
	inputs := make([]model.CardInput, 0, len(body.Cards))
	invalid := 0
	for _, cb := range body.Cards {
		in, msg := cb.toInput()
		if msg != "" {
			invalid++
			continue
		}
		inputs = append(inputs, in)
	}
	res, err := h.Store.BulkCreateCards(c.Request.Context(), auth.UserID(c), deckID, inputs)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"added": res.Added, "skipped": res.Skipped, "invalid": invalid})
}

// ExportDeck은 덱을 가져오기가 다시 읽을 수 있는 CSV로 내려보낸다. 파일을
// 만드는 일은 cardcsv.ExportDeck이 하고, 덱 화면의 내보내기 버튼도 같은 함수를
// 쓴다.
func (h *Handlers) ExportDeck(c *gin.Context) {
	if err := cardcsv.ExportDeck(c.Request.Context(), h.Store, auth.UserID(c), c.Param("slug"), c.Writer); err != nil {
		fail(c, err)
	}
}
