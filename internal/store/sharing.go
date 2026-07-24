package store

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type ShareInfo struct {
	ShareSlug string    `json:"shareSlug"`
	SharedAt  time.Time `json:"sharedAt"`
}

type SharedDeckSummary struct {
	ShareSlug   string    `json:"shareSlug"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	CardCount   int       `json:"cardCount"`
	OwnerName   *string   `json:"ownerName"`
	SharedAt    time.Time `json:"sharedAt"`
	IsMine      bool      `json:"isMine"`
}

// SharedCard is the card payload exposed to non-owners: content only, no ids
// or SRS state.
type SharedCard struct {
	Text     string   `json:"text"`
	Meaning  string   `json:"meaning"`
	CardType string   `json:"cardType"`
	Tags     []string `json:"tags"`
	Phonetic *string  `json:"phonetic"`
	Example  *string  `json:"example"`
	Notes    *string  `json:"notes"`
}

// A share slug is not a secret — the /shared gallery lists every shared deck
// publicly — so it needs no entropy for secrecy; it only has to be globally
// unique. It's a random 5-char Base36 token (same case-insensitive alphabet as
// the deck slug); the rare collision is retried against the unique index in
// ShareDeck.
const shareSlugLen = 5

var shareSlugSpace = new(big.Int).Exp(big.NewInt(36), big.NewInt(shareSlugLen), nil)

func NewShareSlug() string {
	n, err := rand.Int(rand.Reader, shareSlugSpace)
	if err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	s := n.Text(36) // lowercase Base36
	if len(s) < shareSlugLen {
		s = strings.Repeat("0", shareSlugLen-len(s)) + s
	}
	return s
}

// ShareDeck enables sharing, keeping any existing slug so links stay stable.
// The short share slug is globally unique, so on the rare collision we retry
// with a fresh one; coalesce means an already-shared deck reuses its slug and
// can never collide.
func (s *Store) ShareDeck(ctx context.Context, userID, deckID uuid.UUID) (ShareInfo, error) {
	var info ShareInfo
	for attempt := 0; attempt < 5; attempt++ {
		err := s.pool.QueryRow(ctx,
			`update decks set
			   share_slug = coalesce(share_slug, $3),
			   shared_at = coalesce(shared_at, now())
			 where user_id = $1 and id = $2
			 returning share_slug, shared_at`,
			userID, deckID, NewShareSlug()).
			Scan(&info.ShareSlug, &info.SharedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return info, ErrNotFound
		}
		if isUniqueViolation(err) {
			continue
		}
		return info, err
	}
	return info, errors.New("could not generate a unique share slug")
}

// isUniqueViolation reports whether err is a Postgres unique-constraint error.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (s *Store) UnshareDeck(ctx context.Context, userID, deckID uuid.UUID) error {
	return requireRowAffected(s.pool.Exec(ctx,
		`update decks set share_slug = null, shared_at = null
		 where user_id = $1 and id = $2`, userID, deckID))
}

const sharedDeckSelect = `
	select d.share_slug, d.name, d.description,
	       (select count(*) from cards c where c.deck_id = d.id) as card_count,
	       p.display_name, d.shared_at, d.user_id = $1 as is_mine
	from decks d
	join profiles p on p.id = d.user_id
	where d.share_slug is not null`

func scanSharedDeck(row pgx.Row) (SharedDeckSummary, error) {
	var d SharedDeckSummary
	err := row.Scan(&d.ShareSlug, &d.Name, &d.Description, &d.CardCount,
		&d.OwnerName, &d.SharedAt, &d.IsMine)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrNotFound
	}
	return d, err
}

// ListSharedDecks returns the public gallery, newest first.
func (s *Store) ListSharedDecks(ctx context.Context, viewerID uuid.UUID) ([]SharedDeckSummary, error) {
	rows, err := s.pool.Query(ctx, sharedDeckSelect+` order by d.shared_at desc limit 100`, viewerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	decks := []SharedDeckSummary{}
	for rows.Next() {
		d, err := scanSharedDeck(rows)
		if err != nil {
			return nil, err
		}
		decks = append(decks, d)
	}
	return decks, rows.Err()
}

func (s *Store) GetSharedDeck(ctx context.Context, viewerID uuid.UUID, slug string) (SharedDeckSummary, error) {
	return scanSharedDeck(s.pool.QueryRow(ctx, sharedDeckSelect+` and d.share_slug = $2`, viewerID, slug))
}

func (s *Store) GetSharedDeckCards(ctx context.Context, slug string) ([]SharedCard, error) {
	rows, err := s.pool.Query(ctx,
		`select c.text, c.meaning, c.card_type, c.tags, c.phonetic, c.example, c.notes
		 from cards c
		 join decks d on d.id = c.deck_id
		 where d.share_slug = $1
		 order by c.created_at`, slug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cards := []SharedCard{}
	for rows.Next() {
		var c SharedCard
		if err := rows.Scan(&c.Text, &c.Meaning, &c.CardType, &c.Tags,
			&c.Phonetic, &c.Example, &c.Notes); err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

// ImportSharedDeck clones a shared deck and its cards into the viewer's
// account with fresh SRS state, in one transaction.
func (s *Store) ImportSharedDeck(ctx context.Context, viewerID uuid.UUID, slug string) (Deck, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Deck{}, err
	}
	defer tx.Rollback(ctx)

	var srcID uuid.UUID
	var name string
	var description *string
	err = tx.QueryRow(ctx,
		`select id, name, description from decks where share_slug = $1`, slug).
		Scan(&srcID, &name, &description)
	if errors.Is(err, pgx.ErrNoRows) {
		return Deck{}, ErrNotFound
	}
	if err != nil {
		return Deck{}, err
	}

	var newDeckID uuid.UUID
	err = tx.QueryRow(ctx,
		`insert into decks (user_id, name, description) values ($1, $2, $3) returning id`,
		viewerID, name, description).Scan(&newDeckID)
	if err != nil {
		return Deck{}, err
	}

	if _, err := tx.Exec(ctx,
		`with copied as (
		   insert into cards (user_id, deck_id, text, meaning, card_type,
		                      tags, phonetic, example, notes)
		   select $1, $2, text, meaning, card_type, tags, phonetic, example, notes
		   from cards where deck_id = $3
		   returning id
		 )
		 insert into card_srs (card_id, user_id) select id, $1 from copied`,
		viewerID, newDeckID, srcID); err != nil {
		return Deck{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Deck{}, err
	}
	return s.GetDeck(ctx, viewerID, newDeckID)
}
