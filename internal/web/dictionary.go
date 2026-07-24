package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// 무료 사전 API (api.dictionaryapi.dev). 예전에는 브라우저가 직접 호출했지만
// 이제 Go 서버가 조회해 폼 필드 조각으로 응답한다(htmx out-of-band swap).

var dictClient = &http.Client{Timeout: 8 * time.Second}

type dictEntry struct {
	Phonetic   string
	Definition string
	Example    string
}

type apiEntry struct {
	Phonetic  string `json:"phonetic"`
	Phonetics []struct {
		Text string `json:"text"`
	} `json:"phonetics"`
	Meanings []struct {
		PartOfSpeech string `json:"partOfSpeech"`
		Definitions  []struct {
			Definition string `json:"definition"`
			Example    string `json:"example"`
		} `json:"definitions"`
	} `json:"meanings"`
}

// summarizeEntries는 API 응답을 간추린다. 첫 발음 기호, "(품사) 뜻" 줄 최대
// 둘, 첫 예문만 남긴다.
func summarizeEntries(entries []apiEntry) dictEntry {
	var out dictEntry
	if len(entries) == 0 {
		return out
	}
	first := entries[0]
	out.Phonetic = first.Phonetic
	if out.Phonetic == "" {
		for _, p := range first.Phonetics {
			if p.Text != "" {
				out.Phonetic = p.Text
				break
			}
		}
	}
	var lines []string
	for _, meaning := range first.Meanings {
		if len(meaning.Definitions) == 0 {
			continue
		}
		def := meaning.Definitions[0]
		lines = append(lines, fmt.Sprintf("(%s) %s", meaning.PartOfSpeech, def.Definition))
		if out.Example == "" && def.Example != "" {
			out.Example = def.Example
		}
		if len(lines) >= 2 {
			break
		}
	}
	out.Definition = strings.Join(lines, "\n")
	return out
}

var errWordNotFound = fmt.Errorf("word not found")

func lookupWord(word string) (dictEntry, error) {
	res, err := dictClient.Get(
		"https://api.dictionaryapi.dev/api/v2/entries/en/" + url.PathEscape(strings.TrimSpace(word)))
	if err != nil {
		return dictEntry{}, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return dictEntry{}, errWordNotFound
	}
	if res.StatusCode != http.StatusOK {
		return dictEntry{}, fmt.Errorf("dictionary status %d", res.StatusCode)
	}
	var entries []apiEntry
	if err := json.NewDecoder(res.Body).Decode(&entries); err != nil {
		return dictEntry{}, err
	}
	return summarizeEntries(entries), nil
}

// dictionaryLookup은 사전으로 카드 폼을 채운다. htmx 버튼이 폼 전체를 보내고,
// 응답은 사용자가 아직 입력하지 않은 필드와 상태 줄만 되바꾼다(hx-swap-oob).
func (w *Web) dictionaryLookup(c *gin.Context) {
	form := cardFormView{
		Text:     strings.TrimSpace(c.PostForm("text")),
		Meaning:  c.PostForm("meaning"),
		Phonetic: c.PostForm("phonetic"),
		Example:  c.PostForm("example"),
	}

	entry, err := lookupWord(form.Text)
	switch {
	case err == errWordNotFound:
		form.Status = "사전에서 찾을 수 없어요"
	case err != nil:
		form.Status = "사전 조회에 실패했어요"
	default:
		form.Status = "사전에서 채웠어요"
		// 사용자가 이미 입력한 필드는 건드리지 않는다.
		if strings.TrimSpace(form.Phonetic) == "" {
			form.Phonetic = entry.Phonetic
		}
		if strings.TrimSpace(form.Meaning) == "" {
			form.Meaning = entry.Definition
		}
		if strings.TrimSpace(form.Example) == "" {
			form.Example = entry.Example
		}
	}
	w.renderPartial(c, "lookup_result", form)
}
