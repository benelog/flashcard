// 메뉴 모드 화면. Turbo C처럼 위쪽 메뉴 바에서 풀다운 메뉴를 펴고 항목을
// 고른다. 옵션 없이 CLI를 실행하면 이 화면이 뜬다.
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

type itemID int

const (
	itemHealth itemID = iota
	itemLoginGitHub
	itemLoginGoogle
	itemLogout
	itemHelp
	itemQuit
	itemDecks
	itemCards
	itemDue
	itemStudyDue
	itemStudyDueRev
	itemStudyDeck
	itemStudyDeckRev
)

type menuItem struct {
	id    itemID
	label string
}

type menuDef struct {
	title string
	items []menuItem
}

var menuBar = []menuDef{
	{"프로그램", []menuItem{
		{itemHealth, "서버 확인"},
		{itemLoginGitHub, "GitHub로 로그인…"},
		{itemLoginGoogle, "Google로 로그인…"},
		{itemLogout, "로그아웃"},
		{itemHelp, "조작 안내"},
		{itemQuit, "끝내기"},
	}},
	{"덱", []menuItem{
		{itemDecks, "덱 목록"},
		{itemCards, "카드 보기…"},
		{itemDue, "오늘 복습할 카드 수"},
	}},
	{"학습", []menuItem{
		{itemStudyDue, "오늘 복습"},
		{itemStudyDueRev, "오늘 복습(뜻→표현)"},
		{itemStudyDeck, "덱 골라 학습…"},
		{itemStudyDeckRev, "덱 골라 학습(뜻→표현)…"},
	}},
}

// Auth는 메뉴의 로그인 항목이 쓰는 뒷단이다. cmd 패키지가 채운다. 브라우저를
// 열고 기다리는 일이라 두 단계로 나뉜다. Begin은 열 주소를 만들고, wait는
// 브라우저가 돌아올 때까지 막혔다가 로그인을 저장한다.
type Auth interface {
	Begin(ctx context.Context, provider string) (url string, wait func(context.Context) error, err error)
	OpenBrowser(url string) error
	Logout() (bool, error)
}

// 학습은 화면(Bubble Tea 프로그램)을 새로 띄워야 한다. 프로그램을 겹쳐 돌릴
// 수 없어서 메뉴를 한 번 닫고 RunMenu가 이어서 학습 화면을 띄운다.
type studyAction struct {
	wanted bool
	req    api.SessionRequest
	title  string
}

// RunMenu는 메뉴 화면을 띄운다. 학습을 고르면 메뉴를 닫고 학습 화면으로
// 넘어갔다가, 학습이 끝나면 결과를 안고 메뉴로 돌아온다.
func RunMenu(ctx context.Context, c *api.Client, a Auth) error {
	notice := ""
	for {
		m := newMenu(c, notice)
		m.auth = a
		out, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx)).Run()
		if err != nil {
			return err
		}
		m = out.(menu)
		if m.err != nil {
			return m.err
		}
		if !m.study.wanted {
			return nil
		}
		msg, err := StartAndRun(ctx, c, m.study.req, m.study.title)
		if err != nil {
			notice = "오류: " + err.Error()
			continue
		}
		notice = msg
		if notice == "" {
			notice = "학습을 마쳤습니다."
		}
	}
}

// deckPicker는 덱을 골라야 하는 항목(카드 보기, 덱 학습)에서 쓰는 목록이다.
type deckPicker struct {
	decks   []api.Deck
	cursor  int
	purpose itemID
}

type menu struct {
	client *api.Client
	auth   Auth

	bar  int  // 메뉴 바에서 짚고 있는 메뉴
	open bool // 풀다운을 폈는지
	sel  int  // 펼친 메뉴에서 짚고 있는 항목

	picker  *deckPicker
	content string // 가운데 내용 칸
	status  string // 아래 한 줄
	busy    bool   // 서버 응답을 기다리는 중
	err     error
	study   studyAction

	loginCancel context.CancelFunc // 브라우저를 기다리는 중이면 esc로 접는다

	width, height int
}

func newMenu(c *api.Client, notice string) menu {
	m := menu{client: c, content: notice, width: 80, height: 24}
	if m.content == "" {
		m.content = "flashcard\n\n메뉴에서 덱을 보고 학습을 시작한다.\n조작이 궁금하면 프로그램 ▸ 조작 안내."
	}
	if c != nil {
		m.status = "서버 " + c.BaseURL()
	}
	return m
}

