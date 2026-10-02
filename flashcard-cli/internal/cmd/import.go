package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/benelog/flashcard-cli/internal/api"
)

// deckFile은 import가 읽는 덱 파일(JSON) 하나의 모양이다.
//
//	{"name": "…", "description": "…", "story": "마크다운", "cards": [{"text": "…", "meaning": "…", …}]}
type deckFile struct {
	Name        string        `json:"name"`
	Description *string       `json:"description"`
	Story       string        `json:"story"`
	Cards       []api.NewCard `json:"cards"`
}

// bulkLimit은 서버가 한 번에 받는 카드 수(model.MaxBulkCards)다.
const bulkLimit = 2000

func newImportCmd(client clientFunc) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "import <덱.json>...",
		Short: "JSON 파일에서 덱·스토리·카드를 넣는다",
		Long: `JSON 파일 하나가 덱 하나다. 같은 이름의 덱이 있으면 그 덱에 넣고, 없으면 만든다.

  {"name": "덱 이름", "description": "설명(선택)", "story": "마크다운(선택)",
   "cards": [{"text": "표현", "meaning": "뜻", "cardType": "sentence",
              "tags": ["태그"], "example": "예문", "notes": "메모"}]}

스토리가 있으면 덱의 스토리를 그것으로 바꾼다. 덱에 이미 있는 표현은 서버가
건너뛰므로 같은 파일을 다시 넣어도 카드가 겹치지 않는다.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			files := make([]deckFile, 0, len(args))
			for _, path := range args {
				f, err := readDeckFile(path)
				if err != nil {
					return err
				}
				files = append(files, f)
			}
			out := cmd.OutOrStdout()
			if dryRun {
				for _, f := range files {
					fmt.Fprintf(out, "%s: 카드 %d장, 스토리 %d자\n", f.Name, len(f.Cards), len([]rune(f.Story)))
				}
				return nil
			}
			return importDecks(cmd.Context(), client(), files, out)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "서버에 보내지 않고 파일만 읽어 요약한다")
	return cmd
}

func readDeckFile(path string) (deckFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return deckFile{}, err
	}
	var f deckFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return deckFile{}, fmt.Errorf("%s: %w", path, err)
	}
	f.Name = strings.TrimSpace(f.Name)
	if f.Name == "" {
		return deckFile{}, fmt.Errorf("%s: name이 비어 있다", path)
	}
	return f, nil
}

// importDecks는 파일마다 덱을 찾거나 만들고, 스토리를 바꾸고, 카드를 넣는다.
// 덱은 이름으로 찾는다. 슬러그는 서버가 정하므로 파일이 미리 알 수 없다.
func importDecks(ctx context.Context, c *api.Client, files []deckFile, out io.Writer) error {
	decks, err := c.ListDecks(ctx)
	if err != nil {
		return err
	}
	slugOf := make(map[string]string, len(decks))
	for _, d := range decks {
		slugOf[d.Name] = d.Slug
	}

	for _, f := range files {
		slug, found := slugOf[f.Name]
		if !found {
			deck, err := c.CreateDeck(ctx, f.Name, f.Description)
			if err != nil {
				return fmt.Errorf("%s: 덱 만들기: %w", f.Name, err)
			}
			slug = deck.Slug
			slugOf[f.Name] = slug
		}
		if f.Story != "" {
			if err := c.PutDeckStory(ctx, slug, f.Story); err != nil {
				return fmt.Errorf("%s: 스토리: %w", f.Name, err)
			}
		}
		var sum api.BulkResult
		for start := 0; start < len(f.Cards); start += bulkLimit {
			res, err := c.BulkCreateCards(ctx, slug, f.Cards[start:min(start+bulkLimit, len(f.Cards))])
			if err != nil {
				return fmt.Errorf("%s: 카드: %w", f.Name, err)
			}
			sum.Added += res.Added
			sum.Skipped += res.Skipped
			sum.Invalid += res.Invalid
		}
		verb := "기존 덱"
		if !found {
			verb = "새 덱"
		}
		fmt.Fprintf(out, "%s %s(%s): 카드 %d장 추가, %d장은 이미 있음", verb, f.Name, slug, sum.Added, sum.Skipped)
		if sum.Invalid > 0 {
			fmt.Fprintf(out, ", %d장은 서버가 거절", sum.Invalid)
		}
		fmt.Fprintln(out)
	}
	return nil
}
