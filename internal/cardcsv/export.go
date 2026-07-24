package cardcsv

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/model"
)

// ExportDeck은 한 덱의 카드를 CSV 다운로드로 흘려보낸다. 덱 화면의 내보내기
// 버튼(HTML)과 JSON API의 /export가 같은 파일을 받도록 두 곳이 이 함수를 함께
// 쓴다.
//
// 돌려주는 error는 "덱을 찾거나 카드를 읽지 못했다"는 뜻뿐이라, 부르는 쪽이
// 자기 방식(JSON 응답 또는 HTML 오류 화면)으로 알리면 된다. 파일을 흘려보내던
// 중의 실패는 응답이 이미 나가 버려 되돌릴 수 없으므로 여기서 기록만 남긴다.
func ExportDeck(ctx context.Context, s model.Store, userID uuid.UUID, slug string, w http.ResponseWriter) error {
	deck, err := s.GetDeckBySlug(ctx, userID, slug)
	if err != nil {
		return err
	}
	cards, err := s.ListCards(ctx, userID, deck.ID)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "deck-"+deck.Slug+".csv"))
	if err := Write(w, cards); err != nil {
		log.Printf("csv export: %v", err)
	}
	return nil
}
