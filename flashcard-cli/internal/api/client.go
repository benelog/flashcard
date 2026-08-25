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

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New는 baseURL의 서버를 부르는 클라이언트를 만든다. token이 비어 있으면
// Authorization 헤더를 붙이지 않는다(로컬 모드는 인증이 없다).
func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
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
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
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
