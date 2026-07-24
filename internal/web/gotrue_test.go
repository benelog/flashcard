package web

import (
	"net/url"
	"strings"
	"testing"
)

// RFC 7636 부록 B의 공식 예시 값. 이 변환이 틀리면 GoTrue가 코드 교환을 거부해
// 운영 로그인이 통째로 실패하므로, 표준 벡터로 고정해 둔다.
func TestPKCEChallengeMatchesRFC7636(t *testing.T) {
	got := pkceChallenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk")
	want := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	if got != want {
		t.Errorf("pkceChallenge() = %q, want %q", got, want)
	}
}

func TestNewPKCEVerifier(t *testing.T) {
	a, b := newPKCEVerifier(), newPKCEVerifier()
	// 32바이트를 패딩 없는 base64url로 적으면 43자다(RFC 7636의 최소 길이).
	if len(a) != 43 || strings.ContainsAny(a, "+/=") {
		t.Errorf("verifier %q is not 43 chars of unpadded base64url", a)
	}
	if a == b {
		t.Error("two verifiers must differ")
	}
}

func TestAuthorizeURL(t *testing.T) {
	g := newGoTrue("https://ref.supabase.co", "anon")

	raw := g.authorizeURL("github", "https://app.example/auth/callback", "challenge123")

	base, query, ok := strings.Cut(raw, "?")
	if !ok || base != "https://ref.supabase.co/auth/v1/authorize" {
		t.Fatalf("authorize URL = %q", raw)
	}
	q, err := url.ParseQuery(query)
	if err != nil {
		t.Fatal(err)
	}
	if q.Get("provider") != "github" ||
		q.Get("redirect_to") != "https://app.example/auth/callback" ||
		q.Get("code_challenge") != "challenge123" ||
		q.Get("code_challenge_method") != "s256" {
		t.Errorf("authorize query = %v", q)
	}
}
