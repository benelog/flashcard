package tui

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/benelog/flashcard-cli/internal/api"
)

// 목록을 표 문자열로 만든다. 명령 모드는 그대로 찍고, 메뉴 모드는 내용 칸에
// 넣는다. 두 모드가 같은 표를 보도록 여기 한 군데에 둔다.

func DeckTable(decks []api.Deck) string {
	if len(decks) == 0 {
		return "덱이 없습니다."
	}
	return table("SLUG\t이름\t카드\t공유", func(w *tabwriter.Writer) {
		for _, d := range decks {
			shared := ""
			if d.ShareSlug != nil {
				shared = "공유 중"
			}
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", d.Slug, d.Name, d.CardCount, shared)
		}
	})
}

func CardTable(cards []api.Card) string {
	if len(cards) == 0 {
		return "카드가 없습니다."
	}
	return table("표현\t뜻\t복습 횟수\t다음 복습", func(w *tabwriter.Writer) {
		for _, c := range cards {
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\n",
				c.Text, c.Meaning, c.Attempts, c.DueAt.Local().Format("2006-01-02"))
		}
	})
}

func table(header string, rows func(*tabwriter.Writer)) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 2, 0, 2, ' ', 0)
	fmt.Fprintln(w, header)
	rows(w)
	w.Flush()
	return strings.TrimRight(b.String(), "\n")
}
