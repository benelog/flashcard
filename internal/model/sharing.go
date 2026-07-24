package model

import (
	"crypto/rand"
	"math/big"
	"strings"
	"time"
)

type ShareInfo struct {
	ShareSlug string    `json:"shareSlug"`
	SharedAt  time.Time `json:"sharedAt"`
}

type SharedDeckSummary struct {
	ShareSlug   string    `json:"shareSlug"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	CardCount   int       `json:"cardCount"`
	OwnerName   *string   `json:"ownerName"`
	SharedAt    time.Time `json:"sharedAt"`
	IsMine      bool      `json:"isMine"`
}

// SharedCard is the card payload exposed to non-owners: content only, no ids
// or SRS state.
type SharedCard struct {
	Text     string   `json:"text"`
	Meaning  string   `json:"meaning"`
	CardType string   `json:"cardType"`
	Tags     []string `json:"tags"`
	Phonetic *string  `json:"phonetic"`
	Example  *string  `json:"example"`
	Notes    *string  `json:"notes"`
}

// A share slug is not a secret — the /shared gallery lists every shared deck
// publicly — so it needs no entropy for secrecy; it only has to be globally
// unique. It's a random 5-char Base36 token (same case-insensitive alphabet as
// the deck slug); the rare collision is retried against the unique index in
// ShareDeck.
const shareSlugLen = 5

var shareSlugSpace = new(big.Int).Exp(big.NewInt(36), big.NewInt(shareSlugLen), nil)

func NewShareSlug() string {
	n, err := rand.Int(rand.Reader, shareSlugSpace)
	if err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	s := n.Text(36) // lowercase Base36
	if len(s) < shareSlugLen {
		s = strings.Repeat("0", shareSlugLen-len(s)) + s
	}
	return s
}
