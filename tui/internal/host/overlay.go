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
//
// 앱 전환도 여기 있다. 전용 키를 두지 않는 이유는 터미널이 ctrl+tab 을
// tab 과 구별하지 못하기 때문이다 — kitty 키보드 프로토콜이 있어야 하는데
// Terminal.app 은 지원하지 않는다. 터미널에 따라 되다 안 되다 하는 키는
// 없는 것만 못하다.
//
// 그리고 애초에 새 키가 필요 없다. "화면에 상시로 자리를 주지 않는다"는
// 원칙대로 `/` 가 갈 곳을 보여주면 된다. 사이드바를 없앤 것과 같은 이유다.
func (m Model) hostCommands() []app.Command {
	out := make([]app.Command, 0, len(m.apps)+2)
	for i, a := range m.apps {
		if !m.home && i == m.current {
			continue // 지금 보고 있는 앱으로 갈 이유는 없다
		}
		help := "switch to " + a.Name()
		if n := a.Badge(); n > 0 {
			help += fmt.Sprintf(" · %d unread", n)
		}
		out = append(out, app.Command{
			Name: "/" + a.Name(), Help: help, Run: switchTo(i),
		})
	}
	return append(out, []app.Command{
		{Name: "/cost", Help: "what this session has spent",
			Run: func(string) tea.Cmd { return func() tea.Msg { return showCostMsg{} } }},
		{Name: "/help", Help: "keys and commands",
			Run: func(string) tea.Cmd { return func() tea.Msg { return showHelpMsg{} } }},
	}...)
}

// 전환은 cmd+tab 에 가깝다. 앱은 시작할 때 전부 켜져서 끝까지 살아 있고,
// 화면만 갈아끼운다. 그래서 채팅을 보는 중에도 음악은 계속 재생된다.
func switchTo(i int) func(string) tea.Cmd {
	return func(string) tea.Cmd {
		return func() tea.Msg { return switchAppMsg{index: i} }
	}
}

type switchAppMsg struct{ index int }

type (
	showHelpMsg struct{}
	showCostMsg struct{}
)

// 호스트 명령과 지금 앱의 명령을 합친다. 사용자에게는 하나로 보인다.
//
// 앱 전환을 앞에 둔다. 다른 앱으로 가는 길이 그 앱의 명령보다 먼저다.
func (m Model) allCommands() []app.Command {
	host := m.hostCommands()
	out := make([]app.Command, 0, len(host)+4)
	for _, c := range host {
		if strings.HasPrefix(c.Name, "/") && isAppName(m, c.Name) {
			out = append(out, c)
		}
	}
	// 홈에서는 앱 명령을 내놓지 않는다. 들어가지도 않은 앱의 /queue 를
	// 실행하면 화면은 홈인데 큐만 바뀌어 있다.
	if !m.home {
		out = append(out, m.app().Commands()...)
	}
	for _, c := range host {
		if !isAppName(m, c.Name) {
			out = append(out, c)
		}
	}
	return out
}

func isAppName(m Model, cmd string) bool {
	for _, a := range m.apps {
		if cmd == "/"+a.Name() {
			return true
		}
	}
	return false
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

// 고를 수 있는 줄만 센다. 보이지 않는 줄에 커서가 서면 ↓ 를 눌러도
// 아무 일도 안 일어나고, enter 는 본 적 없는 명령을 실행한다.
func (m Model) overlayCount() int {
	if m.showHelp {
		return len(helpRows)
	}
	n := len(m.matchedCommands())
	if n > maxOverlayRows {
		return maxOverlayRows - 1 // 마지막 줄은 "+N more" 다
	}
	return n
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
		// 명령이 만든 Cmd 를 그대로 돌려준다. 그것이 뱉는 메시지는
		// 호스트 것이면 호스트가 처리하고, 앱 것이면 앱으로 흘러간다.
		return m, c.Run(arg)
	}

	m.input.Reset()
	m.log = append(m.log, logEntry{who: "host", text: "No such command: " + name, err: true})
	return m, nil
}

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
	// 잘린 것이 있으면 마지막 줄로 말한다. 말없이 자르면 없는 명령으로 보인다.
	more := 0
	if len(cs) > maxOverlayRows {
		more = len(cs) - maxOverlayRows + 1
		cs = cs[:maxOverlayRows-1]
	}
	out := make([]string, 0, len(cs)+1)
	for i, c := range cs {
		out = append(out, m.renderCommand(i, c, w))
	}
	if more > 0 {
		out = append(out, "  "+style.Faint.Render(fmt.Sprintf("+%d more — keep typing", more)))
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
	{"enter", "send the request · play or open what is selected"},
	{"/", "commands"},
	{"shift+tab", "switch between ask and search"},
	{"ctrl+f", "search"},
	{"ctrl+j", "flip the panel — lyrics ⇄ what you asked"},
	{"ctrl+o", "show what the last request did"},
	{"↑ ↓", "move through the list"},
	{"tab", "next section · / goes straight to one"},
	{"shift+← ↓ →", "previous · play / pause · next"},
	{"esc", "back out one step"},
	{"ctrl+c", "quit"},
}

func (m Model) renderHelp(i int, r struct{ key, what string }, w int) string {
	return "  " + style.Row(style.BrandSoft.Render(r.key), style.Faint.Render(r.what), w-2)
}
