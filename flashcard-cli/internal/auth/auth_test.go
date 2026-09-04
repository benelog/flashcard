package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/benelog/flashcard-cli/internal/api"
)

// RFC 7636 부록 B의 공식 예시 값. 서버 쪽 gotrue_test와 같은 벡터다.
func TestChallengeMatchesRFC7636(t *testing.T) {
	got := challenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk")
	if want := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"; got != want {
		t.Errorf("challenge() = %q, want %q", got, want)
	}
}

func TestStore(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "sub", "credentials.json"))

	if c, err := s.Load("https://a"); err != nil || c != nil {
		t.Fatalf("빈 저장소 Load = %v, %v", c, err)
	}
	want := Credentials{AccessToken: "at", RefreshToken: "rt", ExpiresAt: time.Now().Add(time.Hour).Round(0)}
	if err := s.Save("https://a", want); err != nil {
		t.Fatal(err)
	}
	if err := s.Save("https://b", Credentials{AccessToken: "other"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load("https://a")
	if err != nil || got == nil || got.AccessToken != "at" || !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Fatalf("Load = %+v, %v", got, err)
	}
	if ok, err := s.Delete("https://a"); err != nil || !ok {
		t.Fatalf("Delete = %v, %v", ok, err)
	}
	if ok, _ := s.Delete("https://a"); ok {
		t.Error("두 번째 Delete가 true다")
	}
	if got, _ := s.Load("https://b"); got == nil || got.AccessToken != "other" {
		t.Error("다른 서버의 로그인이 사라졌다")
	}
}

// 만료가 가까운 토큰은 갱신해 저장하고, 넉넉한 토큰은 그대로 쓴다.
func TestTokenSourceRefreshes(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/refresh" {
			t.Errorf("unexpected %s", r.URL)
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("갱신 요청에 Authorization이 붙었다")
		}
		calls++
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["refreshToken"] != "rt-old" {
			t.Errorf("refreshToken = %q", body["refreshToken"])
		}
		w.Write([]byte(`{"accessToken":"at-new","refreshToken":"rt-new","expiresIn":3600}`))
	}))
	defer srv.Close()

	s := NewStore(filepath.Join(t.TempDir(), "credentials.json"))
	s.Save(srv.URL, Credentials{AccessToken: "at-old", RefreshToken: "rt-old", ExpiresAt: time.Now().Add(10 * time.Second)})
	src := s.TokenSource(srv.URL)

	tok, err := src(context.Background())
	if err != nil || tok != "at-new" {
		t.Fatalf("첫 호출 = %q, %v", tok, err)
	}
	tok, err = src(context.Background())
	if err != nil || tok != "at-new" || calls != 1 {
		t.Fatalf("둘째 호출 = %q, %v (갱신 %d번)", tok, err, calls)
	}
	if c, _ := s.Load(srv.URL); c == nil || c.RefreshToken != "rt-new" {
		t.Error("갱신한 토큰이 저장되지 않았다")
	}

	// 로그인이 없으면 빈 토큰이다(로컬 서버처럼 인증 없는 곳을 위해).
	if tok, err := s.TokenSource("https://none")(context.Background()); err != nil || tok != "" {
		t.Errorf("로그인 없음 = %q, %v", tok, err)
	}
}

// Begin이 만든 주소로 브라우저가 갔다가 code를 들고 콜백으로 돌아오는 흐름을
// 서버 없이 흉내 낸다. 서버는 code와 verifier가 challenge와 맞는지 본다.
func TestLoginFlow(t *testing.T) {
	callbackAddr = "127.0.0.1:0"

	var seenChallenge string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/auth/config":
			w.Write([]byte(`{"authMode":"supabase","supabaseUrl":"https://ref.supabase.co"}`))
		case "/api/auth/exchange":
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if body["code"] != "code1" || challenge(body["codeVerifier"]) != seenChallenge {
				t.Errorf("exchange body = %v", body)
			}
			w.Write([]byte(`{"accessToken":"at","refreshToken":"rt","expiresIn":3600}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	f, err := Begin(ctx, api.New(srv.URL, ""), "github")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(f.URL)
	if err != nil || u.Host != "ref.supabase.co" || u.Path != "/auth/v1/authorize" {
		t.Fatalf("authorize URL = %q", f.URL)
	}
	q := u.Query()
	if q.Get("provider") != "github" || q.Get("redirect_to") != f.RedirectURL() || q.Get("code_challenge_method") != "s256" {
		t.Errorf("authorize query = %v", q)
	}
	seenChallenge = q.Get("code_challenge")

	// 브라우저가 돌아온다.
	res, err := http.Get(f.RedirectURL() + "?code=code1")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Errorf("callback status = %d", res.StatusCode)
	}

	creds, err := f.Wait(ctx)
	if err != nil || creds.AccessToken != "at" || creds.RefreshToken != "rt" {
		t.Fatalf("Wait = %+v, %v", creds, err)
	}
	if time.Until(creds.ExpiresAt) < 59*time.Minute {
		t.Errorf("ExpiresAt이 이르다: %v", creds.ExpiresAt)
	}
}

func TestLoginFlowErrors(t *testing.T) {
	callbackAddr = "127.0.0.1:0"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"authMode":"supabase","supabaseUrl":"https://ref.supabase.co"}`))
	}))
	defer srv.Close()
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"authMode":"local"}`))
	}))
	defer local.Close()
	ctx := context.Background()

	// 로컬 모드 서버에는 로그인이 없다.
	if _, err := Begin(ctx, api.New(local.URL, ""), "github"); !errors.Is(err, ErrNoLogin) {
		t.Errorf("로컬 모드 Begin = %v, want ErrNoLogin", err)
	}

	// 제공자가 오류를 돌려보낸 경우.
	f, err := Begin(ctx, api.New(srv.URL, ""), "google")
	if err != nil {
		t.Fatal(err)
	}
	res, _ := http.Get(f.RedirectURL() + "?error=access_denied&error_description=nope")
	res.Body.Close()
	if _, err := f.Wait(ctx); err == nil || !strings.Contains(err.Error(), "access_denied") {
		t.Errorf("제공자 오류 = %v", err)
	}

	// 기다리다 취소한 경우.
	f, _ = Begin(ctx, api.New(srv.URL, ""), "google")
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := f.Wait(cctx); err == nil || !strings.Contains(err.Error(), "그만뒀") {
		t.Errorf("취소 = %v", err)
	}
}
