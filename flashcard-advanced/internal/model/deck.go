package model

import (
	"time"

	"github.com/google/uuid"
)

// tag::deck[]
type Deck struct {
	ID          uuid.UUID  `json:"id"`
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Description *string    `json:"description"`
	CardCount   int        `json:"cardCount"`
	ShareSlug   *string    `json:"shareSlug"`
	SharedAt    *time.Time `json:"sharedAt"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// end::deck[]
