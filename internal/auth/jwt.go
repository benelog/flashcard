package auth

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const userIDKey = "auth.userID"

var (
	jwks     keyfunc.Keyfunc
	jwksOnce sync.Once
	jwksErr  error
)

func keyfuncFor(jwksURL, secret string) (jwt.Keyfunc, error) {
	if secret != "" {
		return func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrTokenSignatureInvalid
			}
			return []byte(secret), nil
		}, nil
	}
	jwksOnce.Do(func() {
		jwks, jwksErr = keyfunc.NewDefaultCtx(context.Background(), []string{jwksURL})
	})
	if jwksErr != nil {
		return nil, jwksErr
	}
	return jwks.Keyfunc, nil
}

func bearerToken(c *gin.Context) string {
	token, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
	if !ok {
		return ""
	}
	return token
}

// tag::parse-claims[]
// parseClaims는 Supabase 액세스 토큰을 검증하고 클레임을 돌려준다.
// 토큰을 받아들이는 조건은 이 한 곳에만 적는다. 헤더로 오는 API 요청과 쿠키로
// 오는 페이지 요청이 같은 문을 지나야, 한쪽에만 검사를 더하고 다른 쪽을 잊는
// 일이 생기지 않는다.
func parseClaims(raw string, kf jwt.Keyfunc) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}
	if _, err := jwt.ParseWithClaims(raw, claims, kf,
		jwt.WithValidMethods([]string{"HS256", "RS256", "ES256"}),
		jwt.WithAudience("authenticated"),
		jwt.WithExpirationRequired(),
	); err != nil {
		return nil, err
	}
	return claims, nil
}

// end::parse-claims[]

func userIDFrom(claims jwt.MapClaims) (uuid.UUID, error) {
	sub, _ := claims["sub"].(string)
	return uuid.Parse(sub)
}

// parseUserID는 토큰을 검증하고 subject(사용자 ID)만 돌려준다.
func parseUserID(raw string, kf jwt.Keyfunc) (uuid.UUID, error) {
	claims, err := parseClaims(raw, kf)
	if err != nil {
		return uuid.Nil, err
	}
	return userIDFrom(claims)
}

// tag::parse-user[]
// ParseUser는 토큰을 검증해 사용자 ID와 email 클레임을 돌려준다. 토큰을
// Authorization 헤더 대신 쿠키에 싣고 다니는 웹 계층이 쓴다.
func ParseUser(raw, jwksURL, secret string) (uuid.UUID, string, error) {
	kf, err := keyfuncFor(jwksURL, secret)
	if err != nil {
		return uuid.Nil, "", err
	}
	claims, err := parseClaims(raw, kf)
	if err != nil {
		return uuid.Nil, "", err
	}
	id, err := userIDFrom(claims)
	email, _ := claims["email"].(string)
	return id, email, err
}

// end::parse-user[]

// SetUserID는 인증된 사용자 ID를 요청 컨텍스트에 싣는다. API 미들웨어와 같은
// 키를 쓰므로, 쿠키로 인증한 웹 요청에서도 핸들러와 EnsureProfile이 똑같이
// 동작한다.
func SetUserID(c *gin.Context, id uuid.UUID) {
	c.Set(userIDKey, id)
}

// Middleware는 Supabase 액세스 토큰을 검증하고 사용자 ID를 컨텍스트에 싣는다.
func Middleware(jwksURL, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := bearerToken(c)
		if raw == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		kf, err := keyfuncFor(jwksURL, secret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "jwks unavailable"})
			return
		}
		userID, err := parseUserID(raw, kf)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set(userIDKey, userID)
		c.Next()
	}
}

// OptionalMiddleware는 유효한 토큰이 있으면 사용자 ID를 싣고, 없어도 요청을
// 거절하지 않는다. 공개 끝점이 로그인한 호출자에게만 "내 것" 표시 같은
// 개인화를 더할 수 있게 하기 위해서다.
func OptionalMiddleware(jwksURL, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if raw := bearerToken(c); raw != "" {
			if kf, err := keyfuncFor(jwksURL, secret); err == nil {
				if userID, err := parseUserID(raw, kf); err == nil {
					c.Set(userIDKey, userID)
				}
			}
		}
		c.Next()
	}
}

func UserID(c *gin.Context) uuid.UUID {
	return c.MustGet(userIDKey).(uuid.UUID)
}

// OptionalUserID는 호출자의 ID를, 미인증이면 uuid.Nil을 돌려준다.
func OptionalUserID(c *gin.Context) uuid.UUID {
	if v, ok := c.Get(userIDKey); ok {
		return v.(uuid.UUID)
	}
	return uuid.Nil
}
