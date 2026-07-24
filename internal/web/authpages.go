package web

import (
	"log"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/auth"
)

// loginPage shows the OAuth buttons. Signed-in visitors (and local mode,
// which has no login) go straight through.
func (w *Web) loginPage(c *gin.Context) {
	next := safeNext(c.Query("next"))
	if w.cfg.AuthMode == "local" || auth.OptionalUserID(c) != uuid.Nil {
		c.Redirect(http.StatusSeeOther, next)
		return
	}
	w.render(c, http.StatusOK, "login", "로그인", gin.H{"Next": next})
}

// startOAuth kicks off the server-side PKCE flow: remember the verifier and
// destination in short-lived cookies, then hand the visitor to GoTrue.
func (w *Web) startOAuth(c *gin.Context) {
	provider := c.Param("provider")
	if provider != "google" && provider != "github" {
		w.renderError(c, http.StatusNotFound, "지원하지 않는 로그인 방식이에요.")
		return
	}
	if w.goTrue == nil {
		c.Redirect(http.StatusSeeOther, "/")
		return
	}
	verifier := newPKCEVerifier()
	setCookie(c, pkceCookie, verifier, 300)
	setCookie(c, nextCookie, safeNext(c.Query("next")), 300)
	redirectTo := origin(c) + "/auth/callback"
	c.Redirect(http.StatusSeeOther, w.goTrue.authorizeURL(provider, redirectTo, pkceChallenge(verifier)))
}

// oauthCallback finishes the flow: trade the code for tokens and store them
// in HttpOnly cookies. The browser never sees an access token.
func (w *Web) oauthCallback(c *gin.Context) {
	next := safeNext(cookieValue(c, nextCookie))
	verifier := cookieValue(c, pkceCookie)
	clearCookie(c, pkceCookie)
	clearCookie(c, nextCookie)

	// GoTrue reports its own failures (provider errors, misconfigured
	// secrets) as ?error= instead of ?code=; surface them in the logs.
	if errCode := c.Query("error"); errCode != "" {
		log.Printf("oauth callback: gotrue error %q: %s", errCode, c.Query("error_description"))
		setFlash(c, "error", "로그인에 실패했어요. 다시 시도해주세요.")
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}
	code := c.Query("code")
	if w.goTrue == nil || code == "" {
		log.Printf("oauth callback: no code in callback")
		setFlash(c, "error", "로그인에 실패했어요. 다시 시도해주세요.")
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}
	if verifier == "" {
		// The 5-minute PKCE cookie is gone: the visitor lingered on the
		// provider screen, or a newer login attempt replaced it.
		log.Printf("oauth callback: pkce cookie missing or expired")
		setFlash(c, "error", "로그인 확인 정보가 만료됐어요. 처음부터 다시 시도해주세요.")
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}
	tok, err := w.goTrue.exchangeCode(c.Request.Context(), code, verifier)
	if err != nil {
		log.Printf("oauth callback: code exchange failed: %v", err)
		setFlash(c, "error", "로그인에 실패했어요. 다시 시도해주세요.")
		c.Redirect(http.StatusSeeOther, "/login")
		return
	}
	w.setAuthCookies(c, tok)
	c.Redirect(http.StatusSeeOther, next)
}

func (w *Web) logout(c *gin.Context) {
	if w.goTrue != nil {
		if at := cookieValue(c, accessCookie); at != "" {
			// Best-effort revocation; clearing cookies signs the browser out
			// regardless.
			_ = w.goTrue.logout(c.Request.Context(), at)
		}
	}
	w.clearAuthCookies(c)
	// signed_out은 app.js에 대한 신호다: 서비스 워커가 오프라인용으로 캐시해 둔
	// 이 사용자의 페이지 HTML을 지우게 한다.
	c.Redirect(http.StatusSeeOther, "/login?signed_out=1")
}

// origin rebuilds the request's external base URL, proxy-aware.
func origin(c *gin.Context) string {
	scheme := "http"
	if isHTTPS(c) {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}

// redirectBack sends a plain form post back where it came from (PRG pattern).
func redirectBack(c *gin.Context, fallback string) {
	ref := c.Request.Referer()
	if ref == "" {
		c.Redirect(http.StatusSeeOther, fallback)
		return
	}
	u, err := url.Parse(ref)
	if err != nil || u.Host != c.Request.Host {
		c.Redirect(http.StatusSeeOther, fallback)
		return
	}
	c.Redirect(http.StatusSeeOther, u.RequestURI())
}
