// Package host 는 터미널 안에 앱들을 담는 껍데기다.
//
// 두 줄만 갖는다 — 입력창과 상태줄. 그 위는 전부 앱의 것이다.
// 파는 것은 화면이 아니라 조작법이다. docs/07-호스트-계약.md
package host

import (
	"strings"

	"amcli/tui/internal/app"
	"amcli/tui/internal/style"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// 입력창은 하나지만 하는 일이 둘이다. 무엇을 하는 중인지 화면이 말해야 한다.
//
//	기본     › 프롬프트 — 자연어 요청. 목록을 건드리지 않는다
//	ctrl+f   ⌕ 검색   — 지금 보고 있는 것을 즉시 거른다
//	/        › 명령    — 로컬에서 바로 실행
type inputMode int

const (
	modePrompt inputMode = iota
	modeSearch
)

type Model struct {
	apps    []app.App
	current int

	input    textarea.Model
	mode     inputMode
	showHelp bool
	notice   string

	// 팔레트·도움말에서 고른 줄.
	pick int

	// 로그 — 호스트의 세 번째 자산. 입력의 짝이다.
	log     []logEntry
	logOpen bool
	routing bool            // 라우터의 답을 기다리는 중
	pending map[string]bool // 답을 기다리는 앱
	spinner spinner.Model

	w, h int
	send func(tea.Msg)
}

func New(apps ...app.App) Model {
	ta := textarea.New()
	ta.SetHeight(1)
	ta.CharLimit = 500
	ta.ShowLineNumbers = false
	styleInput(&ta)
	ta.Focus() // 입력창은 늘 활성이다. 타이핑이 언제나 먼저 온다.

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(style.ColBrand)

	m := Model{apps: apps, input: ta, spinner: sp, pending: map[string]bool{}}
	(&m).applyMode()
	return m
}

func (m Model) app() app.App { return m.apps[m.current] }

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{textarea.Blink, m.spinner.Tick}
	for _, a := range m.apps {
		cmds = append(cmds, a.Init(m.push))
	}
	return tea.Batch(cmds...)
}

// push 는 앱이 이벤트 루프 밖에서 메시지를 넣는 통로다.
// 폴링이 아니라 밀어 넣는 앱(채팅 등)에 필요하다.
func (m Model) push(msg tea.Msg) {
	if m.send != nil {
		m.send(msg)
	}
}

// SetSend 는 tea.Program 이 만들어진 뒤 주입한다.
func (m *Model) SetSend(f func(tea.Msg)) { m.send = f }

// commanding — 입력이 `/` 로 시작하면 명령 모드다. 별도 상태를 두지 않는다.
func (m Model) commanding() bool {
	return m.mode == modePrompt && strings.HasPrefix(m.input.Value(), "/")
}

// overlaying — 본문 자리를 호스트가 잠시 빌려 쓰는 중인가.
func (m Model) overlaying() bool { return m.showHelp || m.commanding() }

func (m *Model) applyMode() {
	if m.mode == modeSearch {
		m.input.Prompt = "⌕ "
		m.input.Placeholder = "Search your library"
	} else {
		m.input.Prompt = "› "
		m.input.Placeholder = "Ask for anything    /  commands     ?  help"
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.input.SetWidth(style.ContentWidth(m.w))
		return m.forward(app.ResizeMsg{
			Width:  style.ContentWidth(m.w),
			Height: m.bodyHeight(),
		})

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case app.SayMsg:
		return m.handleSay(msg), nil

	case routedMsg:
		return m.deliver(msg)

	case switchAppMsg:
		// 화면만 갈아끼운다. 다른 앱은 계속 살아 있다.
		if msg.index >= 0 && msg.index < len(m.apps) {
			m.current = msg.index
			m.mode = modePrompt
			m.showHelp = false
			(&m).applyMode()
			return m.forward(app.ResizeMsg{
				Width:  style.ContentWidth(m.w),
				Height: m.bodyHeight(),
			})
		}
		return m, nil

	case showHelpMsg:
		m.showHelp, m.pick = true, 0
		return m, nil

	case showCostMsg:
		// 비용은 앱이 쓴 것이므로 앱의 상태줄에 이미 있다.
		// 여기서는 그것을 잠시 앞으로 끌어낸다.
		m.notice = m.app().Status()
		return m, nil

	case tea.KeyPressMsg:
		if handled, mm, cmd := m.handleKey(msg); handled {
			return mm, cmd
		}
		// 호스트가 안 쓰는 키는 입력창이 먼저 본다. 타이핑이 언제나 먼저다.
		before := m.input.Value()
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if m.input.Value() != before {
			m.pick = 0
			if m.mode == modeSearch {
				m.apps[m.current] = m.app().Filter(m.input.Value())
			}
			return m, cmd
		}
		// 입력창이 안 먹은 키만 앱에게 간다 (방향키·tab 등).
		next, appCmd := m.app().Update(msg)
		m.apps[m.current] = next
		return m, tea.Batch(cmd, appCmd)
	}
	return m.forward(msg)
}

