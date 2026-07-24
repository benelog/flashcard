package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/cardcsv"
	"github.com/benelog/flashcard/internal/store"
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
func (b *cardBody) toInput() (store.CardInput, string) {
	b.Text = strings.TrimSpace(b.Text)
	b.Meaning = strings.TrimSpace(b.Meaning)
	if b.Text == "" || b.Meaning == "" {
		return store.CardInput{}, "text and meaning are required"
	}
	if b.CardType == "" {
		b.CardType = store.DefaultCardType
	}
	// API는 폼·CSV와 달리 모르는 종류를 조용히 고치지 않는다. 프로그램이 보내는
	// 값이라 오타라면 알려 주는 편이 낫다.
	if !store.IsCardType(b.CardType) {
		return store.CardInput{}, "cardType must be " + strings.Join(store.CardTypes, ", ")
	}
	// end::to-input[]
	if b.Tags == nil {
		b.Tags = []string{}
	}
	return store.CardInput{
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
	// UpdateCard edits content only and keeps the card in its deck, so no deck
	// slug is needed (the card is identified by its UUID alone).
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

// BulkCreateCards imports rows the client parsed from CSV.
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
	if len(body.Cards) == 0 || len(body.Cards) > store.MaxBulkCards {
		badRequest(c, fmt.Sprintf("cards must contain 1-%d rows", store.MaxBulkCards))
		return
	}
	inputs := make([]store.CardInput, 0, len(body.Cards))
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

// ExportDeck streams the deck as CSV, the same format 가져오기 reads back.
func (h *Handlers) ExportDeck(c *gin.Context) {
	if err := ExportDeckCSV(c, h.Store, auth.UserID(c), c.Param("slug")); err != nil {
		fail(c, err)
	}
}

// ExportDeckCSV streams one deck's cards as a CSV download. 덱 화면의 내보내기
// 버튼(HTML)과 JSON API의 /export가 같은 파일을 받도록 두 곳이 이 함수를 함께
// 쓴다.
//
// 돌려주는 error는 "덱을 찾거나 카드를 읽지 못했다"는 뜻뿐이라, 부르는 쪽이
// 자기 방식(JSON 응답 또는 HTML 오류 화면)으로 알리면 된다. 파일을 흘려보내던
// 중의 실패는 응답이 이미 나가 버려 되돌릴 수 없으므로 여기서 기록만 남긴다.
func ExportDeckCSV(c *gin.Context, s Store, userID uuid.UUID, slug string) error {
	ctx := c.Request.Context()
	deck, err := s.GetDeckBySlug(ctx, userID, slug)
	if err != nil {
		return err
	}
	cards, err := s.ListCards(ctx, userID, deck.ID)
	if err != nil {
		return err
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "deck-"+deck.Slug+".csv"))
	if err := cardcsv.Write(c.Writer, cards); err != nil {
		_ = c.Error(err)
	}
	return nil
}
