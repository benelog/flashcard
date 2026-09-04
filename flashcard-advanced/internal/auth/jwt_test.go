package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// 이 테스트는 HS256 비밀키 경로만 쓴다. keyfuncFor는 비밀키가 있으면 JWKS를
// 아예 보러 가지 않으므로, 네트워크 없이 "토큰을 언제 받아들이는가"를 그대로
// 확인할 수 있다.
const testSecret = "test-secret-please-ignore"

var testUserID = uuid.MustParse("11111111-2222-3333-4444-555555555555")

// signed는 주어진 claim으로 토큰을 만든다. 기본값은 통과해야 하는 토큰이고,
// 각 테스트는 어긋뜨릴 항목만 바꾼다.
func signed(t *testing.T, edit func(jwt.MapClaims)) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":   testUserID.String(),
		"aud":   "authenticated",
		"email": "benelog@example.com",
		"exp":   time.Now().Add(time.Hour).Unix(),
	}
	if edit != nil {
		edit(claims)
	}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestParseUser(t *testing.T) {
	id, email, err := ParseUser(signed(t, nil), "", testSecret)
	if err != nil {
		t.Fatal(err)
	}
	if id != testUserID {
		t.Errorf("id = %v, want %v", id, testUserID)
	}
	if email != "benelog@example.com" {
		t.Errorf("email = %q", email)
	}
}

// 토큰이 어느 한 가지라도 어긋나면 거절해야 한다. 특히 만료와 대상(aud) 검사는
// 옵션으로 켜 둔 것이라, 빠뜨리면 조용히 통과한다.
func TestParseUserRejectsBadTokens(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"빈 토큰", ""},
		{"토큰이 아님", "not.a.token"},
		{"만료됨", signed(t, func(c jwt.MapClaims) {
			c["exp"] = time.Now().Add(-time.Minute).Unix()
		})},
		{"만료 시각이 없음", signed(t, func(c jwt.MapClaims) { delete(c, "exp") })},
		{"대상이 다름", signed(t, func(c jwt.MapClaims) { c["aud"] = "anon" })},
		{"대상이 없음", signed(t, func(c jwt.MapClaims) { delete(c, "aud") })},
		{"sub가 UUID가 아님", signed(t, func(c jwt.MapClaims) { c["sub"] = "nobody" })},
		{"sub가 없음", signed(t, func(c jwt.MapClaims) { delete(c, "sub") })},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if id, _, err := ParseUser(tt.raw, "", testSecret); err == nil {
				t.Fatalf("accepted a bad token as user %v", id)
			}
		})
	}
}

// 다른 비밀키로 서명한 토큰은 통과하면 안 된다. 서명 검증이 실제로 도는지를
// 확인하는 최소한의 검사다.
func TestParseUserRejectsAForeignSignature(t *testing.T) {
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": testUserID.String(),
		"aud": "authenticated",
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte("someone-elses-secret"))
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := ParseUser(raw, "", testSecret); err == nil {
		t.Fatal("accepted a token signed with another secret")
	}
}

// alg=none은 서명이 아예 없는 토큰이다. 허용 알고리즘 목록이 이것을 막는다.
func TestParseUserRejectsUnsignedToken(t *testing.T) {
	raw, err := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"sub": testUserID.String(),
		"aud": "authenticated",
		"exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := ParseUser(raw, "", testSecret); err == nil {
		t.Fatal("accepted an unsigned (alg=none) token")
	}
}

// 쿠키로 오는 페이지 요청(ParseUser)과 헤더로 오는 API 요청(Middleware)은 같은
// 검증을 지나야 한다. 한쪽만 통과하는 토큰이 있으면 두 문의 기준이 갈라진 것이다.
func TestBothDoorsAgree(t *testing.T) {
	tokens := []string{
		signed(t, nil),
		signed(t, func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Minute).Unix() }),
		signed(t, func(c jwt.MapClaims) { c["aud"] = "anon" }),
		signed(t, func(c jwt.MapClaims) { delete(c, "exp") }),
		signed(t, func(c jwt.MapClaims) { c["sub"] = "nobody" }),
	}
	for _, raw := range tokens {
		_, _, cookieErr := ParseUser(raw, "", testSecret)

		rec := runMiddleware(t, Middleware("", testSecret), "Bearer "+raw)
		headerOK := rec.Code == http.StatusOK

		if headerOK != (cookieErr == nil) {
			t.Errorf("cookie path err = %v but header path status = %d", cookieErr, rec.Code)
		}
	}
}

