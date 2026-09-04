package web

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/auth"
)

// 브라우저에 남기는 상태는 전부 쿠키다: 로그인 상태, 지난 학습 방향, 한 번만
// 보여 줄 알림, 방문자의 시간대. 이 파일이 그 쿠키들을 읽고 쓰는 유일한 곳이다.
//
// 파일 이름을 session.go로 두지 않은 것은 이 저장소에서 "세션"이 이미 세 가지를
// 뜻하기 때문이다. 카드 한 벌을 도는 학습 세션(study_sessions), 로그인 상태,
// 그리고 DB 연결이다.
//
// 쿠키 이름들. 세션 토큰은 HttpOnly라 페이지 스크립트(주입된 스크립트 포함)가
// 읽을 수 없다. localStorage 대신 쿠키를 쓰는 가장 큰 이유다.
const (
	accessCookie  = "fc_access"
	refreshCookie = "fc_refresh"
	pkceCookie    = "fc_pkce"
	nextCookie    = "fc_next"
	flashCookie   = "fc_flash"
	dirCookie     = "fc_direction"
	tzCookie      = "tz"
)

const (
	refreshMaxAge   = 30 * 24 * 60 * 60  // 이 앱이 정한 리프레시 쿠키 수명. Supabase 토큰 자체는 회전만 하고 기본 설정에서는 만료되지 않는다
	dirCookieMaxAge = 180 * 24 * 60 * 60 // 지난번에 고른 학습 방향은 오래 기억해 둔다
	emailKey        = "web.email"
)

// isHTTPS는 원 요청이 TLS로 왔는지 알린다(직접 또는 Vercel 프록시 경유).
// 쿠키의 Secure 플래그가 여기에 달렸다.
func isHTTPS(c *gin.Context) bool {
	return c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
}

func setCookie(c *gin.Context, name, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    url.QueryEscape(value),
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   isHTTPS(c),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearCookie(c *gin.Context, name string) {
	setCookie(c, name, "", -1)
}

func cookieValue(c *gin.Context, name string) string {
	v, err := c.Cookie(name)
	if err != nil {
		return ""
	}
	return v
}

func (w *Web) setAuthCookies(c *gin.Context, tok tokenResponse) {
	maxAge := tok.ExpiresIn - 60 // GoTrue가 만료시키기 전에 갱신하도록 여유를 둔다
	if maxAge <= 0 {
		maxAge = 300
	}
	setCookie(c, accessCookie, tok.AccessToken, maxAge)
	setCookie(c, refreshCookie, tok.RefreshToken, refreshMaxAge)
}

func (w *Web) clearAuthCookies(c *gin.Context) {
	clearCookie(c, accessCookie)
	clearCookie(c, refreshCookie)
}

// withUser는 세션 쿠키에서 방문자를 알아내고, access token이 만료됐으면
// refresh token으로 페이지 핸들러 몰래 갱신한다. 익명 방문자는 통과시키며,
// 막는 일은 requireUser가 한다.
func (w *Web) withUser() gin.HandlerFunc {
	if w.cfg.AuthMode == "local" {
		return func(c *gin.Context) {
			auth.SetUserID(c, auth.LocalUserID)
			c.Next()
		}
	}
	return func(c *gin.Context) {
		if raw := cookieValue(c, accessCookie); raw != "" {
			if id, email, err := auth.ParseUser(raw, w.cfg.JWKSURL, w.cfg.JWTSecret); err == nil {
				auth.SetUserID(c, id)
				c.Set(emailKey, email)
				c.Next()
				return
			}
		}
		// access token이 없거나 만료됐다. refresh token을 한 번만 써 본다.
		if rt := cookieValue(c, refreshCookie); rt != "" {
			if tok, err := w.goTrue.refresh(c.Request.Context(), rt); err == nil {
				w.setAuthCookies(c, tok)
				if id, email, err := auth.ParseUser(tok.AccessToken, w.cfg.JWKSURL, w.cfg.JWTSecret); err == nil {
					auth.SetUserID(c, id)
					c.Set(emailKey, email)
					c.Next()
					return
				}
			}
			w.clearAuthCookies(c)
		}
		c.Next()
	}
}

// requireUser는 익명 방문자를 가려던 곳을 기억한 채 로그인 화면으로 보낸다.
func (w *Web) requireUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		if auth.OptionalUserID(c) != uuid.Nil {
			c.Next()
			return
		}
		c.Redirect(http.StatusSeeOther, "/login?next="+url.QueryEscape(c.Request.URL.RequestURI()))
		c.Abort()
	}
}

func userEmail(c *gin.Context) string {
	if v, ok := c.Get(emailKey); ok {
		return v.(string)
	}
	return ""
}

// safeNext는 앱 안의 경로만 받아들인다. 조작된 ?next= 링크가 로그인 뒤
// 방문자를 다른 출처로 튕겨 보내지 못하게 한다. 슬래시 하나로 시작해야 하고,
// "//host"는 다른 출처다. 역슬래시도 거절하는데, 브라우저가 "/\host"의
// 역슬래시를 슬래시로 읽어 "//host"로 가기 때문이다.
func safeNext(next string) string {
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.Contains(next, `\`) {
		return "/"
	}
	return next
}

// 플래시 메시지는 짧은 수명의 쿠키로 리다이렉트 한 번을 살아남는다.

// 플래시 종류. 템플릿이 이 값을 그대로 CSS 클래스로 쓴다.
const (
	flashInfo  = "info"
	flashError = "error"
)

func setFlash(c *gin.Context, kind, message string) {
	setCookie(c, flashCookie, kind+"|"+message, 60)
}

// redirectWithFlash는 모든 폼 핸들러가 공유하는 PRG(Post/Redirect/Get)
// 마무리다. 한 번짜리 메시지를 남기고 303으로 보내므로 새로고침해도 폼이
// 다시 제출되지 않는다.
func redirectWithFlash(c *gin.Context, kind, message, path string) {
	setFlash(c, kind, message)
	c.Redirect(http.StatusSeeOther, path)
}

// takeFlash는 대기 중인 플래시 메시지를 읽고 지운다.
func takeFlash(c *gin.Context) (kind, message string) {
	raw := cookieValue(c, flashCookie)
	if raw == "" {
		return "", ""
	}
	clearCookie(c, flashCookie)
	kind, message, ok := strings.Cut(raw, "|")
	if !ok {
		return flashInfo, raw
	}
	return kind, message
}
