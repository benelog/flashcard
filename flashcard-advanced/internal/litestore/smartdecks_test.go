package litestore

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/model"
)

func TestSmartDeckCRUD(t *testing.T) {
	s, userID := testStore(t)
	ctx := context.Background()

	rule := json.RawMessage(`{"type":"stale","notReviewedDays":7,"limit":20}`)
	created, err := s.CreateSmartDeck(ctx, userID, "오래 안 본 카드", rule)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == uuid.Nil || created.CreatedAt.IsZero() {
		t.Errorf("created smart deck missing id or timestamp: %+v", created)
	}

	list, err := s.ListSmartDecks(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != created.ID || list[0].Name != "오래 안 본 카드" {
		t.Fatalf("list = %+v, want the created deck", list)
	}
	if string(list[0].Rule) != string(rule) {
		t.Errorf("Rule = %s, want stored as given", list[0].Rule)
	}

	// 남의 스마트 덱은 지울 수 없고, 흔적도 ErrNotFound로만 남는다.
	if err := s.DeleteSmartDeck(ctx, uuid.New(), created.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("delete as stranger: err = %v, want model.ErrNotFound", err)
	}
	if err := s.DeleteSmartDeck(ctx, userID, created.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSmartDeck(ctx, userID, created.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("second delete: err = %v, want model.ErrNotFound", err)
	}
}
