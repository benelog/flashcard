package pgstore

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/benelog/flashcard/internal/model"
)

const profileColumns = `id, display_name, settings, created_at`

func scanProfile(row pgx.Row) (model.Profile, error) {
	var p model.Profile
	err := row.Scan(&p.ID, &p.DisplayName, &p.Settings, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, model.ErrNotFound
	}
	return p, err
}

// GetOrCreateProfile lazily creates the profile row on first API contact,
// so no Supabase-side trigger on auth.users is needed.
func (s *Store) GetOrCreateProfile(ctx context.Context, userID uuid.UUID, displayName string) (model.Profile, error) {
	_, err := s.pool.Exec(ctx,
		`insert into profiles (id, display_name) values ($1, nullif($2, '')) on conflict (id) do nothing`,
		userID, displayName)
	if err != nil {
		return model.Profile{}, err
	}
	return scanProfile(s.pool.QueryRow(ctx,
		`select `+profileColumns+` from profiles where id = $1`, userID))
}

func (s *Store) UpdateProfile(ctx context.Context, userID uuid.UUID, displayName *string, settings json.RawMessage) (model.Profile, error) {
	return scanProfile(s.pool.QueryRow(ctx,
		`update profiles set
		   display_name = coalesce($2, display_name),
		   settings = coalesce($3, settings)
		 where id = $1
		 returning `+profileColumns,
		userID, displayName, settings))
}
