package pgstore

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/model"
)

func (s *Store) ListSmartDecks(ctx context.Context, userID uuid.UUID) ([]model.SmartDeck, error) {
	rows, err := s.pool.Query(ctx,
		`select id, name, rule, created_at from smart_decks where user_id = $1 order by created_at desc`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	decks := []model.SmartDeck{}
	for rows.Next() {
		var d model.SmartDeck
		if err := rows.Scan(&d.ID, &d.Name, &d.Rule, &d.CreatedAt); err != nil {
			return nil, err
		}
		decks = append(decks, d)
	}
	return decks, rows.Err()
}

func (s *Store) CreateSmartDeck(ctx context.Context, userID uuid.UUID, name string, rule json.RawMessage) (model.SmartDeck, error) {
	var d model.SmartDeck
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
