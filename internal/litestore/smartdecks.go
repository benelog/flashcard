package litestore

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/model"
)

func (s *Store) ListSmartDecks(ctx context.Context, userID uuid.UUID) ([]model.SmartDeck, error) {
	rows, err := s.db.QueryContext(ctx,
		`select id, name, rule, created_at from smart_decks where user_id = ? order by created_at desc`,
		userID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	decks := []model.SmartDeck{}
	for rows.Next() {
		var d model.SmartDeck
		var id, rule, createdAt string
		if err := rows.Scan(&id, &d.Name, &rule, &createdAt); err != nil {
			return nil, err
		}
		if d.ID, err = uuid.Parse(id); err != nil {
			return nil, err
		}
		d.Rule = json.RawMessage(rule)
		if d.CreatedAt, err = parseTime(createdAt); err != nil {
			return nil, err
		}
		decks = append(decks, d)
	}
	return decks, rows.Err()
}

func (s *Store) CreateSmartDeck(ctx context.Context, userID uuid.UUID, name string, rule json.RawMessage) (model.SmartDeck, error) {
	d := model.SmartDeck{ID: uuid.New(), Name: name, Rule: rule}
	now := fmtTime(time.Now())
	_, err := s.db.ExecContext(ctx,
		`insert into smart_decks (id, user_id, name, rule, created_at) values (?, ?, ?, ?, ?)`,
		d.ID.String(), userID.String(), name, jsonArg(rule), now)
	if err != nil {
		return model.SmartDeck{}, err
	}
	d.CreatedAt, err = parseTime(now)
	return d, err
}

func (s *Store) DeleteSmartDeck(ctx context.Context, userID, id uuid.UUID) error {
	return requireRowAffected(s.db.ExecContext(ctx,
		`delete from smart_decks where user_id = ? and id = ?`, userID.String(), id.String()))
}
