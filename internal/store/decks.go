package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// tag::deck[]
type Deck struct {
	ID          uuid.UUID  `json:"id"`
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Description *string    `json:"description"`
	CardCount   int        `json:"cardCount"`
	ShareSlug   *string    `json:"shareSlug"`
	SharedAt    *time.Time `json:"sharedAt"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// end::deck[]

const deckSelect = `
	select d.id, d.name, d.description,
	       (select count(*) from cards c where c.deck_id = d.id) as card_count,
	       d.share_slug, d.shared_at, d.created_at, d.updated_at, d.seq
	from decks d`

func scanDeck(row pgx.Row) (Deck, error) {
	var d Deck
	var seq int64
	// tag::scan-not-found[]
	err := row.Scan(&d.ID, &d.Name, &d.Description, &d.CardCount,
		&d.ShareSlug, &d.SharedAt, &d.CreatedAt, &d.UpdatedAt, &seq)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrNotFound
	}
	// end::scan-not-found[]
	d.Slug = EncodeDeckSlug(seq)
	return d, err
}

// tag::list-decks[]
func (s *Store) ListDecks(ctx context.Context, userID uuid.UUID) ([]Deck, error) {
	rows, err := s.pool.Query(ctx, deckSelect+` where d.user_id = $1 order by d.created_at desc`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	decks := []Deck{}
	for rows.Next() {
		d, err := scanDeck(rows)
		if err != nil {
			return nil, err
		}
		decks = append(decks, d)
	}
	return decks, rows.Err()
}

// end::list-decks[]

func (s *Store) GetDeck(ctx context.Context, userID, deckID uuid.UUID) (Deck, error) {
	return scanDeck(s.pool.QueryRow(ctx, deckSelect+` where d.user_id = $1 and d.id = $2`, userID, deckID))
}

// GetDeckBySlug loads a deck by its public Base36 URL slug.
func (s *Store) GetDeckBySlug(ctx context.Context, userID uuid.UUID, slug string) (Deck, error) {
	seq, err := DecodeDeckSlug(slug)
	if err != nil {
		return Deck{}, ErrNotFound
	}
	return scanDeck(s.pool.QueryRow(ctx, deckSelect+` where d.user_id = $1 and d.seq = $2`, userID, seq))
}

// tag::deck-id-by-slug[]
// DeckIDBySlug resolves a deck slug to the internal deck id, doubling as the
// caller's ownership/existence check.
func (s *Store) DeckIDBySlug(ctx context.Context, userID uuid.UUID, slug string) (uuid.UUID, error) {
	seq, err := DecodeDeckSlug(slug)
	if err != nil {
		return uuid.Nil, ErrNotFound
	}
	var id uuid.UUID
	err = s.pool.QueryRow(ctx,
		`select id from decks where user_id = $1 and seq = $2`, userID, seq).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	return id, err
}

// end::deck-id-by-slug[]

func (s *Store) CreateDeck(ctx context.Context, userID uuid.UUID, name string, description *string) (Deck, error) {
	return scanDeck(s.pool.QueryRow(ctx,
		`with ins as (
		   insert into decks (user_id, name, description) values ($1, $2, $3)
		   returning id, name, description, created_at, updated_at, seq
		 )
		 select id, name, description, 0, null::text, null::timestamptz, created_at, updated_at, seq from ins`,
		userID, name, description))
}

func (s *Store) UpdateDeck(ctx context.Context, userID, deckID uuid.UUID, name *string, description *string) (Deck, error) {
	err := requireRowAffected(s.pool.Exec(ctx,
		`update decks set
		   name = coalesce($3, name),
		   description = coalesce($4, description),
		   updated_at = now()
		 where user_id = $1 and id = $2`,
		userID, deckID, name, description))
	if err != nil {
		return Deck{}, err
	}
	return s.GetDeck(ctx, userID, deckID)
}

func (s *Store) DeleteDeck(ctx context.Context, userID, deckID uuid.UUID) error {
	return requireRowAffected(s.pool.Exec(ctx,
		`delete from decks where user_id = $1 and id = $2`, userID, deckID))
}
