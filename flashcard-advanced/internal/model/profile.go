package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Profile struct {
	ID          uuid.UUID       `json:"id"`
	DisplayName *string         `json:"displayName"`
	Settings    json.RawMessage `json:"settings"`
	CreatedAt   time.Time       `json:"createdAt"`
}

type SmartDeck struct {
	ID        uuid.UUID       `json:"id"`
	Name      string          `json:"name"`
	Rule      json.RawMessage `json:"rule"`
	CreatedAt time.Time       `json:"createdAt"`
}
