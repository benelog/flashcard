package main

import (
	"errors"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestDeckAndCardRoundTrip(t *testing.T) {
	store := newTestStore(t)

	deckID, err := store.CreateDeck("기본 영단어")
	if err != nil {
		t.Fatalf("CreateDeck: %v", err)
	}

	card, err := store.CreateCard(deckID, "resilient", "회복력 있는")
	if err != nil {
		t.Fatalf("CreateCard: %v", err)
	}
	if card.DeckID != deckID {
		t.Errorf("card.DeckID = %d, want %d", card.DeckID, deckID)
	}

	decks, err := store.ListDecks()
	if err != nil {
		t.Fatalf("ListDecks: %v", err)
	}
	if len(decks) != 1 || decks[0].Name != "기본 영단어" {
		t.Errorf("ListDecks = %+v, want one deck named 기본 영단어", decks)
	}

	cards, err := store.ListCards(deckID)
	if err != nil {
		t.Fatalf("ListCards: %v", err)
	}
	if len(cards) != 1 || cards[0].Text != "resilient" {
		t.Errorf("ListCards = %+v, want one card with text resilient", cards)
	}

	if err := store.DeleteCard(card.ID); err != nil {
		t.Fatalf("DeleteCard: %v", err)
	}
	if err := store.DeleteCard(card.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteCard twice = %v, want ErrNotFound", err)
	}
}

func TestGetDeckNotFound(t *testing.T) {
	store := newTestStore(t)

	if _, err := store.GetDeck(999); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetDeck(999) = %v, want ErrNotFound", err)
	}
}

func TestRenameDeck(t *testing.T) {
	store := newTestStore(t)

	deckID, err := store.CreateDeck("기본 영단어")
	if err != nil {
		t.Fatalf("CreateDeck: %v", err)
	}

	if err := store.RenameDeck(deckID, "고급 영단어"); err != nil {
		t.Fatalf("RenameDeck: %v", err)
	}
	deck, err := store.GetDeck(deckID)
	if err != nil {
		t.Fatalf("GetDeck: %v", err)
	}
	if deck.Name != "고급 영단어" {
		t.Errorf("deck.Name = %q, want 고급 영단어", deck.Name)
	}

	if err := store.RenameDeck(999, "없는 덱"); !errors.Is(err, ErrNotFound) {
		t.Errorf("RenameDeck(999) = %v, want ErrNotFound", err)
	}
}

func TestDeleteDeckCascadesToCards(t *testing.T) {
	store := newTestStore(t)

	deckID, err := store.CreateDeck("기본 영단어")
	if err != nil {
		t.Fatalf("CreateDeck: %v", err)
	}
	if _, err := store.CreateCard(deckID, "resilient", "회복력 있는"); err != nil {
		t.Fatalf("CreateCard: %v", err)
	}

	if err := store.DeleteDeck(deckID); err != nil {
		t.Fatalf("DeleteDeck: %v", err)
	}
	if _, err := store.GetDeck(deckID); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetDeck after delete = %v, want ErrNotFound", err)
	}
	cards, err := store.ListCards(deckID)
	if err != nil {
		t.Fatalf("ListCards: %v", err)
	}
	if len(cards) != 0 {
		t.Errorf("ListCards after deck delete = %+v, want empty", cards)
	}

	if err := store.DeleteDeck(deckID); !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteDeck twice = %v, want ErrNotFound", err)
	}
}
