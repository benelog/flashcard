package litestore

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/model"
)

func (s *Store) scanProfile(ctx context.Context, userID uuid.UUID) (model.Profile, error) {
	var p model.Profile
	var id, settings, createdAt string
	err := s.db.QueryRowContext(ctx,
		`select id, display_name, settings, created_at from profiles where id = ?`,
		userID.String()).Scan(&id, &p.DisplayName, &settings, &createdAt)
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

// GetOrCreateProfile lazily creates the profile row on first API contact; in
// local mode this is what brings the fixed local user into existence.
func (s *Store) GetOrCreateProfile(ctx context.Context, userID uuid.UUID, displayName string) (model.Profile, error) {
	_, err := s.db.ExecContext(ctx,
		`insert into profiles (id, display_name, created_at) values (?, nullif(?, ''), ?)
		 on conflict (id) do nothing`,
		userID.String(), displayName, fmtTime(time.Now()))
	if err != nil {
		return model.Profile{}, err
	}
	return s.scanProfile(ctx, userID)
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
	return s.scanProfile(ctx, userID)
}