type decksMsg struct {
	decks   []api.Deck
	purpose itemID
	err     error
}

type cardsMsg struct {
	name  string
	cards []api.Card
	err   error
}

type dueMsg struct {
	count int
	err   error
}

type healthMsg struct{ err error }

// loginStartedMsg는 브라우저에 열 주소가 준비됐다는 신호다. 화면에 주소를
// 보여 준 뒤 브라우저를 열고 wait를 시작한다.
type loginStartedMsg struct {
	url  string
	wait func(context.Context) error
	err  error
}

type loginDoneMsg struct {
	name string // 로그인한 사용자의 표시 이름
	err  error
}

type logoutMsg struct {
	had bool
	err error
}

func (m menu) Init() tea.Cmd { return nil }

func (m menu) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyMsg:
		return m.onKey(msg)

	case decksMsg:
		m.busy = false
		if msg.err != nil {
			m.status = "오류: " + msg.err.Error()
			return m, nil
		}
		if msg.purpose == itemDecks {
			m.content = DeckTable(msg.decks)
			m.status = fmt.Sprintf("덱 %d개", len(msg.decks))
			return m, nil
		}
		if len(msg.decks) == 0 {
			m.status = "덱이 없습니다."
			return m, nil
		}
		m.picker = &deckPicker{decks: msg.decks, purpose: msg.purpose}
		m.status = "↑↓ 고르고 enter · esc 취소"

	case cardsMsg:
		m.busy = false
		if msg.err != nil {
			m.status = "오류: " + msg.err.Error()
			return m, nil
		}
		m.content = CardTable(msg.cards)
		m.status = fmt.Sprintf("%s · 카드 %d장", msg.name, len(msg.cards))

	case dueMsg:
		m.busy = false
		if msg.err != nil {
			m.status = "오류: " + msg.err.Error()
			return m, nil
		}
		m.content = fmt.Sprintf("복습할 카드: %d장", msg.count)
		m.status = ""

	case healthMsg:
		m.busy = false
		if msg.err != nil {
			m.status = "오류: " + msg.err.Error()
			return m, nil
		}
		m.content = m.client.BaseURL() + " 에 연결됩니다."
		m.status = ""

	case loginStartedMsg:
		if msg.err != nil {
			m.busy = false
			m.status = "오류: " + msg.err.Error()
			return m, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		m.loginCancel = cancel
		m.content = "브라우저에서 로그인을 마치세요.\n\n브라우저가 열리지 않으면 이 주소를 직접 여세요:\n" + msg.url
		m.status = "브라우저를 기다리는 중… esc 취소"
		return m, tea.Batch(m.openBrowser(msg.url), m.waitLogin(ctx, msg.wait))

	case loginDoneMsg:
		m.busy = false
		if m.loginCancel != nil {
			m.loginCancel()
			m.loginCancel = nil
		}
		if msg.err != nil {
			m.content = "로그인하지 못했습니다."
			m.status = "오류: " + msg.err.Error()
			return m, nil
		}
		m.content = msg.name + "님으로 로그인했습니다."
		m.status = ""

	case logoutMsg:
		m.busy = false
		if msg.err != nil {
			m.status = "오류: " + msg.err.Error()
			return m, nil
		}
		if msg.had {
			m.content = "로그아웃했습니다."
		} else {
			m.content = "저장된 로그인이 없습니다."
		}
		m.status = ""
	}
	return m, nil
}

func (m menu) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	if m.busy {
		// 서버 응답을 기다리는 중에는 키를 받지 않는다. 브라우저를 기다리는
		// 중이라면 esc로 접을 수는 있다.
		if key == "esc" && m.loginCancel != nil {
			m.loginCancel()
		}
		return m, nil
	}
	if m.picker != nil {
		return m.onPickerKey(key)
	}

	switch key {
	case "q":
		return m, tea.Quit
	case "left", "h":
		m.bar = (m.bar - 1 + len(menuBar)) % len(menuBar)
		m.sel = 0
	case "right", "l":
		m.bar = (m.bar + 1) % len(menuBar)
		m.sel = 0
	case "esc":
		m.open = false
	case "up", "k":
		if m.open {
			items := menuBar[m.bar].items
			m.sel = (m.sel - 1 + len(items)) % len(items)
		}
	case "down", "j":
		if !m.open {
			m.open, m.sel = true, 0
		} else {
			items := menuBar[m.bar].items
			m.sel = (m.sel + 1) % len(items)
		}
	case "enter", " ":
		if !m.open {
			m.open, m.sel = true, 0
			return m, nil
		}
		return m.run(menuBar[m.bar].items[m.sel].id)
	}
	return m, nil
}

