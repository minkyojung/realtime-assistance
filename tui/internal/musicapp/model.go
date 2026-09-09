package musicapp

import (
	"amcli/tui/internal/style"
	"errors"
	"fmt"
	"strings"

	"amcli/tui/internal/api"
	"amcli/tui/internal/data"
	"amcli/tui/internal/intent"
	"amcli/tui/internal/music"
	"charm.land/bubbles/v2/spinner"
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

// 입력창은 하나지만 하는 일이 둘이다. 무엇을 하는 중인지 화면이 말해야 한다.
//
//	기본     › 프롬프트 — 자연어 요청. 목록을 건드리지 않는다
//	ctrl+f   ⌕ 검색   — 라이브러리를 즉시 걸러 보여준다
//	/        › 명령    — 로컬에서 바로 실행
type inputMode int

const (
	modePrompt inputMode = iota
	modeSearch
)

type Model struct {
	sections   []section
	sectionIdx int
	mode       inputMode

	listIdx int
	listTop int

	input textarea.Model

	// 재생 상태는 Music.app 폴링으로 채운다. DB 에 없는 값이다.
	queue        []api.QueueItem
	nowPlayingID int64
	positionMs   int
	playing      bool

	// Music.app 이 말해주는 것 그대로. 라이브러리에 없는 곡도 여기 담긴다.
	live music.PlayerState

	// 명령·도움말은 목록 패널을 잠시 빌려 쓴다. 화면을 새로 만들지 않는다.
	showHelp bool
	notice   string

	// 의도 층 — 자연어 한 줄이 큐가 되는 경로.
	thinking   bool
	spinner    spinner.Model
	queueTitle string
	note       string
	intentErr  error
	usage      api.Usage

	// 첫 실행 관문 — 로그인이 아니라 권한과 앱 실행 여부다.
	playerErr error
	polled    bool

	w, h int
}

func New() Model {
	l := data.Lib()

	ta := textarea.New()
	ta.Placeholder = "What do you want to hear?    /  commands     ?  help"
	ta.SetHeight(1)
	ta.CharLimit = 500
	ta.ShowLineNumbers = false
	styleInput(&ta)

	ta.Focus() // 입력창은 늘 활성이다. 타이핑이 언제나 먼저 온다.

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(style.ColBrand)

	m := Model{
		sections: buildSections(l),
		input:    ta,
		spinner:  sp,
		playing:  true,
	}

	(&m).applyMode()
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick, fetchStatus, tick())
}

// searching — 입력창이 `/` 로 시작하지 않는 글자로 채워져 있고 포커스가
// 입력창에 있으면 목록을 즉시 걸러 보여준다. 별도 검색 화면을 두지 않는 이유다.
// overlaying — 목록 패널이 섹션이 아닌 다른 것을 보여주는 중인가.
func (m Model) overlaying() bool {
	return m.showHelp || m.commanding() || m.mode == modeSearch
}

func (m Model) searching() bool {
	return m.mode == modeSearch && strings.TrimSpace(m.input.Value()) != ""
}

