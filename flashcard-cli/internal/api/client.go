// Package api는 flashcard-advanced 서버의 JSON API(/api/*)를 부르는 클라이언트다.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TokenSource는 요청마다 액세스 토큰을 내준다. 빈 문자열이면 Authorization
// 헤더를 붙이지 않는다. 저장된 로그인을 만료 전에 갱신하는 일은 여기서 한다.
type TokenSource func(ctx context.Context) (string, error)

type Client struct {
	baseURL string
	token   TokenSource
	http    *http.Client
}

// New는 baseURL의 서버를 부르는 클라이언트를 만든다. token이 비어 있으면
// Authorization 헤더를 붙이지 않는다(로컬 모드는 인증이 없다).
func New(baseURL, token string) *Client {
	return NewWithTokenSource(baseURL, func(context.Context) (string, error) { return token, nil })
}

// NewWithTokenSource는 토큰을 고정하지 않고 요청 때마다 src에 묻는 클라이언트다.
func NewWithTokenSource(baseURL string, src TokenSource) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   src,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	token, err := c.token(ctx)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return apiError(res)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// apiError는 실패 응답을 오류로 옮긴다. 서버는 {"error":"..."} 꼴로 답한다.
func apiError(res *http.Response) error {
	var body struct {
		Error string `json:"error"`
	}
	if json.NewDecoder(res.Body).Decode(&body) == nil && body.Error != "" {
		if res.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("로그인이 필요합니다(서버 응답 401: %s)", body.Error)
		}
		return fmt.Errorf("서버 응답 %d: %s", res.StatusCode, body.Error)
	}
	return fmt.Errorf("서버 응답 %d", res.StatusCode)
}

// BaseURL은 이 클라이언트가 부르는 서버 주소다. 화면에 보여 줄 때 쓴다.
func (c *Client) BaseURL() string { return c.baseURL }

func (c *Client) Healthz(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/api/healthz", nil, nil)
}

func (c *Client) ListDecks(ctx context.Context) ([]Deck, error) {
	var decks []Deck
	err := c.do(ctx, http.MethodGet, "/api/decks", nil, &decks)
	return decks, err
}

func (c *Client) GetDeck(ctx context.Context, slug string) (Deck, error) {
	var deck Deck
	err := c.do(ctx, http.MethodGet, "/api/decks/"+url.PathEscape(slug), nil, &deck)
	return deck, err
}

func (c *Client) ListDeckCards(ctx context.Context, slug string) ([]Card, error) {
	var cards []Card
	err := c.do(ctx, http.MethodGet, "/api/decks/"+url.PathEscape(slug)+"/cards", nil, &cards)
	return cards, err
}

func (c *Client) DueCount(ctx context.Context) (int, error) {
	var out struct {
		Count int `json:"count"`
	}
	err := c.do(ctx, http.MethodGet, "/api/due-count", nil, &out)
	return out.Count, err
}

func (c *Client) StartSession(ctx context.Context, req SessionRequest) (StartedSession, error) {
	var out StartedSession
	err := c.do(ctx, http.MethodPost, "/api/sessions", req, &out)
	return out, err
}

func (c *Client) RecordReview(ctx context.Context, sessionID, cardID string, result bool) (ReviewOutcome, error) {
	body := map[string]any{"cardId": cardID, "result": result}
	var out ReviewOutcome
	err := c.do(ctx, http.MethodPost, "/api/sessions/"+url.PathEscape(sessionID)+"/reviews", body, &out)
	return out, err
}

func (c *Client) FinishSession(ctx context.Context, sessionID string, completed bool) error {
	body := map[string]any{"completed": completed}
	return c.do(ctx, http.MethodPost, "/api/sessions/"+url.PathEscape(sessionID)+"/finish", body, nil)
}

// 로그인 끝점. 세 가지 모두 서버가 인증 없이 받는다.

func (c *Client) AuthConfig(ctx context.Context) (AuthConfig, error) {
	var out AuthConfig
	err := c.do(ctx, http.MethodGet, "/api/auth/config", nil, &out)
	return out, err
}

// ExchangeCode는 OAuth 콜백으로 받은 code를 PKCE verifier와 함께 토큰으로 바꾼다.
func (c *Client) ExchangeCode(ctx context.Context, code, verifier string) (TokenSet, error) {
	body := map[string]string{"code": code, "codeVerifier": verifier}
	var out TokenSet
	err := c.do(ctx, http.MethodPost, "/api/auth/exchange", body, &out)
	return out, err
}

func (c *Client) RefreshToken(ctx context.Context, refreshToken string) (TokenSet, error) {
	body := map[string]string{"refreshToken": refreshToken}
	var out TokenSet
	err := c.do(ctx, http.MethodPost, "/api/auth/refresh", body, &out)
	return out, err
}

func (c *Client) Me(ctx context.Context) (Profile, error) {
	var out Profile
	err := c.do(ctx, http.MethodGet, "/api/me", nil, &out)
	return out, err
}
