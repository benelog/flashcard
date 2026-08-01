package pgstore

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/benelog/flashcard/internal/model"
	"github.com/benelog/flashcard/internal/smartrules"
)

// ruleQuery는 스마트 규칙을 맞는 카드 id를 고르는 Postgres SQL로 옮긴다.
// internal/litestore에 같은 이름의 짝이 있고, 둘은 서로를 모른 채 같은 규칙을
// 각자의 방언으로 옮긴다. 규칙 자체(internal/smartrules)는 어느 DB도 모른다.
//
// 여기서는 날짜 셈과 태그 검색을 DB에 맡길 수 있다. make_interval로 "며칠 전"을
// 구하고, 배열 열에 겹침 연산자 &&를 쓰고, nulls first를 직접 지정한다. SQLite
// 쪽은 셋 다 안 되어 날짜는 Go에서 계산하고 태그는 json_each로 펼친다.
//
// $1은 언제나 사용자 id이고, 돌려주는 args가 $2부터 이어진다.
func ruleQuery(r smartrules.Rule) (sql string, args []any) {
	base := "select id from cards_with_stats where user_id = $1"
	switch r.Type {
	case smartrules.HighError:
		return base + " and attempts >= $2 and error_rate >= $3 order by error_rate desc, attempts desc limit $4",
			[]any{r.MinAttempts, r.MinErrorRate, r.Limit}
	case smartrules.Stale:
		return base + " and (last_reviewed_at is null or last_reviewed_at < now() - make_interval(days => $2)) order by last_reviewed_at asc nulls first limit $3",
			[]any{r.NotReviewedDays, r.Limit}
	case smartrules.Tag:
		return base + " and tags && $2 order by created_at desc limit $3",
			[]any{r.Tags, r.Limit}
	case smartrules.Recent:
		return base + " and created_at >= now() - make_interval(days => $2) order by created_at desc limit $3",
			[]any{r.AddedWithinDays, r.Limit}
	}
	return "", nil
}

// CardsByRule은 스마트 규칙을 평가해 맞는 카드를 규칙 순서대로 돌려준다.
func (s *Store) CardsByRule(ctx context.Context, userID uuid.UUID, rule smartrules.Rule) ([]model.Card, error) {
	q, extra := ruleQuery(rule)
	args := append([]any{userID}, extra...)
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	ids, err := collect(rows, scanUUID)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []model.Card{}, nil
	}

	// id들은 pgtype.UUID로 넘긴다. 풀이 simple protocol로 돌기 때문인데(Supabase의
	// transaction pooler), 그 모드의 pgx는 파라미터 타입을 모르는 채로는
	// []uuid.UUID 슬라이스를 text로 인코딩하지 못한다. pgtype.UUID는 pgx 자체의
	// uuid 타입이라 스스로 인코딩되고 ::uuid[] 캐스트도 필요 없다.
	pgIDs := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		pgIDs[i] = pgtype.UUID{Bytes: id, Valid: true}
	}
	rows, err = s.pool.Query(ctx, cardSelect+` where user_id = $1 and id = any($2)`, userID, pgIDs)
	if err != nil {
		return nil, err
	}
	cards, err := collect(rows, scanCard)
	if err != nil {
		return nil, err
	}
	return model.SortCardsByIDOrder(cards, ids), nil
}

func (s *Store) CountByRule(ctx context.Context, userID uuid.UUID, rule smartrules.Rule) (int, error) {
	q, extra := ruleQuery(rule)
	args := append([]any{userID}, extra...)
	var n int
	err := s.pool.QueryRow(ctx, "select count(*) from ("+q+") matched", args...).Scan(&n)
	return n, err
}
