// Package pgstore is the pgx implementation of model.Store, the one the
// deployed app runs on. 로컬 모드의 SQLite 구현(internal/litestore)과 나란히
// 같은 계약을 만족하며, 행 타입과 ErrNotFound는 둘 다 internal/model에서
// 읽어 온다. 여기 있는 것은 PostgreSQL 방언과 pgx 배선뿐이다.
package pgstore

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

// tag::collect[]
// collect drains a query into a slice, closing the cursor on every path.
//
// 목록을 돌려주는 함수가 이 패키지에만 여덟 개인데, 행마다 다른 것은 한 줄을
// 읽어 값으로 만드는 방법(scan)뿐이고 나머지 루프는 글자 하나까지 같았다.
// 다른 부분만 인자로 받으면 같은 부분은 한 번만 적으면 된다.
func collect[T any](rows pgx.Rows, scan func(pgx.Row) (T, error)) ([]T, error) {
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// end::collect[]

// scanUUID reads a single id column, for the queries that pick ids first and
// fetch the rows in a second pass.
func scanUUID(row pgx.Row) (uuid.UUID, error) {
	var id uuid.UUID
	err := row.Scan(&id)
	return id, err
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
