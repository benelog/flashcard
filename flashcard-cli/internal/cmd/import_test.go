package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/benelog/flashcard-cli/internal/api"
)

// 있는 덱은 이름으로 찾아 쓰고, 없는 덱은 만든다. 스토리는 바꾸고, 카드는
// 서버 한도로 나눠 보낸다.
func TestImportDecks(t *testing.T) {
	var calls []string
	bulkSizes := []int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /api/decks":
			w.Write([]byte(`[{"slug":"old1","name":"Jun 1주차"}]`))
		case "POST /api/decks":
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"slug":"new1","name":"Jun 회의"}`))
		case "PUT /api/decks/old1/story":
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if !strings.Contains(body["story"], "Carl") {
				t.Errorf("story = %q", body["story"])
			}
			w.Write([]byte(`{}`))
		case "POST /api/decks/old1/cards/bulk", "POST /api/decks/new1/cards/bulk":
			var body struct{ Cards []api.NewCard }
			json.NewDecoder(r.Body).Decode(&body)
			bulkSizes = append(bulkSizes, len(body.Cards))
			fmt.Fprintf(w, `{"added":%d,"skipped":0,"invalid":0}`, len(body.Cards))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	many := make([]api.NewCard, bulkLimit+1)
	for i := range many {
		many[i] = api.NewCard{Text: fmt.Sprint("t", i), Meaning: "m"}
	}
	files := []deckFile{
		{Name: "Jun 1주차", Story: "**Carl**: Heading into work?", Cards: many},
		{Name: "Jun 회의", Cards: []api.NewCard{{Text: "a", Meaning: "b"}}},
	}
	var out bytes.Buffer
	if err := importDecks(context.Background(), api.New(srv.URL, "tok"), files, &out); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"GET /api/decks",
		"PUT /api/decks/old1/story",
		"POST /api/decks/old1/cards/bulk", "POST /api/decks/old1/cards/bulk",
		"POST /api/decks", // 스토리가 없으면 스토리를 건드리지 않는다
		"POST /api/decks/new1/cards/bulk",
	}
	if strings.Join(calls, "\n") != strings.Join(want, "\n") {
		t.Errorf("calls =\n%s\nwant\n%s", strings.Join(calls, "\n"), strings.Join(want, "\n"))
	}
	if fmt.Sprint(bulkSizes) != fmt.Sprint([]int{bulkLimit, 1, 1}) {
		t.Errorf("bulk sizes = %v", bulkSizes)
	}
	if !strings.Contains(out.String(), "기존 덱 Jun 1주차(old1): 카드 2001장 추가") ||
		!strings.Contains(out.String(), "새 덱 Jun 회의(new1)") {
		t.Errorf("output = %q", out.String())
	}
}

func TestReadDeckFileNeedsName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deck.json")
	os.WriteFile(path, []byte(`{"name":"  ","cards":[]}`), 0o600)
	if _, err := readDeckFile(path); err == nil {
		t.Error("blank name should fail")
	}
}
