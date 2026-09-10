// 한글 입력 스파이크
//
// 확인하려는 것 딱 하나 — 터미널 입력창에 한국어를 칠 때
// 글자가 깨지지 않고, 커서가 글자를 따라가고, 지우기가 정확한가.
//
// 여기서 통과하면 이 입력창 위에 S1 재생 뷰를 쌓아 올린다.
// 깨지면 Ink 로 선회한다.
package main

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"
)

var (
	frame = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1)

	label = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	value = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	ok    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	dim   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

type model struct {
	ta        textarea.Model
	submitted []string
	width     int
}

func newModel() model {
	ta := textarea.New()
	ta.Placeholder = "여기에 한국어를 쳐보세요 — 예: 새벽 코딩용, 가사 없는 걸로"
	ta.SetWidth(60)
	ta.SetHeight(3)
	ta.CharLimit = 500
	ta.ShowLineNumbers = false
	ta.Focus()
	return model{ta: ta}
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		w := msg.Width - 6
		if w > 76 {
			w = 76
		}
		if w < 20 {
			w = 20
		}
		m.ta.SetWidth(w)
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			text := strings.TrimSpace(m.ta.Value())
			if text != "" {
				m.submitted = append(m.submitted, text)
				m.ta.Reset()
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.ta, cmd = m.ta.Update(msg)
	return m, cmd
}

func (m model) View() tea.View {
	cur := m.ta.Value()

	var b strings.Builder

	b.WriteString(title.Render("한글 입력 스파이크"))
	b.WriteString("\n")
	b.WriteString(dim.Render("Enter 전송 · Esc 종료 · 터미널 폭 " + fmt.Sprint(m.width)))
	b.WriteString("\n\n")

	b.WriteString(frame.Render(m.ta.View()))
	b.WriteString("\n\n")

	// 폭 계산이 맞는지 눈으로 확인할 수 있게 지표를 같이 보여준다.
	b.WriteString(label.Render("바이트 ") + value.Render(fmt.Sprint(len(cur))))
	b.WriteString(label.Render("   글자(rune) ") + value.Render(fmt.Sprint(len([]rune(cur)))))
	b.WriteString(label.Render("   표시폭 ") + value.Render(fmt.Sprint(runewidth.StringWidth(cur))))
	b.WriteString("\n\n")

	// 한글은 폭 2, 영문은 폭 1. 아래 두 줄의 끝이 정확히 맞아야 한다.
	b.WriteString(label.Render("폭 계산 검증 — 아래 두 줄의 오른쪽 끝이 맞아야 합니다"))
	b.WriteString("\n")
	b.WriteString(value.Render("  |새벽 코딩용, 가사 없는 걸로|"))
	b.WriteString("\n")
	b.WriteString(value.Render("  |" + strings.Repeat("-", runewidth.StringWidth("새벽 코딩용, 가사 없는 걸로")) + "|"))
	b.WriteString("\n\n")

	if len(m.submitted) > 0 {
		b.WriteString(label.Render("전송된 것"))
		b.WriteString("\n")
		for i, s := range m.submitted {
			b.WriteString(ok.Render(fmt.Sprintf("  %d. %s", i+1, s)))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(dim.Render("확인할 것: ①글자가 깨지지 않는가 ②커서가 글자를 따라가는가 ③지우기가 한 글자씩 되는가"))
	b.WriteString("\n")

	return tea.NewView(b.String())
}

func main() {
	p := tea.NewProgram(newModel())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "실행 실패:", err)
		os.Exit(1)
	}
}
