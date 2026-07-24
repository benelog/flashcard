// Package litestore is the SQLite implementation of handlers.Store for local
// single-user mode. It reuses the row types and the ErrNotFound sentinel from
// internal/store; only the SQL dialect differs.
package litestore

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/benelog/flashcard/internal/store"
)

//go:embed schema.sql
var schemaSQL string

// timeLayout is the fixed-width UTC format for every timestamp column. All
// values having the same width makes lexicographic order equal to time order,
// so due_at comparisons stay plain string comparisons in SQL. now() never
// appears in SQL; callers format time.Now().UTC() and bind it.
const timeLayout = "2006-01-02T15:04:05.000Z"

type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the SQLite file and applies the embedded
// schema, which is idempotent. A single connection is enough for one local
// user and keeps writes serialized.
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

// rowScanner lets scan helpers accept both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// tag::require-row-affected[]
// requireRowAffected wraps an Exec that must touch exactly the caller's row.
// The pgx store has the same helper for the same reason: every write is scoped
// by user_id, so "no row matched" means the row is missing or belongs to
// someone else, and both are ErrNotFound to the caller.
func requireRowAffected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

// end::require-row-affected[]

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

// tagsJSON encodes tags as the JSON array stored in cards.tags.
func tagsJSON(tags []string) string {
	if tags == nil {
		tags = []string{}
	}
	b, _ := json.Marshal(tags)
	return string(b)
}

// jsonArg converts raw JSON to a text bind value, mapping nil to NULL.
func jsonArg(raw json.RawMessage) any {
	if raw == nil {
		return nil
	}
	return string(raw)
}
