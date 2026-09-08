package ui

import (
	"fmt"
	"strings"

	"amcli/tui/internal/api"
	"amcli/tui/internal/data"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Screen 은 본문에 무엇을 그릴지 정한다. 슬래시 명령이 이 값을 바꾼다.
type Screen int

const (
	ScreenPlaying Screen = iota // S1
)

type Model struct {
	screen  Screen
	session api.Session
	input   textarea.Model
	w, h    int
}

func New() Model {
	ta := textarea.New()
	ta.Placeholder = "무엇을 들을까요?   / 명령   ? 도움말"
	ta.SetHeight(1)
	ta.CharLimit = 500
	ta.ShowLineNumbers = false
	ta.Prompt = "› "
	styleInput(&ta)
	// 스크린샷용으로 재편성 요청을 미리 채워 둔다. 지우고 쓰면 된다.
	ta.SetValue("좀 더 조용한 걸로")
	ta.Focus()

	return Model{
		screen:  ScreenPlaying,
		session: data.Session(),
		input:   ta,
	}
}

func (m Model) Init() tea.Cmd { return textarea.Blink }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.input.SetWidth(contentWidth(m.w))
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) View() tea.View {
	if m.w == 0 {
		return tea.NewView("")
	}
	s := m.session
	w := contentWidth(m.w)

	var b strings.Builder
	b.WriteString(header(s, w))
	b.WriteString("\n")
	b.WriteString(rule(w))
	b.WriteString("\n")

	switch m.screen {
	case ScreenPlaying:
		b.WriteString(viewPlaying(s, w))
	}

	b.WriteString("\n")
	b.WriteString(rule(w))
	b.WriteString("\n")
	b.WriteString(m.input.View())
	b.WriteString("\n")
	b.WriteString(rule(w))
	b.WriteString("\n")
	b.WriteString(statusBar(s, w))

	// 좌우 여백 한 칸.
	v := tea.NewView(lipgloss.NewStyle().Padding(1, 1).Render(b.String()))
	// v2 에서 alt screen 은 옵션이 아니라 View 의 속성이다.
	v.AltScreen = true
	v.WindowTitle = s.Title
	return v
}

// textarea 기본 스타일은 배경이 검게 깔린다. 나머지 화면과 어긋나므로
// 배경을 비우고 전경색만 팔레트에 맞춘다.
func styleInput(ta *textarea.Model) {
	styles := ta.Styles()
	for _, st := range []*textarea.StyleState{&styles.Focused, &styles.Blurred} {
		st.Base = lipgloss.NewStyle()
		st.CursorLine = lipgloss.NewStyle()
		st.EndOfBuffer = lipgloss.NewStyle()
		st.Text = lipgloss.NewStyle().Foreground(colFg)
		st.Prompt = lipgloss.NewStyle().Foreground(colAccent)
		st.Placeholder = lipgloss.NewStyle().Foreground(colFaint)
	}
	ta.SetStyles(styles)
}

func header(s api.Session, w int) string {
	state := stPlaying.Render("● 재생중")
	if s.Status == api.Ended {
		state = stDim.Render("○ 종료됨")
	}
	return row(stTitle.Render(truncate(s.Title, w-12)), state, w)
}

// 상태줄 — 왼쪽은 큐 요약, 오른쪽은 누적 사용량.
// 사용량을 상시 노출하는 것은 agentic CLI 의 관례다. docs/03 참조.
func statusBar(s api.Session, w int) string {
	left := stDim.Render("큐가 비어 있음")
	if q := s.Summary; q != nil {
		left = stDim.Render(fmt.Sprintf("%d곡 · %s · ", q.TrackCount, humanMinutes(q.TotalDurationMs))) +
			stNever.Render(fmt.Sprintf("미재생 %d", q.NeverPlayedCount))
	}
	u := s.Usage
	right := stFaint.Render(fmt.Sprintf("↑%s ↓%s  $%.4f",
		tokens(u.PromptTokens), tokens(u.CompletionTokens), u.CostUsd))
	return row(left, right, w)
}
