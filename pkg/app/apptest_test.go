package app

import (
	"encoding/json"
	"html"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/benelog/flashcard/internal/auth"
	"github.com/benelog/flashcard/internal/config"
	"github.com/benelog/flashcard/internal/litestore"
	"github.com/benelog/flashcard/internal/model"
)

// 여기 있는 테스트는 핸들러를 하나씩 부르는 대신 앱을 통째로 띄워 진짜 요청을
// 보낸다. run_local.sh가 만드는 것과 같은 구성(임시 폴더의 SQLite, 고정 사용자
// 로그인)이라, 라우팅·미들웨어·템플릿·저장소가 실제로 맞물려 도는지까지 한 번에
// 확인된다. 화면(HTML)과 JSON API가 같은 저장소를 보므로 한쪽으로 넣고 다른
// 쪽으로 읽어 보는 확인도 여기서 할 수 있다.

// app은 테스트용으로 띄운 애플리케이션 하나다.
type app struct {
	t      *testing.T
	engine *gin.Engine
	store  *litestore.Store
	userID uuid.UUID
}

func newTestApp(t *testing.T) *app {
	t.Helper()
	return newAppWith(t, func(s model.Store) model.Store { return s })
}

// newAppWith는 부르는 쪽이 저장소를 감쌀 수 있게 열어 둔다. 실패 테스트가 질의
// 하나만 깨뜨리고 나머지는 정상으로 두는 방법이 이것이다. a.store는 진짜 SQLite
// 저장소로 남으므로 검증은 여전히 실제 값을 읽는다.
func newAppWith(t *testing.T, wrap func(model.Store) model.Store) *app {
	t.Helper()
	s, err := litestore.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	cfg := &config.Config{Driver: "sqlite", AuthMode: "local"}
	return &app{t: t, engine: New(cfg, wrap(s)), store: s, userID: auth.LocalUserID}
}

func (a *app) do(req *http.Request) *httptest.ResponseRecorder {
	a.t.Helper()
	rec := httptest.NewRecorder()
	a.engine.ServeHTTP(rec, req)
	return rec
}

func (a *app) get(path string) *httptest.ResponseRecorder {
	a.t.Helper()
	return a.do(httptest.NewRequest(http.MethodGet, path, nil))
}

// getInTZ는 그 시간대의 방문자로 페이지를 요청한다. app.js가 첫 화면에서
// 심어 주는 tz 쿠키를 흉내 낸다.
func (a *app) getInTZ(path, tz string) *httptest.ResponseRecorder {
	a.t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(&http.Cookie{Name: "tz", Value: tz})
	return a.do(req)
}

// postForm은 템플릿의 폼과 같은 방식으로 브라우저 폼을 제출한다.
func (a *app) postForm(path string, form url.Values) *httptest.ResponseRecorder {
	a.t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return a.do(req)
}

// postHTMX는 htmx 방식으로 폼을 제출한다. 서버는 전체 페이지 대신 조각으로
// 답한다.
func (a *app) postHTMX(path string, form url.Values) *httptest.ResponseRecorder {
	a.t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	return a.do(req)
}

// sendJSON은 JSON API를 부른다. body가 빈 문자열이면 본문 없이 보낸다.
func (a *app) sendJSON(method, path, body string) *httptest.ResponseRecorder {
	a.t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	return a.do(req)
}

// upload는 CSV 가져오기 폼처럼 파일 필드 하나를 올린다.
func (a *app) upload(path, field, filename, content string) *httptest.ResponseRecorder {
	a.t.Helper()
	var body strings.Builder
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile(field, filename)
	if err != nil {
		a.t.Fatalf("create form file: %v", err)
	}
	if _, err := io.WriteString(part, content); err != nil {
		a.t.Fatalf("write form file: %v", err)
	}
	if err := form.Close(); err != nil {
		a.t.Fatalf("close form: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body.String()))
	req.Header.Set("Content-Type", form.FormDataContentType())
	return a.do(req)
}

// ---------- 응답 확인 ----------

func mustStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d\nbody: %s", rec.Code, want, truncate(rec.Body.String()))
	}
}

