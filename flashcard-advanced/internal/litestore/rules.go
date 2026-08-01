package litestore

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/model"
	"github.com/benelog/flashcard/internal/smartrules"
)

// ruleQuery는 스마트 규칙을 맞는 카드 id를 고르는 SQLite SQL로 옮긴다.
// internal/pgstore에 같은 이름의 짝이 있고, 둘은 서로를 모른 채 같은 규칙을
// 각자의 방언으로 옮긴다. Postgres가 DB에 맡기는 셋(make_interval, 배열 겹침
// 연산자 &&, 명시적 nulls first)이 SQLite에는 없어, 날짜 컷오프는 now를 받아
// Go에서 계산하고 태그 겹침은 JSON tags 배열을 json_each로 펼쳐 찾는다.
//
// 첫 ?는 언제나 사용자 id이고, 돌려주는 args가 그 뒤로 이어진다.
func ruleQuery(r smartrules.Rule, now time.Time) (sql string, args []any) {
	base := "select id from cards_with_stats where user_id = ?"
	switch r.Type {
	case smartrules.HighError:
		return base + " and attempts >= ? and error_rate >= ? order by error_rate desc, attempts desc limit ?",
			[]any{r.MinAttempts, r.MinErrorRate, r.Limit}
	case smartrules.Stale:
		cutoff := fmtTime(now.AddDate(0, 0, -r.NotReviewedDays))
		// SQLite는 asc에서 null을 이미 앞에 두므로 Postgres의 명시적
		// nulls first와 순서가 같다.
		return base + " and (last_reviewed_at is null or last_reviewed_at < ?) order by last_reviewed_at asc limit ?",
			[]any{cutoff, r.Limit}
	case smartrules.Tag:
		probe := " and exists (select 1 from json_each(cards_with_stats.tags) where json_each.value in (" +
			placeholders(len(r.Tags)) + "))"
		args = make([]any, 0, len(r.Tags)+1)
		for _, t := range r.Tags {
			args = append(args, t)
		}
		return base + probe + " order by created_at desc limit ?", append(args, r.Limit)
	case smartrules.Recent:
		cutoff := fmtTime(now.AddDate(0, 0, -r.AddedWithinDays))
		return base + " and created_at >= ? order by created_at desc limit ?",
			[]any{cutoff, r.Limit}
	}
	return "", nil
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

// CardsByRule은 스마트 규칙을 평가해 맞는 카드를 규칙 순서대로 돌려준다.
func (s *Store) CardsByRule(ctx context.Context, userID uuid.UUID, rule smartrules.Rule) ([]model.Card, error) {
	q, extra := ruleQuery(rule, time.Now())
	args := append([]any{userID.String()}, extra...)
	rows, err := s.db.QueryContext(ctx, q, args...)
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

	args = make([]any, 0, len(ids)+1)
	args = append(args, userID.String())
	for _, id := range ids {
		args = append(args, id.String())
	}
	rows, err = s.db.QueryContext(ctx,
		cardSelect+` where user_id = ? and id in (`+placeholders(len(ids))+`)`, args...)
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
	q, extra := ruleQuery(rule, time.Now())
	args := append([]any{userID.String()}, extra...)
	var n int
	err := s.db.QueryRowContext(ctx, "select count(*) from ("+q+") matched", args...).Scan(&n)
	return n, err
}