// dispatch 는 문장을 어느 앱에게 줄지 정해 보낸다.
//
// 라우터를 부르지 않는 경우가 대부분이다 — 앱이 하나이거나, 사용자가
// @ 로 지정했으면 답이 이미 정해져 있다.
func (m Model) dispatch(prompt string) (tea.Model, tea.Cmd) {
	if name, rest := m.mention(prompt); name != "" {
		return m.ask([]string{name}, rest)
	}
	if len(m.apps) == 1 {
		return m.ask([]string{m.app().Name()}, prompt)
	}
	m.routing = true
	return m, tea.Batch(
		cmdRoute(prompt, m.specs(), m.app().Name()),
		m.spinner.Tick,
	)
}

// deliver 는 라우터의 결과를 받아 앱들에게 넘긴다.
func (m Model) deliver(msg routedMsg) (tea.Model, tea.Cmd) {
	m.routing = false
	names := msg.apps
	// 라우터가 실패하거나 아무도 못 고르면 지금 보고 있는 앱에게 준다.
	// 틀려도 망하지 않는다 — 그 앱이 못 하겠다고 로그에 남기고 끝이다.
	if msg.err != nil || len(names) == 0 {
		names = []string{m.app().Name()}
	}
	return m.ask(names, msg.prompt)
}

// ask 는 지목된 앱들에게 동시에 묻는다.
//
// 순서를 보장하지 않는다. "틀고 꺼줘"는 순서 의존이 없고, 순서가 필요한
// 요청("A 하고 나서 B")은 지금 범위 밖이다. 결과는 도착하는 대로 로그에 쌓인다.
func (m Model) ask(names []string, prompt string) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{m.spinner.Tick}
	for _, name := range names {
		for i, a := range m.apps {
			if a.Name() != name {
				continue
			}
			m.pending[name] = true
			next, cmd := a.Update(app.AskMsg{Prompt: prompt})
			m.apps[i] = next
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}
	return m, tea.Batch(cmds...)
}

// forward 는 메시지를 지금 앱에게 넘긴다.
// 타입 단언이 없다 — App.Update 가 App 을 돌려주기 때문이다.
func (m Model) forward(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.app().Update(msg)
	m.apps[m.current] = next
	return m, cmd
}

func (m Model) handleKey(msg tea.KeyPressMsg) (bool, tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return true, m, tea.Quit

	case "?":
		if m.input.Value() == "" {
			m.showHelp, m.pick = true, 0
			return true, m, nil
		}

	case "ctrl+j":
		// 로그를 펼쳤다 접는다. 명령 팔레트와 같은 자리, 같은 방식이다.
		m.logOpen = !m.logOpen
		return true, m, nil

	case "ctrl+f":
		m.showHelp = false
		m.mode = modeSearch
		m.input.Reset()
		m.applyMode()
		m.apps[m.current] = m.app().Filter("")
		return true, m, nil

	case "esc":
		// 한 단계씩 물러난다 — 도움말 → 검색 → 입력 비우기 → 종료.
		if m.showHelp {
			m.showHelp = false
			return true, m, nil
		}
		if m.logOpen {
			m.logOpen = false
			return true, m, nil
		}
		m.notice = ""
		if m.mode == modeSearch {
			m.mode = modePrompt
			m.input.Reset()
			m.applyMode()
			m.apps[m.current] = m.app().Filter("")
			return true, m, nil
		}
		if m.input.Value() != "" {
			m.input.Reset()
			return true, m, nil
		}
		return true, m, tea.Quit

	case "up", "ctrl+p":
		if m.overlaying() {
			m.pick = style.Max(m.pick-1, 0)
			return true, m, nil
		}
	case "down", "ctrl+n":
		if m.overlaying() {
			m.pick = style.Min(m.pick+1, style.Max(m.overlayCount()-1, 0))
			return true, m, nil
		}

	case "enter":
		mm, cmd := m.handleEnter()
		return true, mm, cmd
	}
	return false, m, nil
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	if m.showHelp {
		m.showHelp = false
		return m, nil
	}
	if m.commanding() {
		return m.runCommand()
	}
	// 검색 중에는 고른 것을 앱이 처리한다. 프롬프트일 때만 요청으로 보낸다.
	if m.mode == modePrompt {
		if prompt := strings.TrimSpace(m.input.Value()); prompt != "" {
			m.input.Reset()
			m.notice = ""
			m.showHelp = false
			// 한 말은 곧바로 로그에 남는다. 답을 기다리는 동안에도 보인다.
			m.log = append(m.log, logEntry{text: prompt})
			return m.dispatch(prompt)
		}
	}
	return m.forward(tea.KeyPressMsg{Code: tea.KeyEnter})
}

