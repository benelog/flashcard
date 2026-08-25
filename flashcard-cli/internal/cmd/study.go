package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/benelog/flashcard-cli/internal/api"
	"github.com/benelog/flashcard-cli/internal/tui"
)

func newStudyCmd(client clientFunc) *cobra.Command {
	var (
		reverse bool
		limit   int
	)
	studyCmd := &cobra.Command{
		Use:   "study [덱-slug]",
		Short: "학습 세션을 시작한다",
		Long: `학습 세션을 터미널 화면으로 진행한다.

덱 slug를 주면 그 덱 전체를, 주지 않으면 오늘 복습할 카드(due)를 낸다.
space로 뒤집고 o/x로 채점한다. 채점 결과는 서버의 SRS에 기록된다.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client()
			ctx := cmd.Context()

			req := api.SessionRequest{Mode: "due", Limit: limit}
			title := "오늘 복습"
			if reverse {
				req.Direction = api.MeaningToText
			}
			if len(args) == 1 {
				deck, err := c.GetDeck(ctx, args[0])
				if err != nil {
					return err
				}
				req.Mode = "deck"
				req.DeckID = deck.ID
				title = deck.Name
			}

			msg, err := tui.StartAndRun(ctx, c, req, title)
			if err != nil {
				return err
			}
			if msg != "" {
				fmt.Fprintln(cmd.OutOrStdout(), msg)
			}
			return nil
		},
	}
	studyCmd.Flags().BoolVar(&reverse, "reverse", false, "뜻을 보고 표현을 떠올리는 방향으로")
	studyCmd.Flags().IntVar(&limit, "limit", 0, "낼 카드 수 상한(0이면 서버 기본값)")
	return studyCmd
}
