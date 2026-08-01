package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// newContext는 쿠키 헬퍼 테스트용 gin 컨텍스트를 만든다. 이 파일의 함수들은
// 저장소도 템플릿도 보지 않으므로 요청과 응답 기록기만 있으면 된다.
func newContext(t *testing.T, req *http.Request) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	return c, rec
}

// safeNext는 로그인 후 돌아갈 주소를 정한다. 바깥 주소를 받아들이면 공격자가
// 만든 링크로 로그인한 방문자를 튕겨 보낼 수 있다(open redirect).
func TestSafeNext(t *testing.T) {
	tests := []struct {
		name  string
		given string
		want  string
	}{
		{"앱 안의 경로", "/decks", "/decks"},
		{"질의 문자열까지", "/study?mode=due", "/study?mode=due"},
		{"빈 값", "", "/"},
		{"다른 출처", "https://evil.example.com/", "/"},
		{"스킴 없는 다른 출처", "//evil.example.com/", "/"},
		{"상대 경로", "decks", "/"},
		{"역슬래시로 위장", `\\evil.example.com`, "/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := safeNext(tt.given); got != tt.want {
				t.Errorf("safeNext(%q) = %q, want %q", tt.given, got, tt.want)
			}
		})
	}
}

// 플래시는 리다이렉트 한 번을 건너 살아남고, 읽는 순간 사라져야 한다. 남아 있으면
// "저장했어요"가 다음 화면에도 다시 뜬다.
func TestFlashRoundTrip(t *testing.T) {
	c, rec := newContext(t, httptest.NewRequest(http.MethodPost, "/settings", nil))
	setFlash(c, flashInfo, "저장했어요")

	cookie := rec.Result().Cookies()[0]
	if cookie.Name != flashCookie {
		t.Fatalf("set cookie %q, want %q", cookie.Name, flashCookie)
	}

	next, nextRec := newContext(t, httptest.NewRequest(http.MethodGet, "/settings", nil))
	next.Request.AddCookie(cookie)

	kind, message := takeFlash(next)
	if kind != flashInfo || message != "저장했어요" {
		t.Fatalf("takeFlash() = %q, %q; want %q, %q", kind, message, flashInfo, "저장했어요")
	}
	cleared := nextRec.Result().Cookies()
	if len(cleared) != 1 || cleared[0].MaxAge >= 0 {
		t.Errorf("takeFlash left the cookie behind: %+v", cleared)
	}
}

func TestTakeFlashWithoutACookie(t *testing.T) {
	c, _ := newContext(t, httptest.NewRequest(http.MethodGet, "/", nil))

	kind, message := takeFlash(c)
	if kind != "" || message != "" {
		t.Errorf("takeFlash() = %q, %q; want both empty", kind, message)
	}
}

// 구분자가 없는 값은 예전 버전이 남긴 것일 수 있다. 통째로 메시지로 읽는다.
func TestTakeFlashWithoutAKind(t *testing.T) {
	c, rec := newContext(t, httptest.NewRequest(http.MethodGet, "/", nil))
	setCookie(c, flashCookie, "구분자 없는 옛 메시지", 60)

	next, _ := newContext(t, httptest.NewRequest(http.MethodGet, "/", nil))
	next.Request.AddCookie(rec.Result().Cookies()[0])

	kind, message := takeFlash(next)
	if kind != flashInfo || message != "구분자 없는 옛 메시지" {
		t.Errorf("takeFlash() = %q, %q", kind, message)
	}
}

// 메시지에 구분자가 들어가도 잘리지 않아야 한다. Cut은 첫 번째만 자른다.
func TestFlashKeepsSeparatorsInTheMessage(t *testing.T) {
	c, rec := newContext(t, httptest.NewRequest(http.MethodGet, "/", nil))
	setFlash(c, flashError, "a|b|c")

	next, _ := newContext(t, httptest.NewRequest(http.MethodGet, "/", nil))
	next.Request.AddCookie(rec.Result().Cookies()[0])

	if kind, message := takeFlash(next); kind != flashError || message != "a|b|c" {
		t.Errorf("takeFlash() = %q, %q; want %q, %q", kind, message, flashError, "a|b|c")
	}
}

// 세션 쿠키는 자바스크립트가 읽을 수 없어야 한다(HttpOnly). localStorage 대신
// 쿠키를 쓰는 가장 큰 이유다.
func TestSessionCookiesAreHttpOnly(t *testing.T) {
	c, rec := newContext(t, httptest.NewRequest(http.MethodGet, "/", nil))
	setCookie(c, accessCookie, "token", 60)

	cookie := rec.Result().Cookies()[0]
	if !cookie.HttpOnly {
		t.Error("session cookie is readable from page scripts")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", cookie.SameSite)
	}
}

// Secure 플래그는 요청이 TLS로 왔는지에 달렸다. Vercel처럼 프록시 뒤에 있으면
// 헤더로만 알 수 있어서, 이 판정이 틀리면 운영에서 쿠키가 평문으로 오간다.
func TestCookieIsSecureBehindAProxy(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	c, rec := newContext(t, req)

	setCookie(c, accessCookie, "token", 60)

	if !rec.Result().Cookies()[0].Secure {
		t.Error("cookie is not Secure although the visitor arrived over https")
	}
}

func TestCookieIsNotSecureOnPlainHTTP(t *testing.T) {
	c, rec := newContext(t, httptest.NewRequest(http.MethodGet, "/", nil))

	setCookie(c, accessCookie, "token", 60)

	if rec.Result().Cookies()[0].Secure {
		t.Error("cookie is Secure over plain http, so local development cannot log in")
	}
}
