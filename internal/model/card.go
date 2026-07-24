package model

import (
	"slices"
	"time"

	"github.com/google/uuid"
)

type Card struct {
	ID        uuid.UUID `json:"id"`
	DeckID    uuid.UUID `json:"deckId"`
	DeckSlug  string    `json:"deckSlug"` // 편집 화면의 덱 링크용으로 GetCard가 채운다
	Text      string    `json:"text"`
	Meaning   string    `json:"meaning"`
	CardType  string    `json:"cardType"`
	Tags      []string  `json:"tags"`
	Phonetic  *string   `json:"phonetic"`
	Example   *string   `json:"example"`
	Notes     *string   `json:"notes"`
	CreatedAt time.Time `json:"createdAt"`

	// cards_with_stats 뷰가 주는 SRS 요약.
	Attempts     int        `json:"attempts"`
	ErrorRate    float64    `json:"errorRate"`
	IntervalDays float64    `json:"intervalDays"`
	DueAt        time.Time  `json:"dueAt"`
	LastReviewed *time.Time `json:"lastReviewedAt"`
}

type CardInput struct {
	DeckID   uuid.UUID `json:"deckId"`
	Text     string    `json:"text"`
	Meaning  string    `json:"meaning"`
	CardType string    `json:"cardType"`
	Tags     []string  `json:"tags"`
	Phonetic *string   `json:"phonetic"`
	Example  *string   `json:"example"`
	Notes    *string   `json:"notes"`
}

// 카드 종류. DB의 cards.card_type 열이 받는 값과 같다. 카드가 들어오는 길은
// JSON API, 웹 폼, CSV 가져오기 셋인데, 어느 길로 들어오든 여기 있는 값 하나로
// 정해지도록 판정을 이 파일에 모았다.
const (
	CardTypeWord     = "word"
	CardTypeSentence = "sentence"
	CardTypeIdiom    = "idiom"
	CardTypeConcept  = "concept"

	// DefaultCardType은 종류를 고르지 않았을 때의 값이다.
	DefaultCardType = CardTypeWord
)

// CardTypes는 허용하는 카드 종류 전부이며, UI가 보여 주는 순서를 따른다.
var CardTypes = []string{CardTypeWord, CardTypeSentence, CardTypeIdiom, CardTypeConcept}

// IsCardType은 t가 허용하는 종류인지 알려 준다. JSON API 요청처럼 잘못된 값을
// 400으로 되돌려 줘야 하는 곳에서 쓴다.
func IsCardType(t string) bool {
	return slices.Contains(CardTypes, t)
}

// NormalizeCardType은 비었거나 모르는 입력을 DefaultCardType으로 되돌린다. 폼과
// CSV처럼 사람이 손으로 채우는 입력에서 쓴다. 오타 하나로 카드 등록 전체를
// 실패시키는 것보다 기본 종류로 받아 두는 편이 낫기 때문이다.
func NormalizeCardType(t string) string {
	if IsCardType(t) {
		return t
	}
	return DefaultCardType
}

type BulkResult struct {
	Added   int `json:"added"`
	Skipped int `json:"skipped"`
}

// MaxBulkCards는 대량 등록 한 번의 상한이다. 한 번의 요청이 통째로 한 트랜잭션이라,
// 무제한으로 받으면 서버리스 함수의 실행 시간 한도에 먼저 걸린다. CSV 업로드와
// JSON API의 대량 등록이 같은 한도를 쓴다.
const MaxBulkCards = 2000

// SortCardsByIDOrder는 cards를 ids가 나열한 순서로 되돌리고, 더는 카드로
// 이어지지 않는 id는 버린다.
//
// 스마트 규칙이 정한 순서(오답률 높은 순 등)는 ID 목록에만 남아 있다. 카드 본문을
// 가져오는 두 번째 조회는 그 순서를 지켜 주지 않으므로 여기서 되살린다. 저장소
// 구현 둘이 같은 사정이라 이 함수를 함께 쓴다.
// tag::sort-by-id-order[]
func SortCardsByIDOrder(cards []Card, ids []uuid.UUID) []Card {
	cardOf := make(map[uuid.UUID]Card, len(cards))
	for _, card := range cards {
		cardOf[card.ID] = card
	}
	ordered := make([]Card, 0, len(cards))
	for _, id := range ids {
		if card, ok := cardOf[id]; ok {
			ordered = append(ordered, card)
		}
	}
	return ordered
}

// end::sort-by-id-order[]
