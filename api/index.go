// Package handler는 Vercel 서버리스 진입점이다. vercel.json이 모든 요청(HTML
// 화면, 정적 자원, JSON API 전부)을 여기로 다시 쓴다. 원래 경로가 보존되므로
// Gin 라우터가 평소처럼 분기한다.
//
// Vercel은 이 파일을 모듈 바깥에서 컴파일하므로 internal/ 패키지를 (직접)
// import할 수 없다. 필요한 공용 코드는 pkg/에 둔다.
package handler

import (
	"net/http"

	"github.com/benelog/flashcard/pkg/app"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	engine, err := app.Engine()
	if err != nil {
		http.Error(w, "server misconfigured: "+err.Error(), http.StatusInternalServerError)
		return
	}
	engine.ServeHTTP(w, r)
}
