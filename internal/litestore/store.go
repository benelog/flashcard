// Package litestore는 로컬 단일 사용자 모드를 위한 model.Store의 SQLite 구현이다.
// 행 타입과 ErrNotFound는 pgx 구현(internal/pgstore)과 똑같이 internal/model에서
// 읽어 오며, 다른 것은 SQL 방언뿐이다.
package litestore

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"github.com/benelog/flashcard/internal/model"
)

//go:embed schema.sql
var schemaSQL string

// timeLayout은 모든 timestamp 열에 쓰는 고정 폭 UTC 형식이다. 값의 폭이 모두
// 같아 사전순이 곧 시간순이 되므로 due_at 비교가 SQL의 단순 문자열 비교로
// 성립한다. SQL에는 now()를 쓰지 않고, 부르는 쪽이 time.Now().UTC()를 이
// 형식으로 만들어 바인딩한다.
const timeLayout = "2006-01-02T15:04:05.000Z"

type Store struct {
	db *sql.DB
}

// Open은 SQLite 파일을 열고(없으면 만들고) 내장 스키마를 적용한다. 스키마는
// 멱등이라 다시 열어도 안전하다. 로컬 사용자 하나에는 연결 하나면 충분하고,
// 그래야 쓰기가 직렬화된다.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create sqlite dir: %w", err)
		}
	}
	db, err := sql.Open("sqlite",
		path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(wal)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// rowScanner는 scan 헬퍼가 *sql.Row와 *sql.Rows를 함께 받게 한다.
type rowScanner interface {
	Scan(dest ...any) error
}

// collect는 질의 결과를 슬라이스로 모으고, 어느 경로로 나가든 커서를 닫는다.
// internal/pgstore에 같은 이름의 짝이 있다. 행마다 다른 것은 한 줄을 읽어 값으로
// 만드는 방법(scan)뿐이라, 그것만 인자로 받으면 루프는 한 번만 적으면 된다.
func collect[T any](rows *sql.Rows, scan func(rowScanner) (T, error)) ([]T, error) {
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

// tag::require-row-affected[]
// requireRowAffected는 정확히 호출자의 행 하나를 건드려야 하는 Exec을 감싼다.
// internal/pgstore에 같은 이유의 짝이 있다: 모든 쓰기가 user_id로 한정되므로
// "맞은 행이 없다"는 행이 없거나 남의 행이라는 뜻이고, 둘 다 밖에는 구분 없이
// ErrNotFound다.
func requireRowAffected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return model.ErrNotFound
	}
	return nil
}

// end::require-row-affected[]

// scanUUID는 id 열 하나만 읽는다. id를 먼저 고르고 행은 두 번째 질의로 가져오는
// 질의들이 쓴다. internal/pgstore에 같은 이름의 짝이 있다. SQLite는 uuid 타입이
// 없어 text로 담기므로 여기서 되돌린다.
func scanUUID(r rowScanner) (uuid.UUID, error) {
	var id string
	if err := r.Scan(&id); err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(id)
}

func fmtTime(t time.Time) string {
	return t.UTC().Format(timeLayout)
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(timeLayout, s)
}

func parseNullTime(s sql.NullString) (*time.Time, error) {
	if !s.Valid {
		return nil, nil
	}
	t, err := parseTime(s.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// tagsJSON은 tags를 cards.tags에 담는 JSON 배열로 만든다. nil도 null이 아니라
// []로 저장해야 API 응답의 tags가 null이 되지 않는다.
func tagsJSON(tags []string) string {
	if tags == nil {
		tags = []string{}
	}
	b, _ := json.Marshal(tags)
	return string(b)
}

// jsonArg는 원시 JSON을 text 바인딩 값으로 바꾸고, nil은 NULL로 보낸다.
func jsonArg(raw json.RawMessage) any {
	if raw == nil {
		return nil
	}
	return string(raw)
}
