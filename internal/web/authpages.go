package web

import (
	"log"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/auth"
)

// loginPage는 OAuth 버튼을 보여 준다. 이미 로그인한 방문자와 로그인이 없는
// local 모드는 바로 통과시킨다.
func (w *Web) loginPage(c *gin.Context) {
	next := safeNext(c.Query("next"))
	if w.cfg.AuthMode == "local" || auth.OptionalUserID(c) != uuid.Nil {
		c.Redirect(http.StatusSeeOther, next)
		return
	}
	w.render(c, http.StatusOK, "login", "로그인", gin.H{"Next": next})
}

// startOAuth는 서버 측 PKCE 흐름을 시작한다. verifier와 돌아갈 주소를 짧은
// 수명의 쿠키에 남기고 방문자를 GoTrue로 보낸다.
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

// oauthCallback은 흐름을 마무리한다. code를 토큰으로 바꿔 HttpOnly 쿠키에
// 담으므로 브라우저는 access token을 보지 못한다.
func (w *Web) oauthCallback(c *gin.Context) {
	next := safeNext(cookieValue(c, nextCookie))
	verifier := cookieValue(c, pkceCookie)
	clearCookie(c, pkceCookie)
	clearCookie(c, nextCookie)

	// GoTrue는 자기 쪽 실패(제공자 오류, 잘못된 시크릿)를 ?code= 대신
	// ?error=로 알린다. 로그에 남긴다.
	if errCode := c.Query("error"); errCode != "" {
		log.Printf("oauth callback: gotrue error %q: %s", errCode, c.Query("error_description"))
		failLogin(c, "로그인에 실패했어요. 다시 시도해주세요.")
		return
	}
	code := c.Query("code")
	if w.goTrue == nil || code == "" {
		log.Printf("oauth callback: no code in callback")
		failLogin(c, "로그인에 실패했어요. 다시 시도해주세요.")
		return
	}
	if verifier == "" {
		// 5분짜리 PKCE 쿠키가 사라졌다. 제공자 화면에 오래 머물렀거나
		// 새 로그인 시도가 덮어쓴 경우다.
		log.Printf("oauth callback: pkce cookie missing or expired")
		failLogin(c, "로그인 확인 정보가 만료됐어요. 처음부터 다시 시도해주세요.")
		return
	}
	tok, err := w.goTrue.exchangeCode(c.Request.Context(), code, verifier)
	if err != nil {
		log.Printf("oauth callback: code exchange failed: %v", err)
		failLogin(c, "로그인에 실패했어요. 다시 시도해주세요.")
		return
	}
	w.setAuthCookies(c, tok)
	c.Redirect(http.StatusSeeOther, next)
}

// failLogin은 플래시 메시지와 함께 로그인 화면으로 되돌린다.
// 원인은 부르는 쪽이 로그에 남긴다: 방문자에게 보일 문구와 운영자가 볼 기록은
// 다른 물건이다.
func failLogin(c *gin.Context, message string) {
	redirectWithFlash(c, flashError, message, "/login")
}

func (w *Web) logout(c *gin.Context) {
	if w.goTrue != nil {
		if at := cookieValue(c, accessCookie); at != "" {
			// 실패해도 무시한다. 쿠키만 지워도 브라우저는 로그아웃된다.
			_ = w.goTrue.logout(c.Request.Context(), at)
		}
	}
	w.clearAuthCookies(c)
	// signed_out은 app.js에 대한 신호다: 서비스 워커가 오프라인용으로 캐시해 둔
	// 이 사용자의 페이지 HTML을 지우게 한다.
	c.Redirect(http.StatusSeeOther, "/login?signed_out=1")
}

// origin은 프록시를 감안해 요청의 외부 기준 URL을 되살린다.
func origin(c *gin.Context) string {
	scheme := "http"
	if isHTTPS(c) {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}

// redirectBack은 폼 제출을 온 곳으로 되돌린다(PRG 패턴).
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