func (m Model) bodyHeight() int {
	// 입력창1 + 룰1 + 상태줄1 + 위아래 여백2
	w := style.ContentWidth(m.w)
	h := m.h - 5 - len(m.overlayRows(w)) - len(m.logRows(w))
	return style.Max(h, 3)
}

func (m Model) View() tea.View {
	if m.w == 0 || m.h == 0 {
		return tea.NewView("")
	}
	w := style.ContentWidth(m.w)
	bodyH := m.bodyHeight()

	var b strings.Builder
	// 본문은 무엇을 하든 그대로다. 팔레트는 입력창 아래에 붙는다.
	b.WriteString(m.app().View(w, bodyH))
	// 로그는 입력창 바로 위, 팔레트는 바로 아래. 둘 다 본문을 밀어내지 않는다.
	for _, r := range m.logRows(w) {
		b.WriteString("\n")
		b.WriteString(r)
	}
	b.WriteString("\n")
	b.WriteString(m.input.View())
	for _, r := range m.overlayRows(w) {
		b.WriteString("\n")
		b.WriteString(r)
	}
	b.WriteString("\n")
	b.WriteString(style.Rule(w))
	b.WriteString("\n")
	b.WriteString(m.viewStatus(w))

	v := tea.NewView(lipgloss.NewStyle().Padding(1, 1).Render(b.String()))
	v.AltScreen = true
	v.WindowTitle = "Apple Music CLI"
	return v
}

// 상태줄 — 지금 앱이 말하는 것과, 배경 앱들이 말하는 것을 모은다.
func (m Model) viewStatus(w int) string {
	left := m.app().Status()
	if m.notice != "" {
		left = style.BrandSoft.Render("· ") + style.Dim.Render(style.Truncate(m.notice, w-2))
	}

	// 배경 앱은 이름과 배지만 내놓는다. 맥 메뉴바와 같다.
	var bg []string
	for i, a := range m.apps {
		if i == m.current {
			continue
		}
		s := a.Name()
		if n := a.Badge(); n > 0 {
			s += " " + style.Tokens(n)
		}
		bg = append(bg, s)
	}
	if len(bg) == 0 {
		return style.Truncate(left, w)
	}
	return style.Row(left, style.Faint.Render(strings.Join(bg, " · ")), w)
}

// textarea 기본 스타일은 배경이 검게 깔린다. 나머지 화면과 어긋나므로
// 배경을 비우고 전경색만 팔레트에 맞춘다.
func styleInput(ta *textarea.Model) {
	styles := ta.Styles()
	for _, st := range []*textarea.StyleState{&styles.Focused, &styles.Blurred} {
		st.Base = lipgloss.NewStyle()
		st.CursorLine = lipgloss.NewStyle()
		st.EndOfBuffer = lipgloss.NewStyle()
		st.Text = lipgloss.NewStyle().Foreground(style.ColFg)
		st.Prompt = lipgloss.NewStyle().Foreground(style.ColBrand)
		st.Placeholder = lipgloss.NewStyle().Foreground(style.ColFaint)
	}
	styles.Cursor.Color = style.ColBrand
	ta.SetStyles(styles)
}
