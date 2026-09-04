// Package auth는 CLI의 로그인이다. 브라우저에서 OAuth(PKCE)를 마치면 토큰을
// 사용자 설정 디렉터리에 보관하고, 만료가 가까우면 리프레시 토큰으로 갱신한다.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/benelog/flashcard-cli/internal/api"
)

// Credentials는 서버 하나에 대한 로그인이다.
type Credentials struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

// Store는 서버 주소별 로그인을 담은 JSON 파일이다. 서버를 바꿔 가며 써도
// 토큰이 섞이지 않는다.
type Store struct {
	path string
}

// DefaultStore는 사용자 설정 디렉터리(리눅스는 ~/.config)의 flashcard/credentials.json이다.
// FLASHCARD_CONFIG_DIR로 디렉터리를 바꿀 수 있다.
func DefaultStore() (*Store, error) {
	dir := os.Getenv("FLASHCARD_CONFIG_DIR")
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return nil, err
		}
		dir = filepath.Join(base, "flashcard")
	}
	return NewStore(filepath.Join(dir, "credentials.json")), nil
}

func NewStore(path string) *Store { return &Store{path: path} }

func (s *Store) Path() string { return s.path }

func (s *Store) read() (map[string]Credentials, error) {
	all := map[string]Credentials{}
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return all, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &all); err != nil {
		return nil, fmt.Errorf("%s: %w", s.path, err)
	}
	return all, nil
}

func (s *Store) write(all map[string]Credentials) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	// 토큰은 비밀이다. 파일은 소유자만 읽게 만들고, 쓰다 만 파일이 남지 않게
	// 임시 파일에 쓴 뒤 바꿔 넣는다.
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Load는 server의 로그인을 돌려준다. 없으면 nil이다.
func (s *Store) Load(server string) (*Credentials, error) {
	all, err := s.read()
	if err != nil {
		return nil, err
	}
	c, ok := all[server]
	if !ok {
		return nil, nil
	}
	return &c, nil
}

func (s *Store) Save(server string, c Credentials) error {
	all, err := s.read()
	if err != nil {
		return err
	}
	all[server] = c
	return s.write(all)
}

// Delete는 server의 로그인을 지운다. 지울 것이 있었는지 알려 준다.
func (s *Store) Delete(server string) (bool, error) {
	all, err := s.read()
	if err != nil {
		return false, err
	}
	if _, ok := all[server]; !ok {
		return false, nil
	}
	delete(all, server)
	return true, s.write(all)
}

// refreshMargin은 만료 이만큼 전부터 갱신한다. 요청이 서버에 닿는 사이에
// 만료되는 일을 막는다.
const refreshMargin = 60 * time.Second

// TokenSource는 저장된 로그인에서 액세스 토큰을 꺼내는 api.TokenSource다.
// 만료가 가까우면 갱신해 다시 저장한다. 로그인이 없으면 빈 문자열이다.
func (s *Store) TokenSource(server string) api.TokenSource {
	// 갱신 요청 자체는 토큰 없이 보낸다. 같은 소스를 쓰면 되돌이가 된다.
	bare := api.New(server, "")
	return func(ctx context.Context) (string, error) {
		c, err := s.Load(server)
		if err != nil || c == nil {
			return "", err
		}
		if time.Until(c.ExpiresAt) > refreshMargin {
			return c.AccessToken, nil
		}
		tok, err := bare.RefreshToken(ctx, c.RefreshToken)
		if err != nil {
			return "", fmt.Errorf("로그인이 만료됐습니다. 다시 로그인하세요 (%w)", err)
		}
		fresh := FromTokenSet(tok)
		if err := s.Save(server, fresh); err != nil {
			return "", err
		}
		return fresh.AccessToken, nil
	}
}

// FromTokenSet은 서버가 준 토큰 묶음을 저장할 꼴로 바꾼다.
func FromTokenSet(t api.TokenSet) Credentials {
	return Credentials{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(t.ExpiresIn) * time.Second),
	}
}
