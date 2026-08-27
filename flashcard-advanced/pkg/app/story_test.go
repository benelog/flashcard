package app

import (
	"net/http"
	"net/url"
	"testing"
)

// 덱 스토리: 마크다운으로 쓰는 덱 소개 글의 저장·렌더링·미리 보기를 확인한다.

func TestStorySaveAndRender(t *testing.T) {
	a := newTestApp(t)
	slug := a.makeDeck("비즈니스 미팅")

	// 스토리가 없으면 덱 화면에 쓰기 진입점만 보인다.
	rec := a.get("/decks/" + slug)
	mustStatus(t, rec, http.StatusOK)
	mustContain(t, rec, "스토리 쓰기")
	mustNotContain(t, rec, "스토리 읽기")

	// 마크다운을 저장하면 덱 화면으로 돌아간다.
	rec = a.postForm("/decks/"+slug+"/story", url.Values{
		"story": {"# 회의록\n**Sarah:** Let's kick off the meeting."},
	})
	if loc := mustRedirect(t, rec); loc != "/decks/"+slug {
		t.Fatalf("redirect = %q, want %q", loc, "/decks/"+slug)
	}

	// 덱 화면이 변환된 HTML을 그린다.
	rec = a.get("/decks/" + slug)
	mustContain(t, rec, "<h1>회의록</h1>")
	mustContain(t, rec, "<strong>Sarah:</strong>")
	mustContain(t, rec, "스토리 수정")

	// 편집 화면은 변환 전의 원문을 그대로 담는다.
	rec = a.get("/decks/" + slug + "/story")
	mustStatus(t, rec, http.StatusOK)
	mustContain(t, rec, "# 회의록")

	// 빈 제출은 스토리를 지운다.
	mustRedirect(t, a.postForm("/decks/"+slug+"/story", url.Values{"story": {"  "}}))
	rec = a.get("/decks/" + slug)
	mustContain(t, rec, "스토리 쓰기")
	if story := a.deckStory(slug); story != nil {
		t.Fatalf("story = %q, want nil after clearing", *story)
	}
}

// 원문에 섞인 생 HTML은 goldmark가 지운다(html.WithUnsafe를 켜지 않으므로).
// 스토리는 사용자 입력이 그대로 화면에 실리는 자리라 이 동작이 안전망이다.
func TestStoryStripsRawHTML(t *testing.T) {
	a := newTestApp(t)
	slug := a.makeDeck("악성 입력")

	mustRedirect(t, a.postForm("/decks/"+slug+"/story", url.Values{
		"story": {"<script>alert('xss')</script>\n\n안전한 문단"},
	}))
	rec := a.get("/decks/" + slug)
	mustStatus(t, rec, http.StatusOK)
	mustNotContain(t, rec, "<script>alert")
	mustContain(t, rec, "안전한 문단")
}

func TestStoryPreviewRendersWithoutSaving(t *testing.T) {
	a := newTestApp(t)
	slug := a.makeDeck("미리 보기")

	rec := a.postHTMX("/decks/"+slug+"/story/preview", url.Values{
		"story": {"인사는 **bold**하게"},
	})
	mustStatus(t, rec, http.StatusOK)
	mustContain(t, rec, "<strong>bold</strong>")

	// 미리 보기는 저장하지 않는다.
	if story := a.deckStory(slug); story != nil {
		t.Fatalf("story = %q, want nil after preview only", *story)
	}

	// 빈 원문의 미리 보기는 안내 문구다.
	rec = a.postHTMX("/decks/"+slug+"/story/preview", url.Values{"story": {""}})
	mustContain(t, rec, "미리 볼 내용이 없어요")
}

// 스토리에도 듣기 버튼이 붙는다. 읽기 속도는 설정에 저장된 값을 따른다.
func TestStoryHasTTSButton(t *testing.T) {
	a := newTestApp(t)
	slug := a.makeDeck("듣기")

	// 스토리가 없으면 듣기 버튼도 없다.
	mustNotContain(t, a.get("/decks/"+slug), `data-tts-story`)

	mustRedirect(t, a.postForm("/decks/"+slug+"/story", url.Values{
		"story": {"Let's kick off the meeting."},
	}))
	rec := a.get("/decks/" + slug)
	mustContain(t, rec, `data-tts-story="story-body"`)
	mustContain(t, rec, `data-tts-rate="0.9"`) // 기본 읽기 속도

	mustRedirect(t, a.postForm("/settings", url.Values{
		"display_name": {"듣기"},
		"tts_rate":     {"0.6"},
		"daily_goal":   {"20"},
	}))
	mustContain(t, a.get("/decks/"+slug), `data-tts-rate="0.6"`)

	// 미리 보기도 저장 전에 들어볼 수 있다.
	rec = a.postHTMX("/decks/"+slug+"/story/preview", url.Values{"story": {"Hello."}})
	mustContain(t, rec, `data-tts-story="story-preview-body"`)
}

// 공유 덱을 보는 사람도 스토리를 읽고 들을 수 있다. 스토리 링크(?story=1)로
// 들어오면 서버가 details에 open을 붙여 펼친 채로 그린다.
func TestSharedDeckStoryAndOpenLink(t *testing.T) {
	a := newTestApp(t)
	slug := a.makeDeck("공유 스토리")
	a.makeCard(slug, "run", "달리다")
	mustRedirect(t, a.postForm("/decks/"+slug+"/share", nil))
	shareSlug := *a.deck(slug).ShareSlug

	// 스토리가 없으면 공유 화면에 스토리 자리도, 스토리 링크 복사 버튼도 없다.
	mustNotContain(t, a.get("/shared/"+shareSlug), "스토리 읽기")
	mustNotContain(t, a.get("/decks/"+slug), "스토리 링크 복사")

	mustRedirect(t, a.postForm("/decks/"+slug+"/story", url.Values{
		"story": {"# 달리기\nLet's go for a **run**."},
	}))

	// 그냥 공유 링크로 들어오면 접힌 채로 보인다.
	rec := a.get("/shared/" + shareSlug)
	mustStatus(t, rec, http.StatusOK)
	mustContain(t, rec, "<h1>달리기</h1>")
	mustContain(t, rec, `data-tts-story="story-body"`)
	mustNotContain(t, rec, `<details class="story" open>`)

	// 스토리 링크로 들어오면 펼쳐진 채로 보인다.
	rec = a.get("/shared/" + shareSlug + "?story=1")
	mustStatus(t, rec, http.StatusOK)
	mustContain(t, rec, `<details class="story" open>`)
	mustContain(t, rec, "<strong>run</strong>")

	// 덱 주인 화면에는 그 링크를 복사하는 버튼이 생긴다.
	mustContain(t, a.get("/decks/"+slug), "/shared/"+shareSlug+"?story=1")

	// 공유를 풀면 스토리 링크도 함께 죽는다.
	mustRedirect(t, a.postForm("/decks/"+slug+"/unshare", nil))
	mustStatus(t, a.get("/shared/"+shareSlug+"?story=1"), http.StatusNotFound)
}

func TestStoryOfMissingDeckIs404(t *testing.T) {
	a := newTestApp(t)
	mustStatus(t, a.get("/decks/zzzz/story"), http.StatusNotFound)
	mustStatus(t, a.postForm("/decks/zzzz/story", url.Values{"story": {"x"}}), http.StatusNotFound)
	mustStatus(t, a.postHTMX("/decks/zzzz/story/preview", url.Values{"story": {"x"}}), http.StatusNotFound)
}
