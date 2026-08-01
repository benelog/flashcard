package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// 실제 서버를 띄우지 않고 라우팅부터 템플릿까지 한 번에 검증한다.
func newTestApp(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	store := newTestStore(t)
	r := gin.New()
	r.LoadHTMLGlob("templates/*.html")
	registerRoutes(r, store)
	return r
}

func do(r *gin.Engine, method, path string, form url.Values) *httptest.ResponseRecorder {
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRenameAndDeleteDeckOverHTTP(t *testing.T) {
	r := newTestApp(t)

	if w := do(r, "POST", "/decks", url.Values{"name": {"기본 영단어"}}); w.Code != http.StatusSeeOther {
		t.Fatalf("POST /decks = %d, want 303", w.Code)
	}

	if w := do(r, "POST", "/decks/1/rename", url.Values{"name": {"고급 영단어"}}); w.Code != http.StatusSeeOther {
		t.Errorf("POST /decks/1/rename = %d, want 303", w.Code)
	}
	if w := do(r, "GET", "/decks/1", nil); !strings.Contains(w.Body.String(), "고급 영단어") {
		t.Errorf("GET /decks/1 does not show renamed deck: %d", w.Code)
	}

	if w := do(r, "POST", "/decks/1/rename", url.Values{"name": {"  "}}); w.Code != http.StatusBadRequest {
		t.Errorf("rename with blank name = %d, want 400", w.Code)
	}
	if w := do(r, "POST", "/decks/999/rename", url.Values{"name": {"없는 덱"}}); w.Code != http.StatusNotFound {
		t.Errorf("rename missing deck = %d, want 404", w.Code)
	}

	if w := do(r, "DELETE", "/decks/1", nil); w.Code != http.StatusOK {
		t.Errorf("DELETE /decks/1 = %d, want 200", w.Code)
	}
	if w := do(r, "GET", "/decks/1", nil); w.Code != http.StatusNotFound {
		t.Errorf("GET /decks/1 after delete = %d, want 404", w.Code)
	}
	if w := do(r, "DELETE", "/decks/1", nil); w.Code != http.StatusNotFound {
		t.Errorf("DELETE /decks/1 twice = %d, want 404", w.Code)
	}
}
