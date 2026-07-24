package litestore

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/model"
)

// GetOrCreateProfile은 두 번째 호출부터 "Get"이다: 이미 있는 행의 이름을
// 덮어쓰면 안 된다.
func TestGetOrCreateProfileKeepsTheExistingRow(t *testing.T) {
	s, userID := testStore(t)
	ctx := context.Background()

	name := "First"
	if _, err := s.UpdateProfile(ctx, userID, &name, nil); err != nil {
		t.Fatal(err)
	}

	p, err := s.GetOrCreateProfile(ctx, userID, "Second")
	if err != nil {
		t.Fatal(err)
	}
	if p.DisplayName == nil || *p.DisplayName != "First" {
		t.Errorf("DisplayName = %v, want the original name kept", p.DisplayName)
	}
}

func TestUpdateProfile(t *testing.T) {
	s, userID := testStore(t)
	ctx := context.Background()

	name := "Learner"
	settings := json.RawMessage(`{"dailyGoal":30}`)
	p, err := s.UpdateProfile(ctx, userID, &name, settings)
	if err != nil {
		t.Fatal(err)
	}
	if p.DisplayName == nil || *p.DisplayName != "Learner" {
		t.Errorf("DisplayName = %v, want Learner", p.DisplayName)
	}
	if string(p.Settings) != `{"dailyGoal":30}` {
		t.Errorf("Settings = %s", p.Settings)
	}

	// 이름만 바꾸면 설정은 남는다(coalesce). 설정 저장과 이름 저장이 서로를
	// 지우지 않아야 설정 화면의 폼 두 값을 따로 다룰 수 있다.
	renamed := "Renamed"
	p, err = s.UpdateProfile(ctx, userID, &renamed, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(p.Settings) != `{"dailyGoal":30}` {
		t.Errorf("Settings after rename = %s, want kept", p.Settings)
	}
}

// 없는 사용자의 갱신은 ErrNotFound다. pgstore와 같은 계약이어야 화면의 404
// 처리가 두 환경에서 같이 돈다.
func TestUpdateProfileMissingUserIsNotFound(t *testing.T) {
	s, _ := testStore(t)

	name := "Nobody"
	if _, err := s.UpdateProfile(context.Background(), uuid.New(), &name, nil); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("err = %v, want model.ErrNotFound", err)
	}
}
