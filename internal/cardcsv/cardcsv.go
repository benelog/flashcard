// Package cardcsv는 덱 CSV 형식의 유일한 정의다. JSON API(/api/decks/{slug}/export)가
// 만들든, 덱 화면의 내보내기 버튼이 만들든, 가져오기가 읽든 같은 열, 같은 태그
// 구분자, 같은 Excel용 BOM을 쓴다. 형식이 한 곳에만 적혀 있어야 내보낸 파일을
// 그대로 다시 가져올 수 있다.
package cardcsv

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/benelog/flashcard/internal/model"
)

// Columns는 헤더 행이다. 각 열의 뜻은 카드 편집 화면의 입력 칸과 같다.
var Columns = []string{"text", "meaning", "type", "tags", "phonetic", "example"}

// tagSeparator는 CSV 한 칸 안에서 카드의 태그들을 잇는다. 쉼표는 CSV의 칸
// 구분자와 겹치므로 쓰지 않는다.
const tagSeparator = "|"

// utf8BOM은 Excel이 파일을 시스템 코드페이지가 아니라 UTF-8로 열게 한다.
// 코드페이지로 열면 한글이 깨진다.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// Write는 cards를 내려받을 CSV로 흘려보낸다.
func Write(w io.Writer, cards []model.Card) error {
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
			model.OrEmpty(card.Phonetic),
			model.OrEmpty(card.Example),
		}); err != nil {
			return err
		}
	}
	// end::write-rows[]
	out.Flush()
	return out.Error()
}

// Parse는 올라온 덱 CSV를 읽고 버려야 했던 행 수를 함께 알려 준다.
// 열 순서는 헤더 행이 정하므로 손으로 만든 파일도 받을 수 있고, 예전 내보내기가
// 쓰던 front/back 헤더도 알아본다.
func Parse(r io.Reader) (cards []model.CardInput, dropped int, err error) {
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
	// firstValue는 한 열이 받는 이름들 가운데 처음으로 비어 있지 않은 칸을 고른다.
	// "text"와 "front" 같은 표기 차이를 부르는 쪽이 신경 쓰지 않아도 된다.
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
		cards = append(cards, model.CardInput{
			Text:     text,
			Meaning:  meaning,
			CardType: model.NormalizeCardType(strings.ToLower(firstValue(row, "type"))),
			Tags:     SplitTags(firstValue(row, "tags")),
			Phonetic: model.NilIfBlank(firstValue(row, "phonetic")),
			Example:  model.NilIfBlank(firstValue(row, "example")),
		})
	}
	return cards, dropped, nil
}

// SplitTags는 CSV 한 칸의 태그 목록을 읽는다.
func SplitTags(cell string) []string {
	tags := []string{}
	for _, tag := range strings.Split(cell, tagSeparator) {
		if tag = strings.TrimSpace(tag); tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

// normalizeHeader는 대소문자·둘레 공백과, 스프레드시트가 첫 헤더 칸에 남겼을 수
// 있는 BOM을 걷어 낸다.
func normalizeHeader(name string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "\uFEFF")))
}
