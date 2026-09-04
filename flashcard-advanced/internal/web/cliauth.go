package web

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// CLI 로그인 끝점. CLI는 브라우저로 GoTrue의 OAuth(PKCE) 화면을 열고 code를
// localhost로 돌려받은 뒤, 여기로 code와 verifier를 보내 토큰으로 바꾼다.
// CLI가 GoTrue를 직접 부르지 않는 이유는 anon key를 CLI에 심지 않기
// 위해서다. 세 끝점 모두 로그인 전에 부르므로 인증 미들웨어를 거치지 않는다.
func (w *Web) registerCLIAuth(r *gin.Engine) {
	g := r.Group("/api/auth")
	g.GET("/config", w.cliAuthConfig)
	g.POST("/exchange", w.cliExchange)
	g.POST("/refresh", w.cliRefresh)
}

// cliAuthConfig는 CLI가 authorize 주소를 만드는 데 필요한 값을 알려 준다.
// Supabase URL은 웹 로그인 때 브라우저 주소창에도 드러나는 값이라 공개해도
// 된다. 로컬 모드는 로그인이 없으므로 authMode만 돌려준다.
func (w *Web) cliAuthConfig(c *gin.Context) {
	if w.goTrue == nil {
		c.JSON(http.StatusOK, gin.H{"authMode": "local"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"authMode":    "supabase",
		"supabaseUrl": w.goTrue.baseURL,
	})
}

func (w *Web) cliExchange(c *gin.Context) {
	if w.goTrue == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "login not available"})
		return
	}
	var body struct {
		Code         string `json:"code"`
		CodeVerifier string `json:"codeVerifier"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Code == "" || body.CodeVerifier == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code and codeVerifier are required"})
		return
	}
	tok, err := w.goTrue.exchangeCode(c.Request.Context(), body.Code, body.CodeVerifier)
	if err != nil {
		log.Printf("cli exchange: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "code exchange failed"})
		return
	}
	writeCLITokens(c, tok)
}

func (w *Web) cliRefresh(c *gin.Context) {
	if w.goTrue == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "login not available"})
		return
	}
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.RefreshToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refreshToken is required"})
		return
	}
	tok, err := w.goTrue.refresh(c.Request.Context(), body.RefreshToken)
	if err != nil {
		log.Printf("cli refresh: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh failed"})
		return
	}
	writeCLITokens(c, tok)
}

// writeCLITokens는 토큰을 JSON으로 낸다. 비밀이므로 어떤 캐시에도 남지 않게 한다.
func writeCLITokens(c *gin.Context, tok tokenResponse) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"accessToken":  tok.AccessToken,
		"refreshToken": tok.RefreshToken,
		"expiresIn":    tok.ExpiresIn,
	})
}
