package cmd

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/benelog/flashcard-cli/internal/auth"
	"github.com/benelog/flashcard-cli/internal/tui"
)

// loginAuth는 브라우저 로그인을 명령 모드와 메뉴 모드가 같이 쓰는 꼴로 묶는다.
// tui.Auth를 구현한다.
type loginAuth struct {
	client clientFunc
	store  *auth.Store
	server func() string // 플래그가 파싱된 뒤의 서버 주소
}

var errNoStore = errors.New("로그인 저장소를 쓸 수 없습니다. --token을 쓰세요")

func (a *loginAuth) Begin(ctx context.Context, provider string) (string, func(context.Context) error, error) {
	if a.store == nil {
		return "", nil, errNoStore
	}
	flow, err := auth.Begin(ctx, a.client(), provider)
	if err != nil {
		return "", nil, err
	}
	server := a.server()
	wait := func(ctx context.Context) error {
		creds, err := flow.Wait(ctx)
		if err != nil {
			return err
		}
		return a.store.Save(server, creds)
	}
	return flow.URL, wait, nil
}

func (a *loginAuth) OpenBrowser(url string) error { return auth.OpenBrowser(url) }

func (a *loginAuth) Logout() (bool, error) {
	if a.store == nil {
		return false, errNoStore
	}
	return a.store.Delete(a.server())
}

func newLoginCmd(a *loginAuth) *cobra.Command {
	var provider string
	c := &cobra.Command{
		Use:   "login",
		Short: "브라우저로 로그인해 토큰을 저장한다",
		Long: `브라우저를 열어 GitHub 또는 Google로 로그인한다. 끝나면 토큰이
설정 디렉터리(리눅스는 ~/.config/flashcard/credentials.json)에 저장돼
다음 실행부터 자동으로 쓰인다. 만료되면 알아서 갱신한다.

브라우저가 열리지 않으면 화면에 찍힌 주소를 직접 연다. 로그인 창은
localhost:` + fmt.Sprint(auth.CallbackPort) + `로 돌아오므로 같은 컴퓨터에서 열어야 한다.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !slices.Contains(auth.Providers, provider) {
				return fmt.Errorf("--provider는 %s 중 하나다", strings.Join(auth.Providers, "·"))
			}
			out := cmd.OutOrStdout()
			url, wait, err := a.Begin(cmd.Context(), provider)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "브라우저에서 로그인을 마치세요.\n브라우저가 열리지 않으면 이 주소를 직접 여세요:\n\n  %s\n\n", url)
			if err := a.OpenBrowser(url); err != nil {
				fmt.Fprintln(out, "브라우저를 열지 못했습니다:", err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()
			if err := wait(ctx); err != nil {
				return err
			}
			name := ""
			if me, err := a.client().Me(cmd.Context()); err == nil && me.DisplayName != nil {
				name = *me.DisplayName
			}
			if name != "" {
				fmt.Fprintf(out, "%s님으로 로그인했습니다. (%s)\n", name, a.store.Path())
			} else {
				fmt.Fprintf(out, "로그인했습니다. (%s)\n", a.store.Path())
			}
			return nil
		},
	}
	c.Flags().StringVar(&provider, "provider", "github", "로그인 제공자(github 또는 google)")
	return c
}

func newLogoutCmd(a *loginAuth) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "저장된 로그인을 지운다",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			had, err := a.Logout()
			if err != nil {
				return err
			}
			if had {
				fmt.Fprintln(cmd.OutOrStdout(), "로그아웃했습니다.")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "저장된 로그인이 없습니다.")
			}
			return nil
		},
	}
}

var _ tui.Auth = (*loginAuth)(nil)
