package api

import "time"

// 서버(flashcard-advanced)의 JSON 응답을 받는 타입들. 서버 모듈을 import하지
// 않고 JSON 계약만 본다. ID는 CLI가 해석할 일이 없어 문자열로 둔다.

type Deck struct {
	ID          string     `json:"id"`
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Description *string    `json:"description"`
	CardCount   int        `json:"cardCount"`
	ShareSlug   *string    `json:"shareSlug"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	SharedAt    *time.Time `json:"sharedAt"`
}

type Card struct {
	ID       string   `json:"id"`
	DeckID   string   `json:"deckId"`
	Text     string   `json:"text"`
	Meaning  string   `json:"meaning"`
	CardType string   `json:"cardType"`
	Tags     []string `json:"tags"`
	Phonetic *string  `json:"phonetic"`
	Example  *string  `json:"example"`
	Notes    *string  `json:"notes"`

	Attempts     int       `json:"attempts"`
	ErrorRate    float64   `json:"errorRate"`
	IntervalDays float64   `json:"intervalDays"`
	DueAt        time.Time `json:"dueAt"`
}

// NewCard는 카드를 만들 때 보내는 값이다. 서버가 정하는 ID·학습 기록은 없다.
type NewCard struct {
	Text     string   `json:"text"`
	Meaning  string   `json:"meaning"`
	CardType string   `json:"cardType,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Phonetic *string  `json:"phonetic,omitempty"`
	Example  *string  `json:"example,omitempty"`
	Notes    *string  `json:"notes,omitempty"`
}

// BulkResult는 카드 일괄 추가의 결과다. Skipped는 덱에 이미 있던 표현,
// Invalid는 표현이나 뜻이 비었거나 종류가 틀려 서버가 받지 않은 행이다.
type BulkResult struct {
	Added   int `json:"added"`
	Skipped int `json:"skipped"`
	Invalid int `json:"invalid"`
}

type Session struct {
	ID         string    `json:"id"`
	Mode       string    `json:"mode"`
	Direction  string    `json:"direction"`
	TotalCards int       `json:"totalCards"`
	StartedAt  time.Time `json:"startedAt"`
}

// StartedSession은 세션 생성 응답이다. 세션과 함께 낼 카드 목록이 온다.
type StartedSession struct {
	Session Session `json:"session"`
	Cards   []Card  `json:"cards"`
}

// SessionRequest는 세션 생성 요청이다. Mode가 "deck"이면 DeckID가 필요하다.
type SessionRequest struct {
	Mode      string `json:"mode"`
	Direction string `json:"direction,omitempty"`
	DeckID    string `json:"deckId,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

type ReviewOutcome struct {
	DueAt        time.Time `json:"dueAt"`
	IntervalDays float64   `json:"intervalDays"`
}

// 학습 방향. 서버의 study_sessions.direction 값과 같다.
const (
	TextToMeaning = "text_to_meaning"
	MeaningToText = "meaning_to_text"
)

// AuthConfig는 서버의 로그인 방식이다. authMode가 "local"이면 로그인이 없다.
type AuthConfig struct {
	AuthMode    string `json:"authMode"`
	SupabaseURL string `json:"supabaseUrl"`
}

type TokenSet struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int    `json:"expiresIn"` // 초
}

type Profile struct {
	ID          string  `json:"id"`
	DisplayName *string `json:"displayName"`
}