// mustRedirect는 PRG 응답인지 확인하고 어디를 가리키는지 돌려준다.
func mustRedirect(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303\nbody: %s", rec.Code, truncate(rec.Body.String()))
	}
	return rec.Header().Get("Location")
}

func mustContain(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	if !strings.Contains(rec.Body.String(), want) {
		t.Fatalf("response does not contain %q\nbody: %s", want, truncate(rec.Body.String()))
	}
}

func mustNotContain(t *testing.T, rec *httptest.ResponseRecorder, unwanted string) {
	t.Helper()
	if strings.Contains(rec.Body.String(), unwanted) {
		t.Fatalf("response unexpectedly contains %q\nbody: %s", unwanted, truncate(rec.Body.String()))
	}
}

func decodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode json: %v\nbody: %s", err, truncate(rec.Body.String()))
	}
	return out
}

func truncate(body string) string {
	const limit = 1500
	if len(body) <= limit {
		return body
	}
	return body[:limit] + "…(생략)"
}

// ---------- 화면에서 값 꺼내기 ----------

var hiddenInput = regexp.MustCompile(`<input type="hidden" name="([^"]+)" value="([^"]*)">`)

// hiddenFields는 렌더링된 폼의 hidden 입력, 즉 브라우저가 되돌려 보낼 값
// 그대로를 모은다. 학습 화면은 진행 상태를 여기에 실어 나르므로, 이 함수가 곧
// "사용자가 버튼을 누르기 직전의 상태"다.
func hiddenFields(t *testing.T, rec *httptest.ResponseRecorder) url.Values {
	t.Helper()
	values := url.Values{}
	for _, m := range hiddenInput.FindAllStringSubmatch(rec.Body.String(), -1) {
		values.Set(m[1], html.UnescapeString(m[2]))
	}
	if len(values) == 0 {
		t.Fatalf("no hidden fields in response\nbody: %s", truncate(rec.Body.String()))
	}
	return values
}

// ---------- 자료 만들기 ----------

// makeDeck은 웹 폼으로 덱을 만들고 URL 슬러그를 돌려준다.
func (a *app) makeDeck(name string) string {
	a.t.Helper()
	rec := a.postForm("/decks", url.Values{"name": {name}})
	location := mustRedirect(a.t, rec)
	slug, ok := strings.CutPrefix(location, "/decks/")
	if !ok {
		a.t.Fatalf("create deck redirected to %q, want /decks/{slug}", location)
	}
	return slug
}

// makeCard는 웹 폼으로 덱에 카드 한 장을 넣는다.
func (a *app) makeCard(deckSlug, text, meaning string) {
	a.t.Helper()
	rec := a.postForm("/decks/"+deckSlug+"/cards", url.Values{
		"text":      {text},
		"meaning":   {meaning},
		"card_type": {model.CardTypeWord},
	})
	mustRedirect(a.t, rec)
}

// deck은 저장소에서 덱 행을 바로 읽는다. ID가 필요한 검증용이다.
func (a *app) deck(slug string) model.Deck {
	a.t.Helper()
	deck, err := a.store.GetDeckBySlug(a.t.Context(), a.userID, slug)
	if err != nil {
		a.t.Fatalf("get deck %q: %v", slug, err)
	}
	return deck
}

// cards는 저장소에서 덱의 카드를 바로 읽는다. 렌더링된 화면으로는 할 수 없는
// 검증(id, SRS 상태)용이다.
func (a *app) cards(deckSlug string) []model.Card {
	a.t.Helper()
	cards, err := a.store.ListCards(a.t.Context(), a.userID, a.deck(deckSlug).ID)
	if err != nil {
		a.t.Fatalf("list cards: %v", err)
	}
	return cards
}
