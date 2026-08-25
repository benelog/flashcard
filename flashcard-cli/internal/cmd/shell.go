package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newShellCmd(resolve func() options) *cobra.Command {
	return &cobra.Command{
		Use:   "shell",
		Short: "명령을 한 줄씩 받는 셸 모드",
		Long: `명령을 한 줄씩 받는다.

프롬프트에 decks·cards·due·study를 그대로 친다. help로 명령 목록을,
exit(또는 quit, Ctrl-D)로 셸을 나간다. --server·--token은 셸에 들어올 때의
값을 이어 쓰고, 한 줄에서 준 플래그는 그 줄에만 적용된다.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShell(cmd.Context(), resolve(), os.Stdin, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
}

// runShell은 한 줄마다 명령 트리를 새로 만들어 실행한다. 트리를 새로 만드는
// 이유는 cobra 플래그가 값을 남기기 때문이다. 한 번 study --reverse를 하면
// 다음 study까지 역방향이 되는 일을 막는다.
func runShell(ctx context.Context, o options, in io.Reader, out, errOut io.Writer) error {
	o.nested = true
	fmt.Fprintf(out, "flashcard 셸 · 서버 %s\nhelp로 명령 목록, exit로 나간다.\n\n", o.server)

	sc := bufio.NewScanner(in)
	for {
		fmt.Fprint(out, "flashcard> ")
		if !sc.Scan() {
			fmt.Fprintln(out) // Ctrl-D로 나갈 때 프롬프트 줄을 닫는다
			break
		}
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "exit", "quit":
			return nil
		case "shell", "menu":
			fmt.Fprintln(errOut, "오류: 셸 안에서는 쓸 수 없는 명령입니다.")
			continue
		}

		tree := newRootCmd(o)
		tree.SetArgs(fields)
		tree.SetOut(out)
		tree.SetErr(errOut)
		if err := tree.ExecuteContext(ctx); err != nil {
			fmt.Fprintln(errOut, "오류:", err) // 한 줄이 실패해도 셸은 이어 간다
		}
	}
	return sc.Err()
}
