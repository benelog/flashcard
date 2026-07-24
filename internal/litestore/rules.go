package litestore

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/model"
	"github.com/benelog/flashcard/internal/smartrules"
)

// ruleQuery renders a smart rule as SQLite SQL selecting matching card ids.
// smartrules.Rule.Query is Postgres-only (make_interval, tags &&, explicit
// nulls first), so the SQLite dialect lives here: day cutoffs are computed in
// Go from now, and the tags overlap becomes a json_each probe into the JSON
// tags array. The first ? is always the user id.
func ruleQuery(r smartrules.Rule, now time.Time) (sql string, args []any) {
	base := "select id from cards_with_stats where user_id = ?"
	switch r.Type {
	case smartrules.HighError:
		return base + " and attempts >= ? and error_rate >= ? order by error_rate desc, attempts desc limit ?",
			[]any{r.MinAttempts, r.MinErrorRate, r.Limit}
	case smartrules.Stale:
		cutoff := fmtTime(now.AddDate(0, 0, -r.NotReviewedDays))
		// SQLite already sorts nulls first in asc order, matching Postgres's
		// explicit nulls first.
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

// CardsByRule evaluates a smart rule and returns matching cards in rule order.
func (s *Store) CardsByRule(ctx context.Context, userID uuid.UUID, rule smartrules.Rule) ([]model.Card, error) {
	q, extra := ruleQuery(rule, time.Now())
	args := append([]any{userID.String()}, extra...)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	ids, err := collect(rows, scanCardID)
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
