package web

import (
	"net/http"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/model"
	"github.com/gin-gonic/gin"
)

// 덱 스토리 편집 화면과 저장·미리 보기 처리. 스토리는 덱의 소개 글로, 카드들이
// 나온 맥락(대화 전체, 글 한 편)을 마크다운으로 적는 자리다. 완성된 글은 덱
// 화면(deck.html)이 그린다.

type storyFormView struct {
	DeckName string
	Slug     string
	Story    string // 마크다운 원문
}

func (w *Web) storyEditPage(c *gin.Context) {
	userID := auth.UserID(c)
	ctx := c.Request.Context()
	deck, err := w.store.GetDeckBySlug(ctx, userID, c.Param("slug"))
	if err != nil {
		w.failPage(c, err)
		return
	}
	story, err := w.store.DeckStory(ctx, userID, deck.ID)
	if err != nil {
		w.failPage(c, err)
		return
	}
	w.render(c, http.StatusOK, "story_form", deck.Name+" 스토리", storyFormView{
		DeckName: deck.Name,
		Slug:     deck.Slug,
		Story:    model.OrEmpty(story),
	})
}

func (w *Web) saveStory(c *gin.Context) {
	deckID, ok := w.deckIDFromPath(c)
	if !ok {
		return
	}
	// 빈 제출은 스토리를 지운 것이다(NULL로 저장).
	story := model.NilIfBlank(c.PostForm("story"))
	if err := w.store.UpdateDeckStory(c.Request.Context(), auth.UserID(c), deckID, story); err != nil {
		w.failPage(c, err)
		return
	}
	msg := "스토리를 저장했어요"
	if story == nil {
		msg = "스토리를 비웠어요"
	}
	redirectWithFlash(c, flashInfo, msg, "/decks/"+c.Param("slug"))
}

// previewStory는 편집 중인 원문을 저장하지 않고 그려서 돌려준다. 편집 화면의
// 미리 보기 버튼이 htmx로 부른다. 없는 덱이면 폼 오류보다 먼저 404를 내
// 슬러그의 존재가 응답으로 드러나지 않게 한다.
func (w *Web) previewStory(c *gin.Context) {
	if _, ok := w.deckIDFromPath(c); !ok {
		return
	}
	w.renderPartial(c, "story_preview", markdownHTML(c.PostForm("story")))
}
