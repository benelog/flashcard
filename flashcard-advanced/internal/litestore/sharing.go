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

// ShareDeck은 공유를 켜되 기존 슬러그를 유지해 링크가 안 바뀌게 한다. 짧은 공유
// 슬러그는 전역 유일이라 드물게 충돌하면 새 값으로 재시도한다. coalesce 덕에
// 이미 공유된 덱은 제 슬러그를 다시 쓰므로 충돌할 수 없다.
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

// isUniqueViolation은 err가 SQLite의 unique 제약 위반인지 알려 준다. ShareDeck의
// 재시도 루프가 삼켜도 되는 실패는 이것뿐이다. 다른 제약(FK, NOT NULL) 위반까지
// 삼키면 재시도 끝에 엉뚱한 slug 오류로 보고된다.
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

// sharedDeckBySlug는 sharedDeckSelect에 이어 붙는 조건 조각이다. SQL 키워드가
// 없어 메서드 단위 문장 대조(sqlsync_test.go)에 걸리지 않으므로, 이름 있는
// 상수로 두어 상수 대조 쪽에 태운다.
const sharedDeckBySlug = ` and d.share_slug = ?`

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

// ListSharedDecks는 공개 갤러리를 최신순으로 돌려준다.
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
		sharedDeckSelect+sharedDeckBySlug, viewerID.String(), slug))
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

// ImportSharedDeck은 공유 덱과 그 카드들을 보는 이의 계정으로 한 트랜잭션 안에서
// 복제한다. SRS 상태는 새로 시작한다.
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

	// 복사본마다 새 uuid가 필요한데 SQLite는 만들지 못하므로, insert ... select
	// 대신 Go(insertCard)에서 행 단위로 복제한다.
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

// scanCardInput은 카드 행을 insertCard가 받는 모양으로 되읽는다. 카드를 다른
// 덱으로 복제할 때 쓴다.
func scanCardInput(r rowScanner) (model.CardInput, error) {
	var in model.CardInput
	var tags string
	if err := r.Scan(&in.Text, &in.Meaning, &in.CardType, &tags,
		&in.Phonetic, &in.Example, &in.Notes); err != nil {
		return in, err
	}
	return in, json.Unmarshal([]byte(tags), &in.Tags)
}
