package pgstore

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/benelog/flashcard/internal/model"
)

// ShareDeck은 공유를 켜되 기존 슬러그를 유지해 링크가 안 바뀌게 한다. 짧은 공유
// 슬러그는 전역 유일이라 드물게 충돌하면 새 값으로 재시도한다. coalesce 덕에
// 이미 공유된 덱은 제 슬러그를 다시 쓰므로 충돌할 수 없다.
func (s *Store) ShareDeck(ctx context.Context, userID, deckID uuid.UUID) (model.ShareInfo, error) {
	var info model.ShareInfo
	for attempt := 0; attempt < 5; attempt++ {
		err := s.pool.QueryRow(ctx,
			`update decks set
			   share_slug = coalesce(share_slug, $3),
			   shared_at = coalesce(shared_at, now())
			 where user_id = $1 and id = $2
			 returning share_slug, shared_at`,
			userID, deckID, model.NewShareSlug()).
			Scan(&info.ShareSlug, &info.SharedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return info, model.ErrNotFound
		}
		if isUniqueViolation(err) {
			continue
		}
		return info, err
	}
	return info, errors.New("could not generate a unique share slug")
}

// isUniqueViolation은 err가 Postgres의 unique 제약 위반인지 알려 준다. ShareDeck의
// 재시도 루프가 삼켜도 되는 실패는 이것뿐이다. 다른 제약(FK, NOT NULL) 위반까지
// 삼키면 재시도 끝에 엉뚱한 slug 오류로 보고된다.
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

func scanSharedDeck(row pgx.Row) (model.SharedDeckSummary, error) {
	var d model.SharedDeckSummary
	err := row.Scan(&d.ShareSlug, &d.Name, &d.Description, &d.CardCount,
		&d.OwnerName, &d.SharedAt, &d.IsMine)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, model.ErrNotFound
	}
	return d, err
}

// ListSharedDecks는 공개 갤러리를 최신순으로 돌려준다.
func (s *Store) ListSharedDecks(ctx context.Context, viewerID uuid.UUID) ([]model.SharedDeckSummary, error) {
	rows, err := s.pool.Query(ctx, sharedDeckSelect+` order by d.shared_at desc limit 100`, viewerID)
	if err != nil {
		return nil, err
	}
	return collect(rows, scanSharedDeck)
}

func (s *Store) GetSharedDeck(ctx context.Context, viewerID uuid.UUID, slug string) (model.SharedDeckSummary, error) {
	return scanSharedDeck(s.pool.QueryRow(ctx, sharedDeckSelect+` and d.share_slug = $2`, viewerID, slug))
}

func (s *Store) GetSharedDeckCards(ctx context.Context, slug string) ([]model.SharedCard, error) {
	rows, err := s.pool.Query(ctx,
		`select c.text, c.meaning, c.card_type, c.tags, c.phonetic, c.example, c.notes
		 from cards c
		 join decks d on d.id = c.deck_id
		 where d.share_slug = $1
		 order by c.created_at`, slug)
	if err != nil {
		return nil, err
	}
	return collect(rows, scanSharedCard)
}

func scanSharedCard(row pgx.Row) (model.SharedCard, error) {
	var c model.SharedCard
	err := row.Scan(&c.Text, &c.Meaning, &c.CardType, &c.Tags,
		&c.Phonetic, &c.Example, &c.Notes)
	return c, err
}

// ImportSharedDeck은 공유 덱과 그 카드들을 보는 이의 계정으로 한 트랜잭션 안에서
// 복제한다. SRS 상태는 새로 시작한다.
func (s *Store) ImportSharedDeck(ctx context.Context, viewerID uuid.UUID, slug string) (model.Deck, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return model.Deck{}, err
	}
	defer tx.Rollback(ctx)

	var srcID uuid.UUID
	var name string
	var description *string
	err = tx.QueryRow(ctx,
		`select id, name, description from decks where share_slug = $1`, slug).
		Scan(&srcID, &name, &description)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Deck{}, model.ErrNotFound
	}
	if err != nil {
		return model.Deck{}, err
	}

	var newDeckID uuid.UUID
	err = tx.QueryRow(ctx,
		`insert into decks (user_id, name, description) values ($1, $2, $3) returning id`,
		viewerID, name, description).Scan(&newDeckID)
	if err != nil {
		return model.Deck{}, err
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
		return model.Deck{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.Deck{}, err
	}
	return s.GetDeck(ctx, viewerID, newDeckID)
}
