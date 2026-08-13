// Package tui는 학습 세션의 터미널 화면이다. Bubble Tea(Elm 아키텍처)로,
// 카드 뒤집기와 채점 상태 전이를 Update 하나에 모은다.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/benelog/flashcard-cli/internal/api"
)

// Summary는 세션이 끝난 뒤 호출 쪽에 돌려주는 결과다.
type Summary struct {
	Correct  int
	Wrong    int
	Total    int
	Finished bool // 마지막 카드까지 채점했는지
}

type study struct {
	client  *api.Client
	session api.Session
	cards   []api.Card
	title   string

	idx     int
	flipped bool
	grading bool // 채점 요청이 서버에 가 있는 동안 입력을 막는다
	correct int
	wrong   int
	done    bool
	err     error
}

type reviewedMsg struct {
	result  bool
	outcome api.ReviewOutcome
	err     error
}

type finishedMsg struct{ err error }

// Run은 학습 화면을 띄우고 세션이 끝날 때까지 돈다. 중간에 그만두면
// completed=false로, 끝까지 하면 true로 서버에 세션 종료를 알린다.
func Run(client *api.Client, title string, started api.StartedSession) (Summary, error) {
	m := study{client: client, session: started.Session, cards: started.Cards, title: title}
	out, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return Summary{}, err
	}
	final := out.(study)
	if final.err != nil {
		return Summary{}, final.err
	}
	return Summary{
		Correct:  final.correct,
		Wrong:    final.wrong,
		Total:    len(final.cards),
		Finished: final.done,
	}, nil
}

func (m study) Init() tea.Cmd { return nil }

func (m study) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.onKey(msg)
	case reviewedMsg:
		m.grading = false
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		if msg.result {
			m.correct++
		} else {
			m.wrong++
		}
		m.idx++
		m.flipped = false
		if m.idx >= len(m.cards) {
			m.done = true
			return m, m.finish(true)
		}
		return m, nil
	case finishedMsg:
		// 종료 통지는 실패해도 학습 결과 표시를 막지 않는다.
		return m, tea.Quit
	}
	return m, nil
}

func (m study) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.grading {
		return m, nil // 채점 응답을 기다리는 중에는 키를 받지 않는다
	}
	switch msg.String() {
	case "ctrl+c", "q":
		if m.done {
			return m, tea.Quit
		}
		return m, m.finish(false)
	case " ", "enter":
		if !m.done {
			m.flipped = !m.flipped
		}
	case "o":
		if m.flipped {
			m.grading = true
			return m, m.review(true)
		}
	case "x":
		if m.flipped {
			m.grading = true
			return m, m.review(false)
		}
	}
	return m, nil
}

func (m study) review(result bool) tea.Cmd {
	card := m.cards[m.idx]
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		out, err := m.client.RecordReview(ctx, m.session.ID, card.ID, result)
		return reviewedMsg{result: result, outcome: out, err: err}
	}
}

func (m study) finish(completed bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return finishedMsg{err: m.client.FinishSession(ctx, m.session.ID, completed)}
	}
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true)
	dimStyle   = lipgloss.NewStyle().Faint(true)
	cardStyle  = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 3).
			Width(56).
			Align(lipgloss.Center)
	backStyle = cardStyle.BorderForeground(lipgloss.Color("6"))
)

func (m study) View() string {
	if m.done {
		return "" // 마무리 결과는 TUI 종료 후 일반 출력으로 보여 준다
	}
	card := m.cards[m.idx]

	// 방향에 따라 앞뒷면이 바뀐다. meaning_to_text면 뜻을 보고 표현을 떠올린다.
	front, back := card.Text, card.Meaning
	if m.session.Direction == api.MeaningToText {
		front, back = back, front
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s  %s\n\n",
		titleStyle.Render(m.title),
		dimStyle.Render(fmt.Sprintf("%d/%d", m.idx+1, len(m.cards))))

	if m.flipped {
		body := back
		if card.Phonetic != nil {
			body += "\n" + dimStyle.Render("["+*card.Phonetic+"]")
		}
		if card.Example != nil {
			body += "\n\n" + dimStyle.Render(*card.Example)
		}
		b.WriteString(backStyle.Render(body))
		b.WriteString("\n\n")
		if m.grading {
			b.WriteString(dimStyle.Render("기록 중…"))
		} else {
			b.WriteString(dimStyle.Render("o 맞혔다 · x 틀렸다 · space 다시 앞면 · q 그만"))
		}
	} else {
		b.WriteString(cardStyle.Render(front))
		b.WriteString("\n\n")
		b.WriteString(dimStyle.Render("space 뒤집기 · q 그만"))
	}
	b.WriteString("\n")
	return b.String()
}
