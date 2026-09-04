// Package cmd는 CLI의 명령 구조다. 서버 주소와 토큰은 모든 명령이 공유한다.
package cmd

import (
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/benelog/flashcard-cli/internal/api"
	"github.com/benelog/flashcard-cli/internal/tui"
)

// clientFunc는 플래그가 파싱된 뒤(RunE 안에서) 클라이언트를 만드는 함수다.
type clientFunc func() *api.Client

// options는 명령 트리를 만들 때의 초기값이다. 셸 모드는 한 줄마다 트리를 새로
// 만들기 때문에(한 줄에서 준 플래그가 다음 줄에 남지 않도록) 그때 쓰던
// 서버·토큰을 여기에 담아 넘긴다.
type options struct {
	server string
	token  string
	nested bool // 셸 모드 안이면 셸·메뉴로 다시 들어가지 못하게 한다
}

// newRootCmd는 명령 트리를 새로 만든다. 플래그 값을 지역 변수에 묶어 두어
// 트리마다 상태가 따로 논다.
func newRootCmd(o options) *cobra.Command {
	server, token := o.server, ""

	root := &cobra.Command{
		Use:   "flashcard",
		Short: "flashcard 서버를 API로 다루는 CLI",
		Long: `flashcard 서버의 JSON API(/api/*)를 부르는 CLI다.

세 가지 방식으로 쓴다.

  flashcard            메뉴 모드. 위쪽 메뉴 바에서 풀다운 메뉴를 펴고 고른다.
  flashcard decks      명령 모드. 명령과 옵션을 한 번에 준다.
  flashcard shell      셸 모드. 명령을 한 줄씩 받는다.

기본값은 운영 서버(https://flashcard.benelog.net)다. Supabase 인증이 있는
서버를 부르려면 --token이나 FLASHCARD_TOKEN에 액세스 토큰을 넣는다.
로컬 서버(http://localhost:8080)는 --server나 FLASHCARD_SERVER로 지정하며
토큰이 필요 없다.`,
		SilenceUsage:  true, // 서버 오류에 사용법을 다시 찍지 않는다
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&server, "server", o.server, "서버 주소")
	// 토큰은 플래그 기본값으로 두지 않는다. cobra가 도움말에 기본값을 그대로
	// 찍어 비밀이 화면과 로그에 남기 때문이다. 플래그가 비어 있으면 그때
	// 환경 변수에서 온 값(o.token)을 쓴다.
	root.PersistentFlags().StringVar(&token, "token", "", "Supabase 액세스 토큰(없으면 FLASHCARD_TOKEN을 쓴다. 로컬 서버는 불필요)")
	resolvedToken := func() string {
		if token != "" {
			return token
		}
		return o.token
	}

	client := clientFunc(func() *api.Client { return api.New(server, resolvedToken()) })
	root.AddCommand(
		newDecksCmd(client),
		newCardsCmd(client),
		newDueCmd(client),
		newStudyCmd(client),
	)

	if o.nested {
		// 셸 모드 안에서 help를 치면 셸 이야기만 나오게 한다.
		root.Long = "셸 모드다. 아래 명령을 한 줄씩 친다. exit로 나간다."
	} else {
		root.AddCommand(newMenuCmd(client), newShellCmd(func() options {
			return options{server: server, token: resolvedToken(), nested: true}
		}))
		// 옵션 없이 실행하면 메뉴 모드로 간다. NoArgs를 둬야 오타 난 명령이
		// 조용히 메뉴로 빠지지 않고 오류가 된다.
		root.Args = cobra.NoArgs
		root.RunE = func(cmd *cobra.Command, args []string) error {
			// 파이프로 실행하면 화면을 띄울 수 없다. 사용법을 보여 준다.
			if !isatty.IsTerminal(os.Stdin.Fd()) || !isatty.IsTerminal(os.Stdout.Fd()) {
				return cmd.Help()
			}
			return tui.RunMenu(cmd.Context(), client())
		}
	}
	return root
}

func Execute() error {
	return newRootCmd(options{
		server: envOr("FLASHCARD_SERVER", "https://flashcard.benelog.net"),
		token:  os.Getenv("FLASHCARD_TOKEN"),
	}).Execute()
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
