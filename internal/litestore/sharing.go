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

	"github.com/benelog/flashcard/internal/model"
)

// ShareDeck enables sharing, keeping any existing slug so links stay stable.
// The short share slug is globally unique, so on the rare collision we retry
// with a fresh one; coalesce means an already-shared deck reuses its slug and
// can never collide.
func (s *Store) ShareDeck(ctx context.Context, userID, deckID uuid.UUID) (model.ShareInfo, error) {
	var info model.ShareInfo
	for attempt := 0; attempt < 5; attempt++ {
		res, err := s.db.ExecContext(ctx,
			`update decks set
			   share_slug = coalesce(share_slug, ?),
			   shared_at = coalesce(shared_at, ?)
			 where user_id = ? and id = ?`,
			model.NewShareSlug(), fmtTime(time.Now()), userID.String(), deckID.String())
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

// isUniqueViolation reports whether err is a SQLite unique-constraint error,
// the one failure ShareDeck's retry loop may absorb. 다른 제약(FK, NOT NULL)
// 위반까지 여기서 삼키면 재시도 끝에 엉뚱한 slug 오류로 보고된다.
func isUniqueViolation(err error) bool {
	var se *sqlite.Error
	return errors.As(err, &se) && se.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
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

func scanSharedDeck(r rowScanner) (model.SharedDeckSummary, error) {
	var d model.SharedDeckSummary
	var sharedAt string
	err := r.Scan(&d.ShareSlug, &d.Name, &d.Description, &d.CardCount,
		&d.OwnerName, &sharedAt, &d.IsMine)
	if errors.Is(err, sql.ErrNoRows) {
		return d, model.ErrNotFound
	}
	if err != nil {
		return d, err
	}
	d.SharedAt, err = parseTime(sharedAt)
	return d, err
}

// ListSharedDecks returns the public gallery, newest first.
func (s *Store) ListSharedDecks(ctx context.Context, viewerID uuid.UUID) ([]model.SharedDeckSummary, error) {
	rows, err := s.db.QueryContext(ctx,
		sharedDeckSelect+` order by d.shared_at desc limit 100`, viewerID.String())
	if err != nil {
		return nil, err
	}
	return collect(rows, scanSharedDeck)
}

func (s *Store) GetSharedDeck(ctx context.Context, viewerID uuid.UUID, slug string) (model.SharedDeckSummary, error) {
	return scanSharedDeck(s.db.QueryRowContext(ctx,
		sharedDeckSelect+` and d.share_slug = ?`, viewerID.String(), slug))
}

func (s *Store) GetSharedDeckCards(ctx context.Context, slug string) ([]model.SharedCard, error) {
	rows, err := s.db.QueryContext(ctx,
		`select c.text, c.meaning, c.card_type, c.tags, c.phonetic, c.example, c.notes
		 from cards c
		 join decks d on d.id = c.deck_id
		 where d.share_slug = ?
		 order by c.created_at`, slug)
	if err != nil {
		return nil, err
	}
	return collect(rows, scanSharedCard)
}

func scanSharedCard(r rowScanner) (model.SharedCard, error) {
	var c model.SharedCard
	var tags string
	if err := r.Scan(&c.Text, &c.Meaning, &c.CardType, &tags,
		&c.Phonetic, &c.Example, &c.Notes); err != nil {
		return c, err
	}
	return c, json.Unmarshal([]byte(tags), &c.Tags)
}

// ImportSharedDeck clones a shared deck and its cards into the viewer's
// account with fresh SRS state, in one transaction.
func (s *Store) ImportSharedDeck(ctx context.Context, viewerID uuid.UUID, slug string) (model.Deck, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Deck{}, err
	}
	defer tx.Rollback()

	var srcID, name string
	var description *string
	err = tx.QueryRowContext(ctx,
		`select id, name, description from decks where share_slug = ?`, slug).
		Scan(&srcID, &name, &description)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Deck{}, model.ErrNotFound
	}
	if err != nil {
		return model.Deck{}, err
	}

	newDeckID := uuid.New()
	now := fmtTime(time.Now())
	if _, err := tx.ExecContext(ctx, insertDeckSQL,
		newDeckID.String(), viewerID.String(), name, description, now, now); err != nil {
		return model.Deck{}, err
	}

	// Each copy needs a fresh uuid, which SQLite cannot generate, so the cards
	// are cloned row by row in Go (insertCard) rather than with an
	// insert ... select.
	rows, err := tx.QueryContext(ctx,
		`select text, meaning, card_type, tags, phonetic, example, notes
		 from cards where deck_id = ? order by created_at`, srcID)
	if err != nil {
		return model.Deck{}, err
	}
	src, err := collect(rows, scanCardInput)
	if err != nil {
		return model.Deck{}, err
	}
	for _, in := range src {
		in.DeckID = newDeckID
		if _, err := insertCard(ctx, tx, viewerID, in, now); err != nil {
			return model.Deck{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return model.Deck{}, err
	}
	return s.GetDeck(ctx, viewerID, newDeckID)
}

// scanCardInput reads a card row back into the shape insertCard takes, for
// cloning cards into another deck.
func scanCardInput(r rowScanner) (model.CardInput, error) {
	var in model.CardInput
	var tags string
	if err := r.Scan(&in.Text, &in.Meaning, &in.CardType, &tags,
		&in.Phonetic, &in.Example, &in.Notes); err != nil {
		return in, err
	}
	return in, json.Unmarshal([]byte(tags), &in.Tags)
}
