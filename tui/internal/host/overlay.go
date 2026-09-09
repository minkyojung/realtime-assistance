package host

import (
	"fmt"
	"strings"

	"amcli/tui/internal/app"
	"amcli/tui/internal/style"
	tea "charm.land/bubbletea/v2"
)

// 명령 팔레트와 도움말.
//
// 둘 다 본문 자리를 잠시 빌려 쓴다. 오버레이 창을 따로 만들지 않는다.
// "화면에 상시로 자리를 주지 않는다"는 원칙에서 나온 결정이다.

// 호스트 자신의 명령. 앱에 속하지 않는 것들이다.
func (m Model) hostCommands() []app.Command {
	return []app.Command{
		{Name: "/cost", Help: "what this session has spent",
			Run: func(string) tea.Cmd { return func() tea.Msg { return showCostMsg{} } }},
		{Name: "/help", Help: "keys and commands",
			Run: func(string) tea.Cmd { return func() tea.Msg { return showHelpMsg{} } }},
	}
}

type (
	showHelpMsg struct{}
	showCostMsg struct{}
)

// 호스트 명령과 지금 앱의 명령을 합친다. 사용자에게는 하나로 보인다.
func (m Model) allCommands() []app.Command {
	return append(m.app().Commands(), m.hostCommands()...)
}

// 입력한 것으로 시작하는 명령만 남긴다.
func (m Model) matchedCommands() []app.Command {
	typed := strings.ToLower(strings.Fields(m.input.Value() + " ")[0])
	var out []app.Command
	for _, c := range m.allCommands() {
		if strings.HasPrefix(c.Name, typed) {
			out = append(out, c)
		}
	}
	return out
}

func (m Model) overlayCount() int {
	if m.showHelp {
		return len(helpRows)
	}
	return len(m.matchedCommands())
}

func (m Model) runCommand() (tea.Model, tea.Cmd) {
	line := m.input.Value()
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return m, nil
	}
	name, arg := fields[0], strings.TrimSpace(strings.TrimPrefix(line, fields[0]))

	// 목록에서 고른 것이 있으면 그것을 쓴다. 인자를 이미 쳤으면 친 것을 존중한다.
	if cs := m.matchedCommands(); len(cs) > 0 && m.pick < len(cs) {
		if picked := cs[m.pick]; !strings.HasPrefix(line, picked.Name+" ") {
			name, arg = picked.Name, ""
		}
	}

	// 인자가 필요한데 없으면 이름만 남겨 두고 기다린다.
	for _, c := range m.allCommands() {
		if c.Name != name {
			continue
		}
		if c.Arg != "" && arg == "" {
			m.input.SetValue(c.Name + " ")
			m.input.CursorEnd()
			return m, nil
		}
		m.input.Reset()
		m.notice = ""
		m.pick = 0
		if c.Name == "/help" {
			m.showHelp = true
			return m, nil
		}
		return m.forward(runResultMsg{cmd: c.Run(arg)})
	}

	m.input.Reset()
	m.notice = "No such command: " + name
	return m, nil
}

// runResultMsg 는 명령이 만든 Cmd 를 앱 쪽으로 흘려보내기 위한 껍데기다.
type runResultMsg struct{ cmd tea.Cmd }

// 팔레트는 입력창 바로 아래에 뜬다. 본문을 밀어내지 않는다.
//
// 이전에는 본문 자리를 통째로 빌려 썼는데, 명령을 고르는 동안 보고 있던
// 목록이 사라져 맥락을 잃었다. 눈도 화면 위아래를 왕복해야 했다.
// 입력창 아래에 붙이면 둘 다 사라진다 — agentic CLI 의 관례이기도 하다.
const maxOverlayRows = 8

func (m Model) overlayRows(w int) []string {
	if m.showHelp {
		out := make([]string, 0, len(helpRows))
		for i, r := range helpRows {
			out = append(out, m.renderHelp(i, r, w))
		}
		return out
	}
	if !m.commanding() {
		return nil
	}
	cs := m.matchedCommands()
	if len(cs) == 0 {
		return []string{"  " + style.Faint.Render("No such command")}
	}
	if len(cs) > maxOverlayRows {
		cs = cs[:maxOverlayRows]
	}
	out := make([]string, 0, len(cs))
	for i, c := range cs {
		out = append(out, m.renderCommand(i, c, w))
	}
	return out
}

func (m Model) renderCommand(i int, c app.Command, w int) string {
	rail := "  "
	if i == m.pick {
		rail = style.Brand.Render("▌ ")
	}
	left := style.Body.Render(c.Name)
	if c.Arg != "" {
		left += style.Faint.Render(" " + c.Arg)
	}
	return rail + style.Row(left, style.Faint.Render(c.Help), w-2)
}

var helpRows = []struct{ key, what string }{
	{"type", "ask for a queue in your own words"},
	{"enter", "send the request · play the selected track"},
	{"/", "commands"},
	{"ctrl+f", "search"},
	{"ctrl+j", "expand the log"},
	{"↑ ↓", "move through the list"},
	{"tab", "next section"},
	{"shift+← →", "previous / next track"},
	{"esc", "back out one step"},
	{"ctrl+c", "quit"},
}

func (m Model) renderHelp(i int, r struct{ key, what string }, w int) string {
	return "  " + style.Row(style.BrandSoft.Render(r.key), style.Faint.Render(r.what), w-2)
}

var _ = fmt.Sprint
