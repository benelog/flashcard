package litestore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/benelog/flashcard/internal/store"
)

// ShareDeck enables sharing, keeping any existing slug so links stay stable.
// The short share slug is globally unique, so on the rare collision we retry
// with a fresh one; coalesce means an already-shared deck reuses its slug and
// can never collide.
func (s *Store) ShareDeck(ctx context.Context, userID, deckID uuid.UUID) (store.ShareInfo, error) {
	var info store.ShareInfo
	for attempt := 0; attempt < 5; attempt++ {
		res, err := s.db.ExecContext(ctx,
			`update decks set
			   share_slug = coalesce(share_slug, ?),
			   shared_at = coalesce(shared_at, ?)
			 where user_id = ? and id = ?`,
			store.NewShareSlug(), fmtTime(time.Now()), userID.String(), deckID.String())
		if isUniqueViolation(err) {
			continue
		}
		if err := requireRowAffected(res, err); err != nil {
			return info, err
		}
		var sharedAt string
		err = s.db.QueryRowContext(ctx,
			`select share_slug, shared_at from decks where id = ?`, deckID.String()).
			Scan(&info.ShareSlug, &sharedAt)
		if err != nil {
			return info, err
		}
		info.SharedAt, err = parseTime(sharedAt)
		return info, err
	}
	return info, errors.New("could not generate a unique share slug")
}

// isUniqueViolation reports whether err is a SQLite constraint error; in
// ShareDeck's update the only constraint in play is the unique slug index.
func isUniqueViolation(err error) bool {
	var se *sqlite.Error
	return errors.As(err, &se) && se.Code()&0xff == sqlite3.SQLITE_CONSTRAINT
}

func (s *Store) UnshareDeck(ctx context.Context, userID, deckID uuid.UUID) error {
	return requireRowAffected(s.db.ExecContext(ctx,
		`update decks set share_slug = null, shared_at = null
		 where user_id = ? and id = ?`, userID.String(), deckID.String()))
}

const sharedDeckSelect = `
	select d.share_slug, d.name, d.description,
	       (select count(*) from cards c where c.deck_id = d.id) as card_count,
	       p.display_name, d.shared_at, d.user_id = ? as is_mine
	from decks d
	join profiles p on p.id = d.user_id
	where d.share_slug is not null`

func scanSharedDeck(r rowScanner) (store.SharedDeckSummary, error) {
	var d store.SharedDeckSummary
	var sharedAt string
	err := r.Scan(&d.ShareSlug, &d.Name, &d.Description, &d.CardCount,
		&d.OwnerName, &sharedAt, &d.IsMine)
	if errors.Is(err, sql.ErrNoRows) {
		return d, store.ErrNotFound
	}
	if err != nil {
		return d, err
	}
	d.SharedAt, err = parseTime(sharedAt)
	return d, err
}

// ListSharedDecks returns the public gallery, newest first.
func (s *Store) ListSharedDecks(ctx context.Context, viewerID uuid.UUID) ([]store.SharedDeckSummary, error) {
	rows, err := s.db.QueryContext(ctx,
		sharedDeckSelect+` order by d.shared_at desc limit 100`, viewerID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	decks := []store.SharedDeckSummary{}
	for rows.Next() {
		d, err := scanSharedDeck(rows)
		if err != nil {
			return nil, err
		}
		decks = append(decks, d)
	}
	return decks, rows.Err()
}

func (s *Store) GetSharedDeck(ctx context.Context, viewerID uuid.UUID, slug string) (store.SharedDeckSummary, error) {
	return scanSharedDeck(s.db.QueryRowContext(ctx,
		sharedDeckSelect+` and d.share_slug = ?`, viewerID.String(), slug))
}

func (s *Store) GetSharedDeckCards(ctx context.Context, slug string) ([]store.SharedCard, error) {
	rows, err := s.db.QueryContext(ctx,
		`select c.text, c.meaning, c.card_type, c.tags, c.phonetic, c.example, c.notes
		 from cards c
		 join decks d on d.id = c.deck_id
		 where d.share_slug = ?
		 order by c.created_at`, slug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cards := []store.SharedCard{}
	for rows.Next() {
		var c store.SharedCard
		var tags string
		if err := rows.Scan(&c.Text, &c.Meaning, &c.CardType, &tags,
			&c.Phonetic, &c.Example, &c.Notes); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(tags), &c.Tags); err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

// ImportSharedDeck clones a shared deck and its cards into the viewer's
// account with fresh SRS state, in one transaction.
func (s *Store) ImportSharedDeck(ctx context.Context, viewerID uuid.UUID, slug string) (store.Deck, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return store.Deck{}, err
	}
	defer tx.Rollback()

	var srcID, name string
	var description *string
	err = tx.QueryRowContext(ctx,
		`select id, name, description from decks where share_slug = ?`, slug).
		Scan(&srcID, &name, &description)
	if errors.Is(err, sql.ErrNoRows) {
		return store.Deck{}, store.ErrNotFound
	}
	if err != nil {
		return store.Deck{}, err
	}

	newDeckID := uuid.New()
	now := fmtTime(time.Now())
	if _, err := tx.ExecContext(ctx,
		`insert into decks (id, user_id, name, description, seq, created_at, updated_at)
		 values (?, ?, ?, ?, (select coalesce(max(seq), 0) + 1 from decks), ?, ?)`,
		newDeckID.String(), viewerID.String(), name, description, now, now); err != nil {
		return store.Deck{}, err
	}

	// Each copy needs a fresh uuid, which SQLite cannot generate, so the cards
	// are cloned row by row in Go rather than with an insert ... select.
	rows, err := tx.QueryContext(ctx,
		`select text, meaning, card_type, tags, phonetic, example, notes
		 from cards where deck_id = ? order by created_at`, srcID)
	if err != nil {
		return store.Deck{}, err
	}
	type copied struct {
		text, meaning, cardType, tags string
		phonetic, example, notes      *string
	}
	src := []copied{}
	for rows.Next() {
		var c copied
		if err := rows.Scan(&c.text, &c.meaning, &c.cardType, &c.tags,
			&c.phonetic, &c.example, &c.notes); err != nil {
			rows.Close()
			return store.Deck{}, err
		}
		src = append(src, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return store.Deck{}, err
	}
	for _, c := range src {
		cardID := uuid.New()
		if _, err := tx.ExecContext(ctx,
			`insert into cards (id, user_id, deck_id, text, meaning, card_type, tags,
			                    phonetic, example, notes, created_at, updated_at)
			 values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			cardID.String(), viewerID.String(), newDeckID.String(), c.text, c.meaning,
			c.cardType, c.tags, c.phonetic, c.example, c.notes, now, now); err != nil {
			return store.Deck{}, err
		}
		if _, err := tx.ExecContext(ctx,
			`insert into card_srs (card_id, user_id, due_at) values (?, ?, ?)`,
			cardID.String(), viewerID.String(), now); err != nil {
			return store.Deck{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return store.Deck{}, err
	}
	return s.GetDeck(ctx, viewerID, newDeckID)
}
