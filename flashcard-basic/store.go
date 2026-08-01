package main

import (
	"database/sql"
	_ "embed"
	"errors"

	_ "modernc.org/sqlite"
)

// tag::types[]
// Deck은 decks 테이블의 한 행이다.
type Deck struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Card는 cards 테이블의 한 행이다.
type Card struct {
	ID     int64  `json:"id"`
	DeckID int64  `json:"deck_id"`
	Front  string `json:"front"`
	Back   string `json:"back"`
}

// end::types[]

// ErrNotFound는 찾는 행이 없을 때 돌려주는 에러다.
var ErrNotFound = errors.New("not found")

//go:embed schema.sql
var schemaSQL string

// tag::open[]
// Store는 SQLite 연결 하나를 감싼다.
type Store struct {
	db *sql.DB
}

// OpenStore는 SQLite 파일을 열고 스키마를 적용한다.
// 파일이 없으면 새로 만들어지므로 별도의 준비 절차가 없다.
func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// end::open[]

func (s *Store) Close() error { return s.db.Close() }

// tag::list-decks[]
// ListDecks는 모든 덱을 만든 순서대로 돌려준다.
func (s *Store) ListDecks() ([]Deck, error) {
	rows, err := s.db.Query("select id, name from decks order by id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var decks []Deck
	for rows.Next() {
		var d Deck
		if err := rows.Scan(&d.ID, &d.Name); err != nil {
			return nil, err
		}
		decks = append(decks, d)
	}
	return decks, rows.Err()
}

// end::list-decks[]

// tag::create-deck[]
// CreateDeck은 덱 하나를 추가하고 새 행의 id를 돌려준다.
func (s *Store) CreateDeck(name string) (int64, error) {
	res, err := s.db.Exec("insert into decks (name) values (?)", name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// end::create-deck[]

// tag::get-deck[]
// GetDeck은 id로 덱 하나를 찾는다. 없으면 ErrNotFound를 돌려준다.
func (s *Store) GetDeck(id int64) (Deck, error) {
	var d Deck
	err := s.db.QueryRow("select id, name from decks where id = ?", id).
		Scan(&d.ID, &d.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return Deck{}, ErrNotFound
	}
	return d, err
}

// end::get-deck[]

// tag::list-cards[]
// ListCards는 한 덱의 카드를 만든 순서대로 돌려준다.
func (s *Store) ListCards(deckID int64) ([]Card, error) {
	rows, err := s.db.Query(
		"select id, deck_id, front, back from cards where deck_id = ? order by id",
		deckID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []Card
	for rows.Next() {
		var c Card
		if err := rows.Scan(&c.ID, &c.DeckID, &c.Front, &c.Back); err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

// end::list-cards[]

// tag::create-card[]
// CreateCard는 카드 하나를 추가하고 저장된 행을 그대로 돌려준다.
func (s *Store) CreateCard(deckID int64, front, back string) (Card, error) {
	res, err := s.db.Exec(
		"insert into cards (deck_id, front, back) values (?, ?, ?)",
		deckID, front, back)
	if err != nil {
		return Card{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Card{}, err
	}
	return Card{ID: id, DeckID: deckID, Front: front, Back: back}, nil
}

// end::create-card[]

// tag::delete-card[]
// DeleteCard는 카드 하나를 지운다. 지운 행이 없으면 ErrNotFound를 돌려준다.
func (s *Store) DeleteCard(id int64) error {
	res, err := s.db.Exec("delete from cards where id = ?", id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// end::delete-card[]
