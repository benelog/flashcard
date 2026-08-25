package cmd

import (
	"github.com/spf13/cobra"

	"github.com/benelog/flashcard-cli/internal/tui"
)

func newMenuCmd(client clientFunc) *cobra.Command {
	return &cobra.Command{
		Use:   "menu",
		Short: "메뉴 모드로 연다(옵션 없이 실행하면 이 모드다)",
		Long: `위쪽 메뉴 바에서 풀다운 메뉴를 펴고 항목을 고른다.

←/→로 메뉴를, ↑/↓로 항목을 옮기고 enter로 고른다. esc는 펼친 메뉴를 닫고,
q는 끝낸다.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.RunMenu(cmd.Context(), client())
		},
	}
}
