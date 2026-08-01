package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// goTrue는 Supabase Auth REST API의 최소 클라이언트다. OAuth 흐름이 전부 서버
// 사이에서 오가므로 브라우저는 토큰을 보지 못하고 HttpOnly 쿠키만 나른다.
type goTrue struct {
	baseURL string // https://<ref>.supabase.co
	anonKey string
	hc      *http.Client
}

func newGoTrue(baseURL, anonKey string) *goTrue {
	return &goTrue{
		baseURL: baseURL,
		anonKey: anonKey,
		hc:      &http.Client{Timeout: 10 * time.Second},
	}
}

// tokenResponse는 GoTrue /token 응답에서 쓰는 부분만 담는다.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// newPKCEVerifier는 무작위 PKCE code verifier를 만든다(RFC 7636).
func newPKCEVerifier() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// tag::authorize-url[]
// authorizeURL은 로그인 버튼이 가리키는 곳이다. GoTrue가 제공자(Google/GitHub)로
// 넘겼다가 ?code=를 붙여 redirectTo로 되돌린다.
func (g *goTrue) authorizeURL(provider, redirectTo, challenge string) string {
	q := url.Values{
		"provider":              {provider},
		"redirect_to":           {redirectTo},
		"code_challenge":        {challenge},
		"code_challenge_method": {"s256"},
	}
	return g.baseURL + "/auth/v1/authorize?" + q.Encode()
}

// end::authorize-url[]

func (g *goTrue) token(ctx context.Context, grantType string, body any) (tokenResponse, error) {
	var tok tokenResponse
	payload, err := json.Marshal(body)
	if err != nil {
		return tok, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		g.baseURL+"/auth/v1/token?grant_type="+grantType, bytes.NewReader(payload))
	if err != nil {
		return tok, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", g.anonKey)
	res, err := g.hc.Do(req)
	if err != nil {
		return tok, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return tok, fmt.Errorf("gotrue %s: status %d: %s", grantType, res.StatusCode, body)
	}
	if err := json.NewDecoder(res.Body).Decode(&tok); err != nil {
		return tok, err
	}
	if tok.AccessToken == "" || tok.RefreshToken == "" {
		return tok, fmt.Errorf("gotrue %s: empty token response", grantType)
	}
	return tok, nil
}

// exchangeCode는 콜백의 authorization code를 세션으로 바꾼다.
func (g *goTrue) exchangeCode(ctx context.Context, code, verifier string) (tokenResponse, error) {
	return g.token(ctx, "pkce", map[string]string{
		"auth_code":     code,
		"code_verifier": verifier,
	})
}

func (g *goTrue) refresh(ctx context.Context, refreshToken string) (tokenResponse, error) {
	return g.token(ctx, "refresh_token", map[string]string{
		"refresh_token": refreshToken,
	})
}

// logout은 서버 쪽 세션을 무효화한다. 쿠키를 지우는 일은 부르는 쪽 몫이다.
func (g *goTrue) logout(ctx context.Context, accessToken string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		g.baseURL+"/auth/v1/logout", nil)
	if err != nil {
		return err
	}
	req.Header.Set("apikey", g.anonKey)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	res, err := g.hc.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	return nil
}
