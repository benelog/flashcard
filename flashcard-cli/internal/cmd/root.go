// Package cmd는 CLI의 명령 구조다. 서버 주소와 토큰은 모든 명령이 공유한다.
package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/benelog/flashcard-cli/internal/api"
)

var (
	serverURL string
	token     string
)

var rootCmd = &cobra.Command{
	Use:   "flashcard",
	Short: "flashcard 서버를 API로 다루는 CLI",
	Long: `flashcard 서버의 JSON API(/api/*)를 부르는 CLI다.

기본값은 로컬 서버(http://localhost:8080, 인증 없음)다. dev/production처럼
Supabase 인증이 있는 서버를 부르려면 --token이나 FLASHCARD_TOKEN에
액세스 토큰을 넣는다.`,
	SilenceUsage:  true, // 서버 오류에 사용법을 다시 찍지 않는다
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&serverURL, "server",
		envOr("FLASHCARD_SERVER", "http://localhost:8080"), "서버 주소")
	rootCmd.PersistentFlags().StringVar(&token, "token",
		os.Getenv("FLASHCARD_TOKEN"), "Supabase 액세스 토큰(로컬 서버는 불필요)")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func client() *api.Client {
	return api.New(serverURL, token)
}
