package cardcsv

import (
	"bytes"
	"strings"
	"testing"

	"github.com/benelog/flashcard/internal/model"
)

func TestParse(t *testing.T) {
	csv := "text,meaning,type,tags,phonetic,example\n" +
		"serendipity,우연한 행운,word,토익|명사,/ˌserənˈdipəti/,It was pure serendipity.\n" +
		"hit the sack,자러 가다,idiom,,,\n" +
		"unknown-type,뜻,banana,,,\n" +
		",뜻만 있음,,,,\n"

	cards, dropped, err := Parse(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if dropped != 1 {
		t.Errorf("dropped = %d, want 1", dropped)
	}
	if len(cards) != 3 {
		t.Fatalf("len(cards) = %d, want 3", len(cards))
	}
	first := cards[0]
	if first.Text != "serendipity" || first.Meaning != "우연한 행운" {
		t.Errorf("unexpected first card: %+v", first)
	}
	if len(first.Tags) != 2 || first.Tags[0] != "토익" || first.Tags[1] != "명사" {
		t.Errorf("tags = %v, want [토익 명사]", first.Tags)
	}
	if first.Phonetic == nil || *first.Phonetic != "/ˌserənˈdipəti/" {
		t.Errorf("phonetic = %v", first.Phonetic)
	}
	// 모르는 type은 기본 종류로 정규화된다.
	if cards[2].CardType != model.DefaultCardType {
		t.Errorf("cardType = %q, want %q", cards[2].CardType, model.DefaultCardType)
	}
}

func TestParseLegacyHeaders(t *testing.T) {
	// 옛 내보내기 형식(front,back)과 BOM이 있어도 읽힌다.
	csv := "\uFEFFfront,back\napple,사과\n"
	cards, dropped, err := Parse(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if dropped != 0 || len(cards) != 1 {
		t.Fatalf("cards=%d dropped=%d, want 1/0", len(cards), dropped)
	}
	if cards[0].Text != "apple" || cards[0].Meaning != "사과" {
		t.Errorf("unexpected card: %+v", cards[0])
	}
}

func TestParseNoUsableHeader(t *testing.T) {
	cards, _, err := Parse(strings.NewReader("a,b\n1,2\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 0 {
		t.Errorf("len(cards) = %d, want 0", len(cards))
	}
}

// 내보낸 파일을 그대로 다시 가져올 수 있어야 한다. 이 왕복이 깨지면 사용자는
// 자기 덱을 백업했다가 복원하지 못한다.
func TestWriteThenParseRoundTrip(t *testing.T) {
	phonetic, example := "/ˈæpl/", "An apple a day."
	exported := []model.Card{
		{
			Text:     "apple",
			Meaning:  "사과",
			CardType: model.CardTypeWord,
			Tags:     []string{"과일", "기초"},
			Phonetic: &phonetic,
			Example:  &example,
		},
		{Text: "hit the sack", Meaning: "자러 가다", CardType: model.CardTypeIdiom},
	}

	var file bytes.Buffer
	if err := Write(&file, exported); err != nil {
		t.Fatal(err)
	}
	imported, dropped, err := Parse(&file)
	if err != nil {
		t.Fatal(err)
	}
	if dropped != 0 || len(imported) != len(exported) {
		t.Fatalf("cards=%d dropped=%d, want %d/0", len(imported), dropped, len(exported))
	}
	if imported[0].Text != "apple" || imported[0].Meaning != "사과" {
		t.Errorf("unexpected first card: %+v", imported[0])
	}
	if len(imported[0].Tags) != 2 || imported[0].Tags[1] != "기초" {
		t.Errorf("tags = %v, want [과일 기초]", imported[0].Tags)
	}
	if imported[0].Example == nil || *imported[0].Example != example {
		t.Errorf("example = %v, want %q", imported[0].Example, example)
	}
	// 값이 없던 칸은 빈 문자열이 아니라 nil로 돌아와야 한다.
	if imported[1].Phonetic != nil {
		t.Errorf("phonetic = %v, want nil", imported[1].Phonetic)
	}
}
