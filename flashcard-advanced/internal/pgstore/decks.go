package pgstore

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/benelog/flashcard/internal/model"
)

const deckSelect = `
	select d.id, d.name, d.description,
	       (select count(*) from cards c where c.deck_id = d.id) as card_count,
	       d.share_slug, d.shared_at, d.created_at, d.updated_at, d.seq
	from decks d`

func scanDeck(row pgx.Row) (model.Deck, error) {
	var d model.Deck
	var seq int64
	// tag::scan-not-found[]
	err := row.Scan(&d.ID, &d.Name, &d.Description, &d.CardCount,
		&d.ShareSlug, &d.SharedAt, &d.CreatedAt, &d.UpdatedAt, &seq)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, model.ErrNotFound
	}
	// end::scan-not-found[]
	d.Slug = model.EncodeDeckSlug(seq)
	return d, err
}

// tag::list-decks[]
func (s *Store) ListDecks(ctx context.Context, userID uuid.UUID) ([]model.Deck, error) {
	rows, err := s.pool.Query(ctx, deckSelect+` where d.user_id = $1 order by d.created_at desc`, userID)
	if err != nil {
		return nil, err
	}
	return collect(rows, scanDeck)
}

// end::list-decks[]

func (s *Store) GetDeck(ctx context.Context, userID, deckID uuid.UUID) (model.Deck, error) {
	return scanDeck(s.pool.QueryRow(ctx, deckSelect+` where d.user_id = $1 and d.id = $2`, userID, deckID))
}

// GetDeckBySlug는 공개 Base36 URL 슬러그로 덱을 읽는다.
func (s *Store) GetDeckBySlug(ctx context.Context, userID uuid.UUID, slug string) (model.Deck, error) {
	seq, err := model.DecodeDeckSlug(slug)
	if err != nil {
		return model.Deck{}, model.ErrNotFound
	}
	return scanDeck(s.pool.QueryRow(ctx, deckSelect+` where d.user_id = $1 and d.seq = $2`, userID, seq))
}

// tag::deck-id-by-slug[]
// DeckIDBySlug는 덱 슬러그를 내부 덱 id로 바꾸며, 호출자의 소유·존재 확인을
// 겸한다.
func (s *Store) DeckIDBySlug(ctx context.Context, userID uuid.UUID, slug string) (uuid.UUID, error) {
	seq, err := model.DecodeDeckSlug(slug)
	if err != nil {
		return uuid.Nil, model.ErrNotFound
	}
	var id uuid.UUID
	err = s.pool.QueryRow(ctx,
		`select id from decks where user_id = $1 and seq = $2`, userID, seq).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, model.ErrNotFound
	}
	return id, err
}

// end::deck-id-by-slug[]

func (s *Store) CreateDeck(ctx context.Context, userID uuid.UUID, name string, description *string) (model.Deck, error) {
	return scanDeck(s.pool.QueryRow(ctx,
		`with ins as (
		   insert into decks (user_id, name, description) values ($1, $2, $3)
		   returning id, name, description, created_at, updated_at, seq
		 )
		 select id, name, description, 0, null::text, null::timestamptz, created_at, updated_at, seq from ins`,
		userID, name, description))
}

func (s *Store) UpdateDeck(ctx context.Context, userID, deckID uuid.UUID, name *string, description *string) (model.Deck, error) {
	err := requireRowAffected(s.pool.Exec(ctx,
		`update decks set
		   name = coalesce($3, name),
		   description = coalesce($4, description),
		   updated_at = now()
		 where user_id = $1 and id = $2`,
		userID, deckID, name, description))
	if err != nil {
		return model.Deck{}, err
	}
	return s.GetDeck(ctx, userID, deckID)
}

func (s *Store) DeleteDeck(ctx context.Context, userID, deckID uuid.UUID) error {
	return requireRowAffected(s.pool.Exec(ctx,
		`delete from decks where user_id = $1 and id = $2`, userID, deckID))
}

// DeckStory는 덱의 스토리 원문(마크다운)을 읽는다. 없으면 nil이다.
func (s *Store) DeckStory(ctx context.Context, userID, deckID uuid.UUID) (*string, error) {
	var story *string
	err := s.pool.QueryRow(ctx,
		`select story from decks where user_id = $1 and id = $2`, userID, deckID).Scan(&story)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	return story, err
}

func (s *Store) UpdateDeckStory(ctx context.Context, userID, deckID uuid.UUID, story *string) error {
	return requireRowAffected(s.pool.Exec(ctx,
		`update decks set story = $3, updated_at = now()
		 where user_id = $1 and id = $2`,
		userID, deckID, story))
}