func runMiddleware(t *testing.T, mw gin.HandlerFunc, authorization string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/probe", mw, func(c *gin.Context) {
		c.String(http.StatusOK, OptionalUserID(c).String())
	})

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestMiddleware(t *testing.T) {
	tests := []struct {
		name          string
		authorization string
		wantStatus    int
		wantBody      string
	}{
		{"정상 토큰", "Bearer " + signed(t, nil), http.StatusOK, testUserID.String()},
		{"헤더 없음", "", http.StatusUnauthorized, ""},
		{"Bearer 접두어 없음", signed(t, nil), http.StatusUnauthorized, ""},
		{"접두어 대소문자가 다름", "bearer " + signed(t, nil), http.StatusUnauthorized, ""},
		{"토큰이 비었음", "Bearer ", http.StatusUnauthorized, ""},
		{"깨진 토큰", "Bearer not.a.token", http.StatusUnauthorized, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := runMiddleware(t, Middleware("", testSecret), tt.authorization)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantBody != "" && rec.Body.String() != tt.wantBody {
				t.Errorf("body = %q, want %q", rec.Body, tt.wantBody)
			}
		})
	}
}

// 공개 화면용 미들웨어는 아무도 막지 않는다. 토큰이 없거나 틀렸으면 익명으로
// 통과시키고, 맞으면 신원을 붙인다.
func TestOptionalMiddleware(t *testing.T) {
	tests := []struct {
		name          string
		authorization string
		wantID        string
	}{
		{"정상 토큰", "Bearer " + signed(t, nil), testUserID.String()},
		{"헤더 없음", "", uuid.Nil.String()},
		{"틀린 토큰", "Bearer not.a.token", uuid.Nil.String()},
		{"만료된 토큰", "Bearer " + signed(t, func(c jwt.MapClaims) {
			c["exp"] = time.Now().Add(-time.Minute).Unix()
		}), uuid.Nil.String()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := runMiddleware(t, OptionalMiddleware("", testSecret), tt.authorization)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 — 공개 경로는 누구도 막지 않는다", rec.Code)
			}
			if rec.Body.String() != tt.wantID {
				t.Errorf("user = %s, want %s", rec.Body, tt.wantID)
			}
		})
	}
}

// 로컬 모드는 로그인이 없다. 늘 같은 사용자로 통과시켜야 run_local.sh가 뜬다.
func TestLocalMiddleware(t *testing.T) {
	rec := runMiddleware(t, LocalMiddleware(), "")

	if rec.Code != http.StatusOK || rec.Body.String() != LocalUserID.String() {
		t.Fatalf("local mode = %d %s, want 200 %s", rec.Code, rec.Body, LocalUserID)
	}
}

// SetUserID가 쓰는 키와 UserID가 읽는 키는 같아야 한다. 쿠키로 인증한 화면
// 요청이 핸들러에서 신원을 잃지 않는 근거다.
func TestSetUserIDIsWhatUserIDReads(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	if got := OptionalUserID(c); got != uuid.Nil {
		t.Errorf("OptionalUserID() = %v before anyone set it, want Nil", got)
	}
	SetUserID(c, testUserID)
	if got := UserID(c); got != testUserID {
		t.Errorf("UserID() = %v, want %v", got, testUserID)
	}
	if got := OptionalUserID(c); got != testUserID {
		t.Errorf("OptionalUserID() = %v, want %v", got, testUserID)
	}
}

// JWKS를 처음 받아 오는 데 실패해도 그 실패가 프로세스에 눌러앉으면 안 된다.
// 콜드 스타트 직후의 일시적 네트워크 오류가 그 뒤 요청까지 물들이지 않도록,
// 다음 요청은 다시 받아 와야 한다.
func TestJWKSFetchFailureIsRetried(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	b64 := base64.RawURLEncoding.EncodeToString
	var healthy atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !healthy.Load() {
			http.Error(w, "warming up", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": "k1", "use": "sig", "alg": "RS256",
			"n": b64(key.N.Bytes()), "e": b64(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	defer srv.Close()

	// 다른 테스트가 남긴 클라이언트가 없게 비운다.
	jwksMu.Lock()
	jwks = nil
	jwksMu.Unlock()

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": testUserID.String(), "aud": "authenticated", "exp": time.Now().Add(time.Hour).Unix(),
	})
	tok.Header["kid"] = "k1"
	raw, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}

	mw := Middleware(srv.URL, "")
	// 키 묶음이 없는 것은 토큰 탓이 아니므로 401이 아니라 500이다.
	if rec := runMiddleware(t, mw, "Bearer "+raw); rec.Code != http.StatusInternalServerError {
		t.Fatalf("while JWKS is down: status = %d, want 500", rec.Code)
	}
	healthy.Store(true)
	if rec := runMiddleware(t, mw, "Bearer "+raw); rec.Code != http.StatusOK {
		t.Fatalf("after JWKS recovers: status = %d, want 200 (failure must not be cached): %s", rec.Code, rec.Body)
	}
}
