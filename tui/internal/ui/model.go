package ui

import (
	"strings"

	"amcli/tui/internal/api"
	"amcli/tui/internal/data"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// 화면 구조 — Apple Music 과 같은 모양.
//
//	┌──────────┬──────────────────────────┐
//	│ 사이드바  │  목록                     │
//	├──────────┴──────────────────────────┤
//	│ 입력창 (자연어 · / 명령 · 검색)        │
//	├─────────────────────────────────────┤
//	│ 재생 바 (항상 보임)                   │
//	└─────────────────────────────────────┘
//
// 사이드바가 무엇을 고르든 목록 패널 하나가 다 그린다. 그래서 화면이 늘지 않는다.

type focusArea int

const (
	focusList focusArea = iota
	focusSidebar
	focusInput
)

type Model struct {
	sections   []section
	sectionIdx int

	listIdx int
	listTop int

	focus focusArea
	input textarea.Model

	// 재생 상태. 서버 연동 전까지는 로컬 상태다.
	queue        []api.QueueItem
	nowPlayingID int64
	positionMs   int
	playing      bool

	usage api.Usage
	w, h  int
}

func New() Model {
	l := data.Lib()

	ta := textarea.New()
	ta.Placeholder = "What do you want to hear?    /  commands     ?  help"
	ta.SetHeight(1)
	ta.CharLimit = 500
	ta.ShowLineNumbers = false
	ta.Prompt = "› "
	styleInput(&ta)

	m := Model{
		sections: buildSections(l),
		input:    ta,
		focus:    focusList,
		queue:    data.Queue(),
		playing:  true,
		usage:    api.Usage{PromptTokens: 1240, CompletionTokens: 380, CostUsd: 0.0031},
	}
	if len(m.queue) > 0 {
		m.nowPlayingID = m.queue[0].Track.Id
		m.positionMs = data.PlaybackPositionMs
	}
	return m
}

func (m Model) Init() tea.Cmd { return textarea.Blink }

// searching — 입력창이 `/` 로 시작하지 않는 글자로 채워져 있고 포커스가
// 입력창에 있으면 목록을 즉시 걸러 보여준다. 별도 검색 화면을 두지 않는 이유다.
func (m Model) searching() bool {
	v := strings.TrimSpace(m.input.Value())
	return m.focus == focusInput && v != "" && !strings.HasPrefix(v, "/")
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.input.SetWidth(contentWidth(m.w))
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.focus == focusInput {
				m.input.Reset()
				m.focus = focusList
				return m, nil
			}
			return m, tea.Quit
		case "tab":
			m.focus = (m.focus + 1) % 3
			return m, nil
		case "enter":
			if m.focus == focusInput {
				m.input.Reset()
				m.focus = focusList
			}
			return m, nil
		}

		if m.focus != focusInput {
			if handled, mm := m.handleNav(msg.String()); handled {
				return mm, nil
			}
			// 글자를 치면 곧바로 입력창으로 넘어간다.
			if len(msg.String()) == 1 || msg.String() == "/" {
				m.focus = focusInput
			}
		}
	}

	if m.focus == focusInput {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.clampList()
		return m, cmd
	}
	return m, nil
}

func (m Model) handleNav(key string) (bool, Model) {
	switch key {
	case "up", "k":
		if m.focus == focusSidebar {
			m.sectionIdx = clamp(m.sectionIdx-1, 0, len(m.sections)-1)
			m.listIdx, m.listTop = 0, 0
		} else {
			m.listIdx = clamp(m.listIdx-1, 0, maxInt(m.rowCount()-1, 0))
		}
		m.clampList()
		return true, m
	case "down", "j":
		if m.focus == focusSidebar {
			m.sectionIdx = clamp(m.sectionIdx+1, 0, len(m.sections)-1)
			m.listIdx, m.listTop = 0, 0
		} else {
			m.listIdx = clamp(m.listIdx+1, 0, maxInt(m.rowCount()-1, 0))
		}
		m.clampList()
		return true, m
	case "left", "h":
		m.focus = focusSidebar
		return true, m
	case "right", "l":
		m.focus = focusList
		return true, m
	case " ":
		m.playing = !m.playing
		return true, m
	}
	return false, m
}

// clampList — 선택 행이 늘 보이도록 스크롤 위치를 맞춘다.
func (m *Model) clampList() {
	h := m.listHeight()
	n := m.rowCount()
	m.listIdx = clamp(m.listIdx, 0, maxInt(n-1, 0))
	if m.listIdx < m.listTop {
		m.listTop = m.listIdx
	}
	if m.listIdx >= m.listTop+h {
		m.listTop = m.listIdx - h + 1
	}
	m.listTop = clamp(m.listTop, 0, maxInt(n-h, 0))
}

// 세로 예산: 헤더1 + 룰1 + 목록 + 룰1 + 근거(0|1) + 입력1 + 룰1 + 재생바1 + 여백2
func (m Model) listHeight() int {
	reserved := 8
	if _, ok := m.viewReason(10); ok {
		reserved++
	}
	return maxInt(m.h-reserved, 3)
}

func (m Model) View() tea.View {
	if m.w == 0 || m.h == 0 {
		return tea.NewView("")
	}
	w := contentWidth(m.w)
	listW := w - sidebarWidth - 1
	listH := m.listHeight()

	var b strings.Builder
	// 제목줄은 목록 쪽에만 둔다. 사이드바의 LIBRARY 와 겹치지 않게.
	b.WriteString(strings.Repeat(" ", sidebarWidth+1) + m.listHeader(listW))
	b.WriteString("\n")
	b.WriteString(ruleBrand(w))
	b.WriteString("\n")

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		m.viewSidebar(listH),
		stRule.Render(strings.Repeat("│\n", listH)),
		m.viewList(listW, listH))
	b.WriteString(body)
	b.WriteString("\n")
	b.WriteString(rule(w))
	b.WriteString("\n")

	if reason, ok := m.viewReason(w); ok {
		b.WriteString(reason)
		b.WriteString("\n")
	}
	b.WriteString(m.input.View())
	b.WriteString("\n")
	b.WriteString(rule(w))
	b.WriteString("\n")
	b.WriteString(m.viewPlayer(w))

	v := tea.NewView(lipgloss.NewStyle().Padding(1, 1).Render(b.String()))
	v.AltScreen = true
	v.WindowTitle = "Apple Music CLI"
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
		st.Prompt = lipgloss.NewStyle().Foreground(colBrand)
		st.Placeholder = lipgloss.NewStyle().Foreground(colFaint)
	}
	styles.Cursor.Color = colBrand
	ta.SetStyles(styles)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
