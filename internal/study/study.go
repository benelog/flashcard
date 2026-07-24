// Package study decides which cards a study session will run through.
//
// 화면(internal/web)과 JSON API(internal/handlers)는 같은 학습을 서로 다른 옷을
// 입혀 내놓는다. 어느 카드가 어떤 순서로 나오는지는 두 곳에서 같아야 하므로 그
// 판단만 여기 모았고, 제목·되돌아갈 주소·오류를 어떤 모양으로 알릴지는 부르는
// 쪽에 남겼다.
//
// model.Store만 보므로 HTTP 없이 단위 테스트할 수 있다.
package study

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/model"
	"github.com/benelog/flashcard/internal/smartrules"
)

// 하루 학습량(due 모드가 한 번에 내놓는 카드 수)의 정책 숫자. 설정 화면의
// 기본값·상한(internal/web)과 JSON API의 limit 보정(internal/handlers)이 이
// 한 벌을 같이 쓴다.
const (
	DefaultDailyGoal = 50
	MaxDailyGoal     = 200
)

// Request is one caller's ask. 모드마다 쓰는 필드가 다르다.
type Request struct {
	Mode      string
	DeckID    *uuid.UUID      // model.ModeDeck
	Rule      json.RawMessage // model.ModeSmart
	DueBefore time.Time       // model.ModeDue: 이 시각까지 만기인 카드까지
	Limit     int             // model.ModeDue
}

// Plan is the picked card list plus the values that go on the session row.
type Plan struct {
	Mode   string
	Cards  []model.Card
	DeckID *uuid.UUID      // 덱 학습일 때만
	Rule   json.RawMessage // 스마트 학습일 때만, 정규화된 규칙
}

// 잘못된 요청은 종류별 sentinel로 구분해 돌려준다. 화면은 넷 다 404 페이지로,
// JSON API는 400으로 옮기므로 "어떻게 알릴지"는 부르는 쪽 몫이다.
var (
	ErrUnknownMode  = errors.New("study: unknown mode")
	ErrDeckRequired = errors.New("study: deck mode needs a deck id")
	ErrRuleRequired = errors.New("study: smart mode needs a rule")
	ErrInvalidRule  = errors.New("study: invalid rule")
)

// Pick loads the cards for req. 돌려주는 error가 위 sentinel 중 하나면 요청이
// 잘못된 것이고, 그 밖의 error는 저장소 실패다.
func Pick(ctx context.Context, s model.Store, userID uuid.UUID, req Request) (Plan, error) {
	plan := Plan{Mode: req.Mode}
	var err error
	switch req.Mode {
	case model.ModeDeck:
		if req.DeckID == nil {
			return plan, ErrDeckRequired
		}
		plan.DeckID = req.DeckID
		plan.Cards, err = s.ListCards(ctx, userID, *req.DeckID)
		if err == nil {
			// 덱에 담긴 순서대로 외워 버리지 않도록 섞는다.
			rand.Shuffle(len(plan.Cards), func(i, j int) {
				plan.Cards[i], plan.Cards[j] = plan.Cards[j], plan.Cards[i]
			})
		}
	case model.ModeDue:
		plan.Cards, err = s.DueCards(ctx, userID, req.DueBefore, req.Limit)
	case model.ModeSmart:
		if len(req.Rule) == 0 {
			return plan, ErrRuleRequired
		}
		rule, perr := smartrules.Parse(req.Rule)
		if perr != nil {
			return plan, fmt.Errorf("%w: %w", ErrInvalidRule, perr)
		}
		// 규칙은 세션에 적기 전에 정규화한다. 기본값이 채워지고 한계가 잘려
		// 나가므로, 나중에 같은 규칙을 다시 실행해도 같은 카드가 나온다.
		plan.Rule, _ = json.Marshal(rule)
		plan.Cards, err = s.CardsByRule(ctx, userID, rule)
	default:
		return plan, ErrUnknownMode
	}
	if err != nil {
		return Plan{Mode: req.Mode}, err
	}
	return plan, nil
}

// Suggestion is one home-screen tile: a canned rule and how many cards it
// matches right now.
type Suggestion struct {
	Rule  smartrules.Rule
	Count int
}

// Suggestions keeps only the canned rules that have cards at the moment. 빈
// 타일을 권하지 않기 위해서다. 홈 화면과 JSON API의 /suggestions가 같은 목록을
// 내놓도록 세는 일은 여기 한 번만 적는다.
func Suggestions(ctx context.Context, s model.Store, userID uuid.UUID) ([]Suggestion, error) {
	found := []Suggestion{}
	for _, rule := range smartrules.Suggested() {
		n, err := s.CountByRule(ctx, userID, rule)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			found = append(found, Suggestion{Rule: rule, Count: n})
		}
	}
	return found, nil
}
