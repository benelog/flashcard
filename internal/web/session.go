package web

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/auth"
)

// Cookie names. Session tokens are HttpOnly: page scripts (and any injected
// script) can never read them — the main security win over localStorage.
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
	refreshMaxAge = 30 * 24 * 60 * 60 // matches Supabase's default refresh window
	emailKey      = "web.email"
)

// isHTTPS reports whether the original request came in over TLS (directly or
// via Vercel's proxy), which decides the cookies' Secure flag.
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
	maxAge := tok.ExpiresIn - 60 // renew before GoTrue expires it
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

// withUser resolves the visitor from the session cookies and, when the access
// token has expired, renews it with the refresh token — all transparently to
// the page handlers. Anonymous visitors pass through; requireUser is the gate.
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
		// Access token missing or expired: try the refresh token once.
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

// requireUser redirects anonymous visitors to the login page, remembering
// where they were headed.
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

// safeNext only honors same-app paths, so a crafted ?next= link can't bounce
// the visitor to another origin after sign-in.
func safeNext(next string) string {
	if strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "//") {
		return next
	}
	return "/"
}

// Flash messages survive one redirect via a short-lived cookie.

// 플래시 종류. 템플릿이 이 값을 그대로 CSS 클래스로 쓴다.
const (
	flashInfo  = "info"
	flashError = "error"
)

func setFlash(c *gin.Context, kind, message string) {
	setCookie(c, flashCookie, kind+"|"+message, 60)
}

// redirectWithFlash is the PRG (Post/Redirect/Get) ending every form handler
// shares: leave a one-shot message, then send the browser somewhere it can
// safely reload. 303을 쓰므로 새로고침해도 폼이 다시 제출되지 않는다.
func redirectWithFlash(c *gin.Context, kind, message, path string) {
	setFlash(c, kind, message)
	c.Redirect(http.StatusSeeOther, path)
}

// takeFlash reads and clears the pending flash message, if any.
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
