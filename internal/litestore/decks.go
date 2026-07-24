package litestore

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/store"
)

const deckSelect = `
	select d.id, d.name, d.description,
	       (select count(*) from cards c where c.deck_id = d.id) as card_count,
	       d.share_slug, d.shared_at, d.created_at, d.updated_at, d.seq
	from decks d`

func scanDeck(r rowScanner) (store.Deck, error) {
	var d store.Deck
	var id, createdAt, updatedAt string
	var sharedAt sql.NullString
	var seq int64
	err := r.Scan(&id, &d.Name, &d.Description, &d.CardCount,
		&d.ShareSlug, &sharedAt, &createdAt, &updatedAt, &seq)
	if errors.Is(err, sql.ErrNoRows) {
		return d, store.ErrNotFound
	}
	if err != nil {
		return d, err
	}
	if d.ID, err = uuid.Parse(id); err != nil {
		return d, err
	}
	if d.SharedAt, err = parseNullTime(sharedAt); err != nil {
		return d, err
	}
	if d.CreatedAt, err = parseTime(createdAt); err != nil {
		return d, err
	}
	if d.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return d, err
	}
	d.Slug = store.EncodeDeckSlug(seq)
	return d, nil
}

func (s *Store) ListDecks(ctx context.Context, userID uuid.UUID) ([]store.Deck, error) {
	rows, err := s.db.QueryContext(ctx,
		deckSelect+` where d.user_id = ? order by d.created_at desc`, userID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	decks := []store.Deck{}
	for rows.Next() {
		d, err := scanDeck(rows)
		if err != nil {
			return nil, err
		}
		decks = append(decks, d)
	}
	return decks, rows.Err()
}

func (s *Store) GetDeck(ctx context.Context, userID, deckID uuid.UUID) (store.Deck, error) {
	return scanDeck(s.db.QueryRowContext(ctx,
		deckSelect+` where d.user_id = ? and d.id = ?`, userID.String(), deckID.String()))
}

// GetDeckBySlug loads a deck by its public Base36 URL slug.
func (s *Store) GetDeckBySlug(ctx context.Context, userID uuid.UUID, slug string) (store.Deck, error) {
	seq, err := store.DecodeDeckSlug(slug)
	if err != nil {
		return store.Deck{}, store.ErrNotFound
	}
	return scanDeck(s.db.QueryRowContext(ctx,
		deckSelect+` where d.user_id = ? and d.seq = ?`, userID.String(), seq))
}

// DeckIDBySlug resolves a deck slug to the internal deck id, doubling as the
// caller's ownership/existence check.
func (s *Store) DeckIDBySlug(ctx context.Context, userID uuid.UUID, slug string) (uuid.UUID, error) {
	seq, err := store.DecodeDeckSlug(slug)
	if err != nil {
		return uuid.Nil, store.ErrNotFound
	}
	var id string
	err = s.db.QueryRowContext(ctx,
		`select id from decks where user_id = ? and seq = ?`, userID.String(), seq).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, store.ErrNotFound
	}
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(id)
}

func (s *Store) CreateDeck(ctx context.Context, userID uuid.UUID, name string, description *string) (store.Deck, error) {
	id := uuid.New()
	now := fmtTime(time.Now())
	// max(seq)+1 stands in for the Postgres identity column; the single local
	// writer makes it race-free.
	_, err := s.db.ExecContext(ctx,
		`insert into decks (id, user_id, name, description, seq, created_at, updated_at)
		 values (?, ?, ?, ?, (select coalesce(max(seq), 0) + 1 from decks), ?, ?)`,
		id.String(), userID.String(), name, description, now, now)
	if err != nil {
		return store.Deck{}, err
	}
	return s.GetDeck(ctx, userID, id)
}

func (s *Store) UpdateDeck(ctx context.Context, userID, deckID uuid.UUID, name *string, description *string) (store.Deck, error) {
	err := requireRowAffected(s.db.ExecContext(ctx,
		`update decks set
		   name = coalesce(?, name),
		   description = coalesce(?, description),
		   updated_at = ?
		 where user_id = ? and id = ?`,
		name, description, fmtTime(time.Now()), userID.String(), deckID.String()))
	if err != nil {
		return store.Deck{}, err
	}
	return s.GetDeck(ctx, userID, deckID)
}

// tag::delete-deck[]
func (s *Store) DeleteDeck(ctx context.Context, userID, deckID uuid.UUID) error {
	return requireRowAffected(s.db.ExecContext(ctx,
		`delete from decks where user_id = ? and id = ?`, userID.String(), deckID.String()))
}

// end::delete-deck[]
