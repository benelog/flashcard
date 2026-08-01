package litestore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/model"
)

const profileColumns = `id, display_name, settings, created_at`

func scanProfile(r rowScanner) (model.Profile, error) {
	var p model.Profile
	var id, settings, createdAt string
	err := r.Scan(&id, &p.DisplayName, &settings, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return p, model.ErrNotFound
	}
	if err != nil {
		return p, err
	}
	if p.ID, err = uuid.Parse(id); err != nil {
		return p, err
	}
	p.Settings = json.RawMessage(settings)
	p.CreatedAt, err = parseTime(createdAt)
	return p, err
}

// GetOrCreateProfile은 첫 API 접근 때 profile 행을 지연 생성한다. 로컬 모드의
// 고정 사용자는 이렇게 만들어진다.
func (s *Store) GetOrCreateProfile(ctx context.Context, userID uuid.UUID, displayName string) (model.Profile, error) {
	_, err := s.db.ExecContext(ctx,
		`insert into profiles (id, display_name, created_at) values (?, nullif(?, ''), ?)
		 on conflict (id) do nothing`,
		userID.String(), displayName, fmtTime(time.Now()))
	if err != nil {
		return model.Profile{}, err
	}
	return scanProfile(s.db.QueryRowContext(ctx,
		`select `+profileColumns+` from profiles where id = ?`, userID.String()))
}

func (s *Store) UpdateProfile(ctx context.Context, userID uuid.UUID, displayName *string, settings json.RawMessage) (model.Profile, error) {
	err := requireRowAffected(s.db.ExecContext(ctx,
		`update profiles set
		   display_name = coalesce(?, display_name),
		   settings = coalesce(?, settings)
		 where id = ?`,
		displayName, jsonArg(settings), userID.String()))
	if err != nil {
		return model.Profile{}, err
	}
	return scanProfile(s.db.QueryRowContext(ctx,
		`select `+profileColumns+` from profiles where id = ?`, userID.String()))
}
