package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// tag::not-found[]
var ErrNotFound = errors.New("not found")

// end::not-found[]

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// requireRowAffected wraps an Exec that must touch exactly the caller's row.
// Every write here is scoped by user_id, so "no row matched" covers both a
// missing row and someone else's row; both are ErrNotFound to the caller, who
// must not learn the difference. Taking Exec's two results lets the call site
// stay one line:
//
//	err := requireRowAffected(s.pool.Exec(ctx, `delete from ...`, userID, id))
func requireRowAffected(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
