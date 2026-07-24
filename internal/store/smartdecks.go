package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type SmartDeck struct {
	ID        uuid.UUID       `json:"id"`
	Name      string          `json:"name"`
	Rule      json.RawMessage `json:"rule"`
	CreatedAt time.Time       `json:"createdAt"`
}

func (s *Store) ListSmartDecks(ctx context.Context, userID uuid.UUID) ([]SmartDeck, error) {
	rows, err := s.pool.Query(ctx,
		`select id, name, rule, created_at from smart_decks where user_id = $1 order by created_at desc`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	decks := []SmartDeck{}
	for rows.Next() {
		var d SmartDeck
		if err := rows.Scan(&d.ID, &d.Name, &d.Rule, &d.CreatedAt); err != nil {
			return nil, err
		}
		decks = append(decks, d)
	}
	return decks, rows.Err()
}

func (s *Store) CreateSmartDeck(ctx context.Context, userID uuid.UUID, name string, rule json.RawMessage) (SmartDeck, error) {
	var d SmartDeck
	err := s.pool.QueryRow(ctx,
		`insert into smart_decks (user_id, name, rule) values ($1, $2, $3)
		 returning id, name, rule, created_at`, userID, name, rule).
		Scan(&d.ID, &d.Name, &d.Rule, &d.CreatedAt)
	return d, err
}

func (s *Store) DeleteSmartDeck(ctx context.Context, userID, id uuid.UUID) error {
	return requireRowAffected(s.pool.Exec(ctx,
		`delete from smart_decks where user_id = $1 and id = $2`, userID, id))
}
