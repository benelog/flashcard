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
// parseClaims verifies a Supabase access token and hands back its claims.
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

// parseUserID validates a Supabase access token and returns its subject.
func parseUserID(raw string, kf jwt.Keyfunc) (uuid.UUID, error) {
	claims, err := parseClaims(raw, kf)
	if err != nil {
		return uuid.Nil, err
	}
	return userIDFrom(claims)
}

// tag::parse-user[]
// ParseUser validates a Supabase access token and returns its subject and
// email claim. Used by the web layer, which carries the token in a cookie
// instead of the Authorization header.
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

// SetUserID stores the authenticated user id on the request context, under
// the same key the API middleware uses, so handlers and EnsureProfile work
// identically for cookie-authenticated web requests.
func SetUserID(c *gin.Context, id uuid.UUID) {
	c.Set(userIDKey, id)
}

// Middleware validates the Supabase access token and stores the user id.
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

// OptionalMiddleware attaches the user id when a valid token is present but
// never rejects the request, so public endpoints can still personalize their
// response (e.g. an "is mine" flag) for signed-in callers.
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

// OptionalUserID returns the caller's id, or uuid.Nil when unauthenticated.
func OptionalUserID(c *gin.Context) uuid.UUID {
	if v, ok := c.Get(userIDKey); ok {
		return v.(uuid.UUID)
	}
	return uuid.Nil
}
