package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// origin은 OAuth 콜백 주소와 공유 링크를 만드는 데 쓴다. 스킴을 잘못 고르면
// 공유 링크가 http로 나가거나 로그인 리다이렉트가 아예 맞지 않는다.
func TestOrigin(t *testing.T) {
	tests := []struct {
		name    string
		forward string
		want    string
	}{
		{"평문 http", "", "http://example.com"},
		{"프록시 뒤의 https", "https", "https://example.com"},
		{"프록시가 http라고 알림", "http", "http://example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/decks/abcd", nil)
			if tt.forward != "" {
				req.Header.Set("X-Forwarded-Proto", tt.forward)
			}
			c, _ := newContext(t, req)

			if got := origin(c); got != tt.want {
				t.Errorf("origin() = %q, want %q", got, tt.want)
			}
		})
	}
}

// redirectBack은 방금 보던 화면으로 되돌린다. Referer는 브라우저가 보내는 값이라
// 믿을 수 없으므로, 다른 출처를 가리키면 안전한 기본 경로로 떨어져야 한다.
func TestRedirectBack(t *testing.T) {
	tests := []struct {
		name    string
		referer string
		want    string
	}{
		{"같은 출처면 그리로", "http://example.com/decks/abcd", "/decks/abcd"},
		{"질의 문자열도 유지", "http://example.com/study?mode=due", "/study?mode=due"},
		{"Referer 없음", "", "/decks"},
		{"다른 출처", "https://evil.example.com/decks", "/decks"},
		{"깨진 주소", "http://%zz", "/decks"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/smart-decks", nil)
			if tt.referer != "" {
				req.Header.Set("Referer", tt.referer)
			}
			c, rec := newContext(t, req)

			redirectBack(c, "/decks")
			// POST 리다이렉트에는 본문이 없어서 gin이 상태 줄을 아직 쓰지
			// 않는다. 실제 서버가 요청 끝에 하는 일을 여기서 대신한다.
			c.Writer.WriteHeaderNow()

			if rec.Code != http.StatusSeeOther {
				t.Fatalf("status = %d, want 303", rec.Code)
			}
			if got := rec.Header().Get("Location"); got != tt.want {
				t.Errorf("Location = %q, want %q", got, tt.want)
			}
		})
	}
}
