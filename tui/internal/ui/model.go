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

type Model struct {
	sections   []section
	sectionIdx int

	listIdx int
	listTop int

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

	ta.Focus() // 입력창은 늘 활성이다. 타이핑이 언제나 먼저 온다.

	m := Model{
		sections: buildSections(l),
		input:    ta,
		playing:  true,
	}
	// 시작 상태는 사람이 목록에서 직접 고른 것과 같다 — 근거가 없다.
	// 근거 줄은 의도 층(자연어 요청)이 만들어낸 곡에서만 나타난다.
	if songs := l.RecentlyAdded(); len(songs) > 0 {
		m.listIdx = 0
		m = m.playSelected()
		// 재생 위치는 곡 길이에 비례해 잡는다. 고정값을 쓰면
		// 짧은 곡에서 남은 시간이 음수가 된다.
		m.positionMs = songs[0].DurationMs * 2 / 5
	}
	return m
}

func (m Model) Init() tea.Cmd { return textarea.Blink }

// searching — 입력창이 `/` 로 시작하지 않는 글자로 채워져 있고 포커스가
// 입력창에 있으면 목록을 즉시 걸러 보여준다. 별도 검색 화면을 두지 않는 이유다.
func (m Model) searching() bool {
	v := strings.TrimSpace(m.input.Value())
	return v != "" && !strings.HasPrefix(v, "/")
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.input.SetWidth(contentWidth(m.w))
		return m, nil

	case tea.KeyPressMsg:
		// 입력창은 늘 활성이므로, 여기서 가로채는 키만 목록·섹션 조작이다.
		// 글자는 전부 입력창으로 흘려보낸다.
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.input.Value() != "" {
				m.input.Reset()
				m.clampList()
				return m, nil
			}
			return m, tea.Quit

		case "up", "ctrl+p":
			m.listIdx = clamp(m.listIdx-1, 0, maxInt(m.rowCount()-1, 0))
			m.clampList()
			return m, nil
		case "down", "ctrl+n":
			m.listIdx = clamp(m.listIdx+1, 0, maxInt(m.rowCount()-1, 0))
			m.clampList()
			return m, nil

		case "tab":
			m.sectionIdx = (m.sectionIdx + 1) % len(m.sections)
			m.listIdx, m.listTop = 0, 0
			return m, nil
		case "shift+tab":
			m.sectionIdx = (m.sectionIdx - 1 + len(m.sections)) % len(m.sections)
			m.listIdx, m.listTop = 0, 0
			return m, nil

		case "enter":
			// 입력이 있으면 요청, 없으면 선택한 곡을 튼다.
			if strings.TrimSpace(m.input.Value()) != "" {
				m.input.Reset()
				m.clampList()
				return m, nil
			}
			return m.playSelected(), nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.clampList()
	return m, cmd
}

// 목록에서 고른 곡을 재생 중으로 만든다. 서버 연동 전까지는 로컬 상태만 바꾼다.
func (m Model) playSelected() Model {
	rows := m.rows()
	if m.listIdx < 0 || m.listIdx >= len(rows) || rows[m.listIdx].track == nil {
		return m
	}
	t := *rows[m.listIdx].track
	m.nowPlayingID = t.Id
	m.positionMs = 0
	m.playing = true
	// 큐에 없는 곡이면 근거 없이 들어간다 — 사람이 직접 고른 것이다.
	found := false
	for _, it := range m.queue {
		if it.Track.Id == t.Id {
			found = true
			break
		}
	}
	if !found {
		m.queue = append([]api.QueueItem{{
			Id: t.Id, SessionId: 1, Position: 0, Track: t,
			State: api.Playing, Origin: api.QueueItemOriginManual,
		}}, m.queue...)
	}
	return m
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

// 한 번에 보여줄 목록 줄 수의 상한.
// 화면을 꽉 채우면 읽을 게 아니라 스캔할 것이 되어버린다.
const maxListRows = 14

// 세로 예산: 재생바1 + 룰1 + 목록 + 룰1 + 근거(0|1) + 입력1 + 여백2
func (m Model) listHeight() int {
	reserved := 6
	if _, ok := m.viewReason(10); ok {
		reserved++
	}
	h := maxInt(m.h-reserved, 3)
	return minInt(h, maxListRows)
}

func (m Model) View() tea.View {
	if m.w == 0 || m.h == 0 {
		return tea.NewView("")
	}
	w := contentWidth(m.w)
	listW := w - sidebarWidth - 1
	listH := m.listHeight()

	var b strings.Builder
	// 지금 재생 중인 곡이 화면의 첫 줄이다.
	// 섹션 제목과 곡 수는 뺐다 — 사이드바의 레일이 이미 어디인지 말해준다.
	b.WriteString(m.viewPlayer(w))
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
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
