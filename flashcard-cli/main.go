// flashcard CLI: flashcard-advanced 서버를 API로 다룬다.
package main

import (
	"fmt"
	"os"

	"github.com/benelog/flashcard-cli/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "오류:", err)
		os.Exit(1)
	}
}