func (m menu) onPickerKey(key string) (tea.Model, tea.Cmd) {
	p := m.picker
	switch key {
	case "esc", "q":
		m.picker = nil
		m.status = ""
	case "up", "k":
		p.cursor = (p.cursor - 1 + len(p.decks)) % len(p.decks)
	case "down", "j":
		p.cursor = (p.cursor + 1) % len(p.decks)
	case "enter", " ":
		deck := p.decks[p.cursor]
		purpose := p.purpose
		m.picker = nil
		m.status = ""
		if purpose == itemCards {
			m.busy = true
			return m, m.loadCards(deck)
		}
		m.study = studyAction{
			wanted: true,
			req:    api.SessionRequest{Mode: "deck", DeckID: deck.ID},
			title:  deck.Name,
		}
		if purpose == itemStudyDeckRev {
			m.study.req.Direction = api.MeaningToText
		}
		return m, tea.Quit
	}
	return m, nil
}

func (m menu) run(id itemID) (tea.Model, tea.Cmd) {
	m.open = false
	m.status = ""
	switch id {
	case itemQuit:
		return m, tea.Quit
	case itemHelp:
		m.content = helpText
	case itemHealth:
		m.busy = true
		return m, m.checkHealth()
	case itemLoginGitHub, itemLoginGoogle:
		if m.auth == nil {
			m.status = "이 화면에서는 로그인을 쓸 수 없습니다."
			return m, nil
		}
		provider := "github"
		if id == itemLoginGoogle {
			provider = "google"
		}
		m.busy = true
		m.content = "로그인을 준비하는 중…"
		return m, m.beginLogin(provider)
	case itemLogout:
		if m.auth == nil {
			m.status = "이 화면에서는 로그인을 쓸 수 없습니다."
			return m, nil
		}
		m.busy = true
		return m, m.logout()
	case itemDue:
		m.busy = true
		return m, m.loadDue()
	case itemDecks, itemCards, itemStudyDeck, itemStudyDeckRev:
		m.busy = true
		return m, m.loadDecks(id)
	case itemStudyDue, itemStudyDueRev:
		m.study = studyAction{
			wanted: true,
			req:    api.SessionRequest{Mode: "due"},
			title:  "오늘 복습",
		}
		if id == itemStudyDueRev {
			m.study.req.Direction = api.MeaningToText
		}
		return m, tea.Quit
	}
	return m, nil
}

// 서버를 부르는 동안 화면이 멈추지 않도록 tea.Cmd로 넘긴다.

func (m menu) loadDecks(purpose itemID) tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		decks, err := c.ListDecks(ctx)
		return decksMsg{decks: decks, purpose: purpose, err: err}
	}
}

func (m menu) loadCards(deck api.Deck) tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cards, err := c.ListDeckCards(ctx, deck.Slug)
		return cardsMsg{name: deck.Name, cards: cards, err: err}
	}
}

func (m menu) loadDue() tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		n, err := c.DueCount(ctx)
		return dueMsg{count: n, err: err}
	}
}

func (m menu) beginLogin(provider string) tea.Cmd {
	a := m.auth
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		url, wait, err := a.Begin(ctx, provider)
		return loginStartedMsg{url: url, wait: wait, err: err}
	}
}

func (m menu) openBrowser(url string) tea.Cmd {
	a := m.auth
	return func() tea.Msg {
		_ = a.OpenBrowser(url) // 못 열어도 주소를 화면에 보여 줬으니 넘어간다
		return nil
	}
}

// waitLogin은 브라우저가 돌아올 때까지 기다린 뒤, 새 토큰으로 사용자 이름을
// 받아 온다. 클라이언트가 저장소에서 토큰을 읽으므로 따로 건넬 것이 없다.
func (m menu) waitLogin(ctx context.Context, wait func(context.Context) error) tea.Cmd {
	c := m.client
	return func() tea.Msg {
		if err := wait(ctx); err != nil {
			return loginDoneMsg{err: err}
		}
		name := "알 수 없는 사용자"
		if me, err := c.Me(ctx); err == nil && me.DisplayName != nil && *me.DisplayName != "" {
			name = *me.DisplayName
		}
		return loginDoneMsg{name: name}
	}
}

func (m menu) logout() tea.Cmd {
	a := m.auth
	return func() tea.Msg {
		had, err := a.Logout()
		return logoutMsg{had: had, err: err}
	}
}

