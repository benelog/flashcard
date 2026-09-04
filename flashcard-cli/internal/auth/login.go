package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	"github.com/benelog/flashcard-cli/internal/api"
)

// CallbackPort는 브라우저가 code를 돌려줄 localhost 포트다. Supabase의
// Redirect URLs에 http://localhost:45678/callback 이 등록돼 있어야 한다.
// 포트를 고정한 이유는 그 목록에 정확한 주소를 적기 위해서다.
const CallbackPort = 45678

// callbackAddr은 테스트가 빈 포트를 쓰도록 변수로 둔다.
var callbackAddr = fmt.Sprintf("127.0.0.1:%d", CallbackPort)

// Providers는 서버가 지원하는 OAuth 제공자다.
var Providers = []string{"github", "google"}

var ErrNoLogin = errors.New("이 서버는 로그인이 없습니다(로컬 모드)")

// Flow는 진행 중인 로그인이다. Begin이 브라우저에 열 주소를 만들고 콜백
// 서버를 띄운다. Wait는 브라우저가 돌아올 때까지 기다렸다가 토큰으로 바꾼다.
type Flow struct {
	URL string // 브라우저에 열 authorize 주소

	redirect string
	verifier string
	client   *api.Client
	server   *http.Server
	result   chan callbackResult
}

type callbackResult struct {
	code string
	err  error
}

// Begin은 로그인을 시작한다. 콜백 포트가 이미 쓰이고 있으면 실패한다.
func Begin(ctx context.Context, client *api.Client, provider string) (*Flow, error) {
	cfg, err := client.AuthConfig(ctx)
	if err != nil {
		return nil, err
	}
	if cfg.AuthMode != "supabase" {
		return nil, ErrNoLogin
	}
	ln, err := net.Listen("tcp", callbackAddr)
	if err != nil {
		return nil, fmt.Errorf("콜백 포트 %s를 열 수 없습니다: %w", callbackAddr, err)
	}

	f := &Flow{
		redirect: "http://localhost:" + portOf(ln.Addr()) + "/callback",
		verifier: newVerifier(),
		client:   client,
		result:   make(chan callbackResult, 1),
	}
	f.URL = authorizeURL(cfg.SupabaseURL, provider, f.redirect, challenge(f.verifier))
	f.server = &http.Server{Handler: http.HandlerFunc(f.handleCallback), ReadHeaderTimeout: 10 * time.Second}
	go f.server.Serve(ln) //nolint:errcheck // Close 뒤의 ErrServerClosed뿐이다
	return f, nil
}

// RedirectURL은 Supabase에 등록해야 하는 콜백 주소다.
func (f *Flow) RedirectURL() string { return f.redirect }

// Wait는 브라우저가 code를 돌려줄 때까지 기다렸다가 토큰으로 바꾼다. ctx가
// 끝나면 로그인을 접는다.
func (f *Flow) Wait(ctx context.Context) (Credentials, error) {
	defer f.server.Close()
	select {
	case <-ctx.Done():
		return Credentials{}, fmt.Errorf("로그인을 기다리다 그만뒀습니다: %w", ctx.Err())
	case r := <-f.result:
		if r.err != nil {
			return Credentials{}, r.err
		}
		tok, err := f.client.ExchangeCode(ctx, r.code, f.verifier)
		if err != nil {
			return Credentials{}, err
		}
		return FromTokenSet(tok), nil
	}
}

func (f *Flow) handleCallback(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/callback" {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var res callbackResult
	if e := q.Get("error"); e != "" {
		res.err = fmt.Errorf("로그인에 실패했습니다: %s %s", e, q.Get("error_description"))
		fmt.Fprint(w, page("로그인에 실패했습니다", "터미널로 돌아가 다시 시도하세요."))
	} else if code := q.Get("code"); code != "" {
		res.code = code
		fmt.Fprint(w, page("로그인이 끝났습니다", "이 창을 닫고 터미널로 돌아가세요."))
	} else {
		http.Error(w, "code가 없습니다", http.StatusBadRequest)
		return
	}
	// 첫 응답만 받는다. 새로 고침으로 또 오면 버린다.
	select {
	case f.result <- res:
	default:
	}
}

func page(title, body string) string {
	return "<!doctype html><meta charset=utf-8><title>" + title + "</title>" +
		"<body style=\"font-family:sans-serif;padding:3rem\"><h1>" + title + "</h1><p>" + body + "</p>"
}

func portOf(addr net.Addr) string {
	_, port, _ := net.SplitHostPort(addr.String())
	return port
}

// authorizeURL은 서버의 웹 로그인이 만드는 것과 같은 GoTrue authorize 주소다.
func authorizeURL(supabaseURL, provider, redirectTo, challenge string) string {
	q := url.Values{
		"provider":              {provider},
		"redirect_to":           {redirectTo},
		"code_challenge":        {challenge},
		"code_challenge_method": {"s256"},
	}
	return supabaseURL + "/auth/v1/authorize?" + q.Encode()
}

// newVerifier는 무작위 PKCE code verifier다(RFC 7636).
func newVerifier() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// OpenBrowser는 기본 브라우저로 url을 연다. 열 수 없으면 오류를 돌려주고,
// 부르는 쪽이 주소를 화면에 보여 준다.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
