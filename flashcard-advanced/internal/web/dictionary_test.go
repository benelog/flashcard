package web

import "testing"

func TestMapEntries(t *testing.T) {
	entries := []apiEntry{{
		Phonetics: []struct {
			Text string `json:"text"`
		}{{Text: ""}, {Text: "/ˌserənˈdɪpɪti/"}},
		Meanings: []struct {
			PartOfSpeech string `json:"partOfSpeech"`
			Definitions  []struct {
				Definition string `json:"definition"`
				Example    string `json:"example"`
			} `json:"definitions"`
		}{
			{
				PartOfSpeech: "noun",
				Definitions: []struct {
					Definition string `json:"definition"`
					Example    string `json:"example"`
				}{{Definition: "an unsought, unexpected discovery", Example: "pure serendipity"}},
			},
			{
				PartOfSpeech: "verb",
				Definitions: []struct {
					Definition string `json:"definition"`
					Example    string `json:"example"`
				}{{Definition: "second meaning"}},
			},
		},
	}}

	got := summarizeEntries(entries)
	if got.Phonetic != "/ˌserənˈdɪpɪti/" {
		t.Errorf("phonetic = %q", got.Phonetic)
	}
	want := "(noun) an unsought, unexpected discovery\n(verb) second meaning"
	if got.Definition != want {
		t.Errorf("definition = %q, want %q", got.Definition, want)
	}
	if got.Example != "pure serendipity" {
		t.Errorf("example = %q", got.Example)
	}
}

func TestMapEntriesEmpty(t *testing.T) {
	got := summarizeEntries(nil)
	if got.Phonetic != "" || got.Definition != "" || got.Example != "" {
		t.Errorf("expected zero entry, got %+v", got)
	}
}