func (m menu) checkHealth() tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return healthMsg{err: c.Healthz(ctx)}
	}
}

const helpText = `조작

  ←/→    메뉴 옮기기
  ↑/↓    항목 옮기기(메뉴가 닫혀 있으면 ↓로 편다)
  enter  고르기
  esc    펼친 메뉴 닫기
  q      끝내기

로그인

  프로그램 ▸ GitHub로 로그인 / Google로 로그인이 브라우저를 연다.
  로그인은 설정 디렉터리에 저장돼 다음 실행부터 자동으로 쓴다.

학습 화면

  space 뒤집기 · o 맞혔다 · x 틀렸다 · q 그만

다른 모드

  flashcard decks   명령 모드(명령과 옵션을 한 번에)
  flashcard shell   셸 모드(명령을 한 줄씩)`

var (
	menuBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("7")).
			Foreground(lipgloss.Color("0"))
	menuActiveStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("4")).
			Foreground(lipgloss.Color("15")).
			Bold(true)
	dropStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("6")).
			Padding(0, 1)
	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("8")).
			Padding(0, 1)
	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("6")).
			Foreground(lipgloss.Color("0"))
)

func (m menu) View() string {
	var b strings.Builder
	b.WriteString(m.renderBar())
	b.WriteString("\n")

	if m.open {
		b.WriteString(indent(m.renderDrop(), m.dropOffset()))
		b.WriteString("\n")
	}

	body := m.content
	if m.picker != nil {
		body = m.renderPicker()
	}
	b.WriteString(paneStyle.Width(m.paneWidth()).Render(m.fit(body)))
	b.WriteString("\n")

	status := m.status
	if m.busy && m.loginCancel == nil {
		status = "서버에 묻는 중…"
	}
	b.WriteString(dimStyle.Render(status))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("←→ 메뉴 · ↑↓ 항목 · enter 고르기 · esc 닫기 · q 끝내기"))
	b.WriteString("\n")
	return b.String()
}

func (m menu) renderBar() string {
	var b strings.Builder
	b.WriteString(menuBarStyle.Render(" "))
	for i, def := range menuBar {
		label := " " + def.title + " "
		if i == m.bar {
			b.WriteString(menuActiveStyle.Render(label))
		} else {
			b.WriteString(menuBarStyle.Render(label))
		}
	}
	// 남은 칸도 같은 바탕으로 채워 메뉴 바처럼 보이게 한다.
	if rest := m.width - lipgloss.Width(b.String()); rest > 0 {
		b.WriteString(menuBarStyle.Render(strings.Repeat(" ", rest)))
	}
	return b.String()
}

// dropOffset은 펼친 메뉴가 시작할 가로 위치다(메뉴 바의 제목 아래).
func (m menu) dropOffset() int {
	x := 1
	for i := 0; i < m.bar; i++ {
		x += lipgloss.Width(menuBar[i].title) + 2
	}
	return x
}

func (m menu) renderDrop() string {
	items := menuBar[m.bar].items
	width := 0
	for _, it := range items {
		if w := lipgloss.Width(it.label); w > width {
			width = w
		}
	}
	lines := make([]string, len(items))
	for i, it := range items {
		label := it.label + strings.Repeat(" ", width-lipgloss.Width(it.label))
		if i == m.sel {
			label = selectedStyle.Render(label)
		}
		lines[i] = label
	}
	return dropStyle.Render(strings.Join(lines, "\n"))
}

func (m menu) renderPicker() string {
	p := m.picker
	lines := []string{"덱 고르기", ""}
	for i, d := range p.decks {
		line := fmt.Sprintf("%-24s %3d장", d.Name, d.CardCount)
		if i == p.cursor {
			line = selectedStyle.Render(line)
		} else {
			line = " " + line
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (m menu) paneWidth() int {
	return max(20, min(76, m.width-4))
}

// fit은 내용 칸이 화면을 넘치지 않도록 줄 수를 자른다.
func (m menu) fit(s string) string {
	limit := max(3, m.height-12)
	lines := strings.Split(s, "\n")
	if len(lines) <= limit {
		return s
	}
	kept := append([]string{}, lines[:limit]...)
	kept = append(kept, dimStyle.Render(fmt.Sprintf("… %d줄 더", len(lines)-limit)))
	return strings.Join(kept, "\n")
}

func indent(s string, n int) string {
	pad := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = pad + line
	}
	return strings.Join(lines, "\n")
}
