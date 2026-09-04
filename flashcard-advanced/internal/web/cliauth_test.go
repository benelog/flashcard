package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/benelog/flashcard/internal/config"
)

// CLI 로그인 끝점은 GoTrue를 대신 불러 준다. 가짜 GoTrue를 세워 두고, 서버가
// apikey를 붙여 code·verifier를 그대로 넘기고 토큰을 되돌려 주는지 본다.
func TestCLIAuthEndpoints(t *testing.T) {
	gotrue := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("apikey") != "anon" {
			t.Errorf("apikey = %q, want anon", r.Header.Get("apikey"))
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		switch r.URL.RawQuery {
		case "grant_type=pkce":
			if body["auth_code"] != "code1" || body["code_verifier"] != "verifier1" {
				t.Errorf("pkce body = %v", body)
			}
			w.Write([]byte(`{"access_token":"at1","refresh_token":"rt1","expires_in":3600}`))
		case "grant_type=refresh_token":
			if body["refresh_token"] != "rt1" {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error":"invalid_grant"}`))
				return
			}
			w.Write([]byte(`{"access_token":"at2","refresh_token":"rt2","expires_in":3600}`))
		default:
			t.Errorf("unexpected gotrue call: %s", r.URL)
		}
	}))
	defer gotrue.Close()

	cfg := &config.Config{AuthMode: "supabase", SupabaseURL: gotrue.URL, SupabaseAnonKey: "anon"}
	r := gin.New()
	New(cfg, nil).Register(r)

	do := func(method, path, body string) (*httptest.ResponseRecorder, map[string]any) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		var out map[string]any
		json.Unmarshal(rec.Body.Bytes(), &out)
		return rec, out
	}

	rec, out := do(http.MethodGet, "/api/auth/config", "")
	if rec.Code != 200 || out["authMode"] != "supabase" || out["supabaseUrl"] != gotrue.URL {
		t.Errorf("config = %d %v", rec.Code, out)
	}

	rec, out = do(http.MethodPost, "/api/auth/exchange", `{"code":"code1","codeVerifier":"verifier1"}`)
	if rec.Code != 200 || out["accessToken"] != "at1" || out["refreshToken"] != "rt1" {
		t.Errorf("exchange = %d %v", rec.Code, out)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("토큰 응답에 no-store가 없다")
	}

	rec, _ = do(http.MethodPost, "/api/auth/exchange", `{"code":"code1"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("verifier 없는 exchange = %d, want 400", rec.Code)
	}

	rec, out = do(http.MethodPost, "/api/auth/refresh", `{"refreshToken":"rt1"}`)
	if rec.Code != 200 || out["accessToken"] != "at2" {
		t.Errorf("refresh = %d %v", rec.Code, out)
	}

	rec, _ = do(http.MethodPost, "/api/auth/refresh", `{"refreshToken":"stale"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("잘못된 refresh = %d, want 401", rec.Code)
	}
}

// 로컬 모드는 로그인이 없다. config는 그렇게 알리고, 교환·갱신은 404다.
func TestCLIAuthLocalMode(t *testing.T) {
	r := gin.New()
	New(&config.Config{AuthMode: "local"}, nil).Register(r)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/config", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"authMode":"local"`) {
		t.Errorf("config = %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/auth/exchange", strings.NewReader(`{"code":"c","codeVerifier":"v"}`))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("local exchange = %d, want 404", rec.Code)
	}
}
