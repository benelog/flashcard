package model

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID         uuid.UUID       `json:"id"`
	Mode       string          `json:"mode"`
	Direction  string          `json:"direction"`
	DeckID     *uuid.UUID      `json:"deckId"`
	SmartRule  json.RawMessage `json:"smartRule"`
	TotalCards int             `json:"totalCards"`
	StartedAt  time.Time       `json:"startedAt"`
}

type ReviewOutcome struct {
	DueAt        time.Time `json:"dueAt"`
	IntervalDays float64   `json:"intervalDays"`
}

// 학습 방향. DB의 study_sessions.direction 열이 받는 값과 같다. 방향이 들어오는
// 길은 JSON API, 학습 시작 화면의 선택, 채점 폼의 hidden 필드, 지난 선택을 기억한
// 쿠키 넷인데, 어느 길로 들어오든 여기 있는 값 하나로 정해지도록 판정을 모았다.
const (
	TextToMeaning = "text_to_meaning"
	MeaningToText = "meaning_to_text"

	// DefaultDirection은 방향을 고르지 않았을 때의 값이다.
	DefaultDirection = TextToMeaning
)

// Directions는 허용하는 학습 방향 전부이며, UI가 보여 주는 순서를 따른다.
var Directions = []string{TextToMeaning, MeaningToText}

// IsDirection은 d가 허용하는 방향인지 알려 준다. JSON API 요청처럼 잘못된 값을
// 400으로 되돌려 줘야 하는 곳에서 쓴다.
func IsDirection(d string) bool {
	return slices.Contains(Directions, d)
}

// NormalizeDirection은 비었거나 모르는 입력을 DefaultDirection으로 되돌린다.
// 쿠키와 폼처럼 브라우저가 실어 오는 값에서 쓴다. 되돌려 보낼 화면이 없는
// 자리이므로, 알아볼 수 없는 값이면 기본 방향으로 학습을 이어 가는 편이 낫다.
func NormalizeDirection(d string) string {
	if IsDirection(d) {
		return d
	}
	return DefaultDirection
}

// 학습 모드. 세션이 어떤 카드 묶음을 도는지다. deck은 한 덱 전체, due는 오늘
// 복습할 카드, smart는 스마트 규칙이 고른 카드다.
const (
	ModeDeck  = "deck"
	ModeDue   = "due"
	ModeSmart = "smart"

	// DefaultMode는 모드를 지정하지 않고 학습 화면에 들어왔을 때의 값이다.
	DefaultMode = ModeDue
)
