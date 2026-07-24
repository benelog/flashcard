package auth

import (
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/benelog/flashcard/internal/model"
)

// EnsureProfile은 로그인한 사용자의 프로필 행을 미리 만들어 두는 미들웨어다.
// 첫 쓰기(덱 생성, 가져오기 …)가 profiles(id) 외래 키에 걸리지 않게 하기
// 위해서로, warm 인스턴스마다 사용자당 한 번만 저장소에 다녀온다.
//
// 화면과 JSON API가 같이 쓰므로 실패를 어떤 모양으로 알릴지(오류 화면, JSON)는
// fail로 받는다.
// tag::ensure-profile[]
func EnsureProfile(s model.Store, fail func(*gin.Context, error)) gin.HandlerFunc {
	var seen sync.Map
	return func(c *gin.Context) {
		userID := UserID(c)
		if _, ok := seen.Load(userID); !ok {
			if _, err := s.GetOrCreateProfile(c.Request.Context(), userID, ""); err != nil {
				fail(c, err)
				c.Abort()
				return
			}
			seen.Store(userID, struct{}{})
		}
		c.Next()
	}
}

// end::ensure-profile[]
