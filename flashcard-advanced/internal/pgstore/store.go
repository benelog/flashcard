// tag::pkg-doc[]
// Package pgstore는 배포된 앱이 쓰는 model.Store의 pgx 구현이다.
// 로컬 모드의 SQLite 구현(internal/litestore)과 나란히
// 같은 계약을 만족하며, 행 타입과 ErrNotFound는 둘 다 internal/model에서
// 읽어 온다. 여기 있는 것은 PostgreSQL 방언과 pgx 배선뿐이다.
package pgstore

// end::pkg-doc[]

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
// collect는 질의 결과를 슬라이스로 모으고, 어느 경로로 나가든 커서를 닫는다.
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

// scanUUID는 id 열 하나만 읽는다. id를 먼저 고르고 행은 두 번째 질의로 가져오는
// 질의들이 쓴다. internal/litestore에 같은 이름의 짝이 있다.
func scanUUID(row pgx.Row) (uuid.UUID, error) {
	var id uuid.UUID
	err := row.Scan(&id)
	return id, err
}

// requireRowAffected는 정확히 호출자의 행 하나를 건드려야 하는 Exec을 감싼다.
// internal/litestore에 같은 이유의 짝이 있다: 모든 쓰기가 user_id로 한정되므로
// "맞은 행이 없다"는 행이 없거나 남의 행이라는 뜻이고, 둘 다 밖에는 구분 없이
// ErrNotFound다. Exec의 결과 둘을 그대로 받아 호출부가 한 줄로 남는다:
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
