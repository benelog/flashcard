package web

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/cardcsv"
	"github.com/benelog/flashcard/internal/model"
)

// importCSV는 덱 화면의 파일 업로드 폼을 처리한다. 실패는 모두 덱 화면으로
// 되돌아가며 플래시 메시지로 알린다.
func (w *Web) importCSV(c *gin.Context) {
	slug := c.Param("slug")
	deckURL := "/decks/" + slug

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		redirectWithFlash(c, flashError, "CSV 파일을 선택해주세요", deckURL)
		return
	}
	defer file.Close()

	cards, dropped, err := cardcsv.Parse(file)
	if err != nil {
		redirectWithFlash(c, flashError, "CSV 파일을 읽지 못했어요", deckURL)
		return
	}
	if len(cards) == 0 {
		message := "가져올 카드가 없어요. text,meaning (또는 front,back) 헤더가 있는 CSV인지 확인해주세요"
		if dropped > 0 {
			message += fmt.Sprintf(" (%d행 오류)", dropped)
		}
		redirectWithFlash(c, flashError, message, deckURL)
		return
	}
	if len(cards) > model.MaxBulkCards {
		redirectWithFlash(c, flashError,
			fmt.Sprintf("한 번에 %d장까지만 가져올 수 있어요", model.MaxBulkCards), deckURL)
		return
	}

	deckID, ok := w.deckIDFromPath(c)
	if !ok {
		return
	}
	added, err := w.store.BulkCreateCards(c.Request.Context(), auth.UserID(c), deckID, cards)
	if err != nil {
		w.failPage(c, err)
		return
	}
	message := fmt.Sprintf("%d개 추가, %d개 중복 건너뜀", added.Added, added.Skipped)
	if dropped > 0 {
		message += fmt.Sprintf(", %d개 오류", dropped)
	}
	redirectWithFlash(c, flashInfo, message, deckURL)
}

// exportCSV는 덱을 CSV로 내려보낸다. API의 /export가 주는 것과 같은 파일이다.
func (w *Web) exportCSV(c *gin.Context) {
	if err := cardcsv.ExportDeck(c.Request.Context(), w.store, auth.UserID(c), c.Param("slug"), c.Writer); err != nil {
		w.failPage(c, err)
	}
}