// 입력창이 지금 무엇인지에 따라 기호와 안내 문구를 바꾼다.
func (m *Model) applyMode() {
	if m.mode == modeSearch {
		m.input.Prompt = "⌕ "
		m.input.Placeholder = "Search your library"
	} else {
		m.input.Prompt = "› "
		m.input.Placeholder = "What do you want to hear?    /  commands     ?  help"
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case savedMsg:
		if msg.err != nil {
			m.intentErr = msg.err
		} else {
			m.notice = fmt.Sprintf("Saved %q to Apple Music", msg.name)
		}
		return m, nil

	case queueMsg:
		m.thinking = false
		if msg.err != nil {
			m.intentErr = msg.err
			return m, nil
		}
		return m.applyQueue(msg.res)

	case tickMsg:
		return m, tea.Batch(fetchStatus, tick())

	case statusMsg:
		m.polled = true
		m.playerErr = msg.err
		if msg.err == nil {
			m.live = msg.state
			m.playing = msg.state.Playing
			m.positionMs = msg.state.PositionMs
			// 라이브러리에 있는 곡이면 목록에서도 표시한다.
			// 없으면(카탈로그 스트리밍) 재생 바만 그린다 — 그것도 정상 상태다.
			m.nowPlayingID = 0
			if id := msg.state.PersistentID; id != "" {
				if t, ok := data.Lib().ByPersistentID(id); ok {
					m.nowPlayingID = t.Id
					m.ensureQueued(t)
				}
			}
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.input.SetWidth(style.ContentWidth(m.w))
		return m, nil

	case tea.KeyPressMsg:
		// 입력창은 늘 활성이므로, 여기서 가로채는 키만 목록·섹션 조작이다.
		// 글자는 전부 입력창으로 흘려보낸다.
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+g":
			// 권한이 막혀 있을 때 시스템 설정을 연다.
			if m.playerErr == music.ErrPermissionDenied {
				return m, cmdOpenSettings()
			}
			return m, nil

		case "?":
			if m.input.Value() == "" {
				m.showHelp = true
				m.listIdx, m.listTop = 0, 0
				return m, nil
			}

		case "ctrl+f":
			m.showHelp = false
			m.mode = modeSearch
			m.input.Reset()
			m.applyMode()
			m.listIdx, m.listTop = 0, 0
			return m, nil

		case "esc":
			if m.showHelp {
				m.showHelp = false
				m.listIdx, m.listTop = 0, 0
				return m, nil
			}
			m.notice = ""
			// 검색 중이면 검색만 빠져나온다. 그다음 한 번 더 누르면 종료.
			if m.mode == modeSearch {
				m.mode = modePrompt
				m.input.Reset()
				m.applyMode()
				m.listIdx, m.listTop = 0, 0
				return m, nil
			}
			if m.input.Value() != "" {
				m.input.Reset()
				m.clampList()
				return m, nil
			}
			return m, tea.Quit

		case "up", "ctrl+p":
			m.listIdx = style.Clamp(m.listIdx-1, 0, style.Max(m.rowCount()-1, 0))
			m.clampList()
			return m, nil
		case "down", "ctrl+n":
			m.listIdx = style.Clamp(m.listIdx+1, 0, style.Max(m.rowCount()-1, 0))
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

		case "shift+right":
			return m, cmdNext()
		case "shift+left":
			return m, cmdPrevious()

		case "enter":
			if m.showHelp {
				m.showHelp = false
				return m, nil
			}
			// 명령은 모델을 거치지 않는다. 목록에서 고른 것이 있으면 그것을 쓴다.
			if m.commanding() {
				line := m.input.Value()
				if rows := m.rows(); len(rows) > 0 && m.listIdx < len(rows) && rows[m.listIdx].cmd != nil {
					if picked := rows[m.listIdx].cmd; !strings.HasPrefix(line, picked.name+" ") {
						line = picked.name
					}
				}
				mm, cmd := m.runCommand(line)
				mm.clampList()
				return mm, cmd
			}
			// 검색 중에는 고른 곡을 튼다. 프롬프트일 때만 요청으로 보낸다.
			if m.mode == modeSearch {
				mm, cmd := m.playSelected()
				return mm, cmd
			}
			if prompt := strings.TrimSpace(m.input.Value()); prompt != "" {
				if m.thinking {
					return m, nil // 이미 도는 중이면 겹쳐 보내지 않는다
				}
				m.input.Reset()
				m.thinking = true
				m.intentErr = nil
				m.notice = ""
				m.showHelp = false
				m.clampList()
				return m, tea.Batch(
					cmdBuildQueue(prompt, data.Lib().Tracks, intent.Current{
						Title:   m.queueTitle,
						Items:   m.queue,
						Playing: m.nowPlayingID,
					}),
					m.spinner.Tick,
				)
			}
			mm, cmd := m.playSelected()
			return mm, cmd
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.clampList()
	return m, cmd
}

// applyQueue 는 의도 층의 결과를 화면에 앉힌다.
// 큐 섹션으로 옮기고 첫 곡을 튼다 — 요청했으면 소리가 나야 한다.
func (m Model) applyQueue(res intent.Result) (Model, tea.Cmd) {
	l := data.Lib()
	m.queueTitle, m.note = res.Title, res.Note
	m.usage.PromptTokens += res.Usage.PromptTokens
	m.usage.CompletionTokens += res.Usage.CompletionTokens
	m.usage.CostUsd += res.Usage.CostUsd

	items := make([]api.QueueItem, 0, len(res.Picks))
	for i, p := range res.Picks {
		t, ok := l.Track(p.TrackID)
		if !ok {
			continue // 스키마가 막지만, 없는 id 는 조용히 버린다
		}
		reason := p.Reason
		items = append(items, api.QueueItem{
			Id: int64(i + 1), SessionId: 1, Position: i + 1,
			Track: t, Reason: &reason,
			State: api.Pending, Origin: api.QueueItemOriginGeneration,
		})
	}
	if len(items) == 0 {
		m.intentErr = errNoTracks
		return m, nil
	}

	m.queue = items
	m.listIdx, m.listTop = 0, 0
	for i, s := range m.sections {
		if s.kind == secQueue {
			m.sectionIdx = i
		}
	}

	// 재편성으로 지금 곡이 살아남았으면 다시 틀지 않는다.
	// 듣던 곡이 처음으로 되감기는 것만큼 짜증나는 것이 없다.
	for _, it := range items {
		if it.Track.Id == m.nowPlayingID {
			return m, nil
		}
	}

	first := items[0].Track
	m.nowPlayingID = first.Id
	if first.PersistentId != nil {
		return m, cmdPlayTrack(*first.PersistentId)
	}
	return m, nil
}

var (
	errNoTracks = errors.New("고른 곡이 라이브러리에 없습니다")
	errNoQueue  = errors.New("저장할 큐가 없습니다")
)

// 목록에서 고른 곡을 튼다. 실제 재생은 Music.app 이 하고, 화면은 폴링으로 따라간다.
func (m Model) playSelected() (Model, tea.Cmd) {
	rows := m.rows()
	if m.listIdx < 0 || m.listIdx >= len(rows) || rows[m.listIdx].track == nil {
		return m, nil
	}
	t := *rows[m.listIdx].track
	m.nowPlayingID = t.Id
	m.positionMs = 0
	m.playing = true
	m.ensureQueued(t)

	if t.PersistentId != nil {
		return m, cmdPlayTrack(*t.PersistentId)
	}
	return m, nil
}

// 큐에 없는 곡이면 근거 없이 넣는다 — 사람이 직접 고른 것이므로 근거가 없다.
func (m *Model) ensureQueued(t api.Track) {
	for _, it := range m.queue {
		if it.Track.Id == t.Id {
			return
		}
	}
	m.queue = append([]api.QueueItem{{
		Id: t.Id, SessionId: 1, Position: 0, Track: t,
		State: api.Playing, Origin: api.QueueItemOriginManual,
	}}, m.queue...)
}

// clampList — 선택 행이 늘 보이도록 스크롤 위치를 맞춘다.
func (m *Model) clampList() {
	h := m.listHeight()
	n := m.rowCount()
	m.listIdx = style.Clamp(m.listIdx, 0, style.Max(n-1, 0))
	if m.listIdx < m.listTop {
		m.listTop = m.listIdx
	}
	if m.listIdx >= m.listTop+h {
		m.listTop = m.listIdx - h + 1
	}
	m.listTop = style.Clamp(m.listTop, 0, style.Max(n-h, 0))
}

// 한 번에 보여줄 목록 줄 수의 상한.
// 화면을 꽉 채우면 읽을 게 아니라 스캔할 것이 되어버린다.
const maxListRows = 14

// 세로 예산: 재생바1 + 룰1 + 목록 + 룰1 + 근거(0|1) + 입력1 + 여백2
func (m Model) listHeight() int {
	reserved := 7
	if _, ok := m.viewGateHint(10); ok {
		reserved++
	} else if _, ok := m.viewReason(10); ok {
		reserved++
	}
	h := style.Max(m.h-reserved, 3)
	return style.Min(h, maxListRows)
}

func (m Model) View() tea.View {
	if m.w == 0 || m.h == 0 {
		return tea.NewView("")
	}
	w := style.ContentWidth(m.w)
	listW := w - sidebarWidth - 1
	listH := m.listHeight()

	var b strings.Builder
	// 지금 재생 중인 곡이 화면의 첫 줄이다.
	// 섹션 제목과 곡 수는 뺐다 — 사이드바의 레일이 이미 어디인지 말해준다.
	b.WriteString(m.viewPlayer(w))
	b.WriteString("\n")
	b.WriteString(style.RuleBrand(w))
	b.WriteString("\n")

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		m.viewSidebar(listH),
		style.RuleStyle.Render(strings.Repeat("│\n", listH)),
		m.viewList(listW, listH))
	b.WriteString(body)
	b.WriteString("\n")
	b.WriteString(style.Rule(w))
	b.WriteString("\n")

	// 관문 안내가 있으면 근거 자리를 그것이 쓴다. 둘 다 뜨는 일은 없다.
	if hint, ok := m.viewGateHint(w); ok {
		b.WriteString(hint)
		b.WriteString("\n")
	} else if reason, ok := m.viewReason(w); ok {
		b.WriteString(reason)
		b.WriteString("\n")
	}
	b.WriteString(m.input.View())
	b.WriteString("\n")
	b.WriteString(style.Rule(w))
	b.WriteString("\n")
	b.WriteString(m.viewStatus(w))

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
		st.Text = lipgloss.NewStyle().Foreground(style.ColFg)
		st.Prompt = lipgloss.NewStyle().Foreground(style.ColBrand)
		st.Placeholder = lipgloss.NewStyle().Foreground(style.ColFaint)
	}
	styles.Cursor.Color = style.ColBrand
	ta.SetStyles(styles)
}
