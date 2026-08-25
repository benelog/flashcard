package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/benelog/flashcard-cli/internal/tui"
)

func newDecksCmd(client clientFunc) *cobra.Command {
	return &cobra.Command{
		Use:   "decks",
		Short: "덱 목록을 본다",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			decks, err := client().ListDecks(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), tui.DeckTable(decks))
			return nil
		},
	}
}

func newCardsCmd(client clientFunc) *cobra.Command {
	return &cobra.Command{
		Use:   "cards <덱-slug>",
		Short: "덱의 카드 목록을 본다",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cards, err := client().ListDeckCards(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), tui.CardTable(cards))
			return nil
		},
	}
}

func newDueCmd(client clientFunc) *cobra.Command {
	return &cobra.Command{
		Use:   "due",
		Short: "오늘 복습할 카드 수를 본다",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := client().DueCount(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "복습할 카드: %d장\n", n)
			return nil
		},
	}
}
