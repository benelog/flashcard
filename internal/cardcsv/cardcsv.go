// Package cardcsv is the single definition of the deck CSV format: the same
// columns, the same tag separator and the same Excel-friendly BOM whether the
// file is produced by the JSON API (/api/decks/{slug}/export), by the deck
// page's 내보내기 button, or read back by 가져오기. 형식이 한 곳에만 적혀 있어야
// 내보낸 파일을 그대로 다시 가져올 수 있다.
package cardcsv

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/benelog/flashcard/internal/store"
)

// Columns is the header row. 각 열의 뜻은 카드 편집 화면의 입력 칸과 같다.
var Columns = []string{"text", "meaning", "type", "tags", "phonetic", "example"}

// tagSeparator joins a card's tags inside one CSV cell. 쉼표는 CSV의 칸 구분자와
// 겹치므로 쓰지 않는다.
const tagSeparator = "|"

// utf8BOM makes Excel open the file as UTF-8 instead of the system codepage,
// which is what garbles Korean text.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// Write streams cards as a downloadable CSV.
func Write(w io.Writer, cards []store.Card) error {
	if _, err := w.Write(utf8BOM); err != nil {
		return err
	}
	out := csv.NewWriter(w)
	if err := out.Write(Columns); err != nil {
		return err
	}
	// tag::write-rows[]
	for _, card := range cards {
		if err := out.Write([]string{
			card.Text,
			card.Meaning,
			card.CardType,
			strings.Join(card.Tags, tagSeparator),
			store.OrEmpty(card.Phonetic),
			store.OrEmpty(card.Example),
		}); err != nil {
			return err
		}
	}
	// end::write-rows[]
	out.Flush()
	return out.Error()
}

// Parse reads an uploaded deck CSV and reports how many rows it had to drop.
// 열 순서는 헤더 행이 정하므로 손으로 만든 파일도 받을 수 있고, 예전 내보내기가
// 쓰던 front/back 헤더도 알아본다.
func Parse(r io.Reader) (cards []store.CardInput, dropped int, err error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1 // 행마다 열 수가 달라도 허용
	reader.LazyQuotes = true

	header, err := reader.Read()
	if err != nil {
		return nil, 0, fmt.Errorf("CSV 헤더를 읽지 못했어요: %w", err)
	}
	columnAt := map[string]int{}
	for i, name := range header {
		columnAt[normalizeHeader(name)] = i
	}
	// firstValue takes the first non-empty cell among the accepted names for one
	// column, so "text"와 "front" 같은 표기 차이를 부르는 쪽이 신경 쓰지 않아도 된다.
	firstValue := func(row []string, names ...string) string {
		for _, name := range names {
			if i, ok := columnAt[name]; ok && i < len(row) {
				if v := strings.TrimSpace(row[i]); v != "" {
					return v
				}
			}
		}
		return ""
	}

	for {
		row, rowErr := reader.Read()
		if rowErr == io.EOF {
			break
		}
		if rowErr != nil {
			dropped++
			continue
		}
		text := firstValue(row, "text", "front")
		meaning := firstValue(row, "meaning", "back")
		if text == "" || meaning == "" {
			dropped++
			continue
		}
		cards = append(cards, store.CardInput{
			Text:     text,
			Meaning:  meaning,
			CardType: store.NormalizeCardType(strings.ToLower(firstValue(row, "type"))),
			Tags:     SplitTags(firstValue(row, "tags")),
			Phonetic: store.NilIfBlank(firstValue(row, "phonetic")),
			Example:  store.NilIfBlank(firstValue(row, "example")),
		})
	}
	return cards, dropped, nil
}

// SplitTags reads one CSV cell's tag list.
func SplitTags(cell string) []string {
	tags := []string{}
	for _, tag := range strings.Split(cell, tagSeparator) {
		if tag = strings.TrimSpace(tag); tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

// normalizeHeader strips the case, padding and the BOM that a
// spreadsheet may have left on the first header cell.
func normalizeHeader(name string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "\uFEFF")))
}
