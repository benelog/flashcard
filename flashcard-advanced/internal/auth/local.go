package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// LocalUserID는 로컬 모드의 모든 요청이 쓰는 고정 사용자다.
var LocalUserID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// LocalMiddleware는 로컬 단일 사용자 모드에서 토큰 검증을 대신한다. 모든 요청을
// LocalUserID로 로그인시키므로 Authorization 헤더도 Supabase도 필요 없다.
// required·optional 두 자리를 다 맡는다.
func LocalMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(userIDKey, LocalUserID)
		c.Next()
	}
}
