package model

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/smartrules"
)

// tag::store-head[]
// Store는 저장소 계약이다. *pgstore.Store(pgx, 배포 환경)와
// *litestore.Store(SQLite, 로컬 모드)가 둘 다 이것을 만족한다.
//
// 계약이 구현 쪽이 아니라 여기 있는 이유는 소비자가 둘이기 때문이다. HTML을
// 그리는 internal/web과 JSON API인 internal/handlers가 같은 저장소를 보는데,
// 한쪽 패키지에 계약을 두면 다른 쪽이 그 패키지를 이름값과 무관하게 의존하게
// 된다.
type Store interface {
	GetOrCreateProfile(ctx context.Context, userID uuid.UUID, displayName string) (Profile, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, displayName *string, settings json.RawMessage) (Profile, error)

	ListDecks(ctx context.Context, userID uuid.UUID) ([]Deck, error)
	GetDeckBySlug(ctx context.Context, userID uuid.UUID, slug string) (Deck, error)
	DeckIDBySlug(ctx context.Context, userID uuid.UUID, slug string) (uuid.UUID, error)
	CreateDeck(ctx context.Context, userID uuid.UUID, name string, description *string) (Deck, error)
	UpdateDeck(ctx context.Context, userID, deckID uuid.UUID, name, description *string) (Deck, error)
	DeleteDeck(ctx context.Context, userID, deckID uuid.UUID) error
	// end::store-head[]

	// 스토리는 덱의 소개 글(마크다운 원문)이다. 본문이 길 수 있어 Deck 행에
	// 싣지 않고 필요한 화면에서만 따로 읽고 쓴다.
	DeckStory(ctx context.Context, userID, deckID uuid.UUID) (*string, error)
	UpdateDeckStory(ctx context.Context, userID, deckID uuid.UUID, story *string) error

	// ListCards는 덱이 호출자의 것이 아니면 ErrNotFound를 돌려준다. 빈 덱과
	// 없는 덱을 같은 빈 목록으로 답하면 호출자가 둘을 가릴 수 없다.
	ListCards(ctx context.Context, userID, deckID uuid.UUID) ([]Card, error)
	GetCard(ctx context.Context, userID, cardID uuid.UUID) (Card, error)
	CreateCard(ctx context.Context, userID uuid.UUID, in CardInput) (Card, error)
	UpdateCard(ctx context.Context, userID, cardID uuid.UUID, in CardInput) (Card, error)
	DeleteCard(ctx context.Context, userID, cardID uuid.UUID) error
	BulkCreateCards(ctx context.Context, userID, deckID uuid.UUID, inputs []CardInput) (BulkResult, error)
	DueCards(ctx context.Context, userID uuid.UUID, dueBefore time.Time, limit int) ([]Card, error)
	DueCount(ctx context.Context, userID uuid.UUID, dueBefore time.Time) (int, error)
	CardsByRule(ctx context.Context, userID uuid.UUID, rule smartrules.Rule) ([]Card, error)
	CountByRule(ctx context.Context, userID uuid.UUID, rule smartrules.Rule) (int, error)

	CreateSession(ctx context.Context, userID uuid.UUID, mode, direction string, deckID *uuid.UUID, rule json.RawMessage, totalCards int) (Session, error)
	RecordReview(ctx context.Context, userID, sessionID, cardID uuid.UUID, result, isRetry bool) (ReviewOutcome, error)
	FinishSession(ctx context.Context, userID, sessionID uuid.UUID, completed bool) error

	ShareDeck(ctx context.Context, userID, deckID uuid.UUID) (ShareInfo, error)
	UnshareDeck(ctx context.Context, userID, deckID uuid.UUID) error
	ListSharedDecks(ctx context.Context, viewerID uuid.UUID) ([]SharedDeckSummary, error)
	GetSharedDeck(ctx context.Context, viewerID uuid.UUID, slug string) (SharedDeckSummary, error)
	GetSharedDeckCards(ctx context.Context, slug string) ([]SharedCard, error)
	// SharedDeckStory는 공유 슬러그만으로 스토리 원문을 읽는다. 보는 이는
	// 주인이 아니므로 userID로 가리는 DeckStory를 쓸 수 없다.
	SharedDeckStory(ctx context.Context, slug string) (*string, error)
	ImportSharedDeck(ctx context.Context, viewerID uuid.UUID, slug string) (Deck, error)

	ListSmartDecks(ctx context.Context, userID uuid.UUID) ([]SmartDeck, error)
	CreateSmartDeck(ctx context.Context, userID uuid.UUID, name string, rule json.RawMessage) (SmartDeck, error)
	DeleteSmartDeck(ctx context.Context, userID, id uuid.UUID) error

	DailyStats(ctx context.Context, userID uuid.UUID, tz string, days int) ([]DailyStat, error)
	StatsSummary(ctx context.Context, userID uuid.UUID, tz string, loc *time.Location) (Summary, error)
}
