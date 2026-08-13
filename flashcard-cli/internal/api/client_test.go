package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("Authorization = %q, want Bearer tok", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /api/decks":
			w.Write([]byte(`[{"slug":"a1b2","name":"Verbs","cardCount":3}]`))
		case "POST /api/sessions":
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"session":{"id":"s1","mode":"due","totalCards":1},"cards":[{"id":"c1","text":"go","meaning":"가다"}]}`))
		case "POST /api/sessions/s1/reviews":
			w.Write([]byte(`{"dueAt":"2026-08-15T00:00:00Z","intervalDays":1}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"not found"}`))
		}
	}))
	defer srv.Close()

	c := New(srv.URL+"/", "tok") // 끝의 /가 정리되는지도 함께 확인
	ctx := context.Background()

	decks, err := c.ListDecks(ctx)
	if err != nil || len(decks) != 1 || decks[0].Name != "Verbs" {
		t.Fatalf("ListDecks = %v, %v", decks, err)
	}

	started, err := c.StartSession(ctx, SessionRequest{Mode: "due"})
	if err != nil || started.Session.ID != "s1" || len(started.Cards) != 1 {
		t.Fatalf("StartSession = %v, %v", started, err)
	}

	if _, err := c.RecordReview(ctx, "s1", "c1", true); err != nil {
		t.Fatalf("RecordReview: %v", err)
	}

	// 실패 응답은 본문의 error 메시지를 담아 돌려준다.
	if _, err := c.GetDeck(ctx, "zzzz"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("GetDeck 오류 = %v, want 'not found' 포함", err)
	}
}
