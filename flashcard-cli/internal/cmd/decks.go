package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var decksCmd = &cobra.Command{
	Use:   "decks",
	Short: "덱 목록을 본다",
	RunE: func(cmd *cobra.Command, args []string) error {
		decks, err := client().ListDecks(cmd.Context())
		if err != nil {
			return err
		}
		if len(decks) == 0 {
			fmt.Println("덱이 없습니다.")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 2, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SLUG\t이름\t카드\t공유")
		for _, d := range decks {
			shared := ""
			if d.ShareSlug != nil {
				shared = "공유 중"
			}
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", d.Slug, d.Name, d.CardCount, shared)
		}
		return w.Flush()
	},
}

var cardsCmd = &cobra.Command{
	Use:   "cards <덱-slug>",
	Short: "덱의 카드 목록을 본다",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cards, err := client().ListDeckCards(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		if len(cards) == 0 {
			fmt.Println("카드가 없습니다.")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 2, 0, 2, ' ', 0)
		fmt.Fprintln(w, "표현\t뜻\t복습 횟수\t다음 복습")
		for _, c := range cards {
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\n",
				c.Text, c.Meaning, c.Attempts, c.DueAt.Local().Format("2006-01-02"))
		}
		return w.Flush()
	},
}

var dueCmd = &cobra.Command{
	Use:   "due",
	Short: "오늘 복습할 카드 수를 본다",
	RunE: func(cmd *cobra.Command, args []string) error {
		n, err := client().DueCount(cmd.Context())
		if err != nil {
			return err
		}
		fmt.Printf("복습할 카드: %d장\n", n)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(decksCmd, cardsCmd, dueCmd)
}
