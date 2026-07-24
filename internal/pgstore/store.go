// Package pgstore is the pgx implementation of model.Store, the one the
// deployed app runs on. 로컬 모드의 SQLite 구현(internal/litestore)과 나란히
// 같은 계약을 만족하며, 행 타입과 ErrNotFound는 둘 다 internal/model에서
// 읽어 온다. 여기 있는 것은 PostgreSQL 방언과 pgx 배선뿐이다.
package pgstore

import (
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/benelog/flashcard/internal/model"
)

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
		return model.ErrNotFound
	}
	return nil
}
