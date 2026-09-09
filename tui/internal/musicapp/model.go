// Package musicapp 은 호스트가 담는 앱 하나다.
//
// 본문만 그린다. 입력창과 상태줄은 호스트의 것이다 — docs/07-호스트-계약.md.
package musicapp

import (
	"errors"
	"strings"

	"amcli/tui/internal/api"
	"amcli/tui/internal/app"
	"amcli/tui/internal/data"
	"amcli/tui/internal/intent"
	"amcli/tui/internal/music"
	"amcli/tui/internal/style"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type Model struct {
	sections   []section
	sectionIdx int

	listIdx int
	listTop int

	// 호스트가 검색어를 넣어준다. 비어 있으면 필터가 없다.
	filter string

	// 재생 상태는 Music.app 폴링으로 채운다. DB 에 없는 값이다.
	queue        []api.QueueItem
	nowPlayingID int64
	positionMs   int
	playing      bool

	// Music.app 이 말해주는 것 그대로. 라이브러리에 없는 곡도 여기 담긴다.
	live music.PlayerState

	// 의도 층 — 자연어 한 줄이 큐가 되는 경로.
	thinking   bool
	spinner    spinner.Model
	queueTitle string
	note       string
	intentErr  error
	notice     string
	usage      api.Usage

	// 관문 — 로그인이 아니라 권한과 앱 실행 여부다.
	playerErr error
	polled    bool

	// 스크롤 계산에만 쓴다. 그리는 것은 언제나 View 의 인자를 따른다.
	bodyH int
}

// 컴파일 타임에 계약을 지키는지 확인한다.
var _ app.App = Model{}

func New() Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(style.ColBrand)

	return Model{
		sections: buildSections(data.Lib()),
		spinner:  sp,
		playing:  true,
	}
}

func (m Model) Name() string { return "music" }

func (m Model) Description() string {
	return "Plays music from the user's own Apple Music library. " +
		"Handles requests about what to listen to, building or adjusting a queue, " +
		"finding tracks, and playback control."
}

// Init — send 는 이벤트 루프 밖에서 메시지를 넣는 통로다.
// 음악은 폴링이라 아직 쓰지 않지만, 계약이 그렇게 되어 있다.
func (m Model) Init(send func(tea.Msg)) tea.Cmd {
	return tea.Batch(m.spinner.Tick, fetchStatus, tick())
}

func (m Model) Ready() error { return m.playerErr }

func (m Model) Badge() int { return 0 }

// Ask — 자연어 한 줄을 큐로 바꾼다. 라우터가 이 앱을 지목했을 때 불린다.
func (m Model) Ask(prompt string) tea.Cmd {
	return cmdBuildQueue(prompt, data.Lib().Tracks, intent.Current{
		Title:   m.queueTitle,
		Items:   m.queue,
		Playing: m.nowPlayingID,
	})
}

// Thinking 은 호스트가 스피너를 대신 그릴지 판단할 때 쓴다.
func (m Model) Thinking() bool { return m.thinking }

// Filter 는 검색어를 받는다. 즉시 반영되어야 하므로 Cmd 를 돌려주지 않는다.
func (m Model) Filter(q string) app.App {
	if m.filter != q {
		m.listIdx, m.listTop = 0, 0
	}
	m.filter = q
	return m
}

func (m Model) searching() bool { return strings.TrimSpace(m.filter) != "" }

func (m Model) Update(msg tea.Msg) (app.App, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case savedMsg:
		if msg.err != nil {
			return m, app.SayErr(m.Name(), msg.err)
		}
		return m, app.Say(m.Name(), "Saved \""+msg.name+"\" to Apple Music")

	case queueMsg:
		m.thinking = false
		if msg.err != nil {
			return m, app.SayErr(m.Name(), msg.err)
		}
		mm, cmd := m.applyQueue(msg.res)
		// 큐 전체에 대한 한 문장은 대화이므로 로그로 간다.
		// 곡마다 붙는 근거는 곡의 속성이므로 목록에 남는다.
		note := msg.res.Note
		if strings.TrimSpace(note) == "" {
			note = msg.res.Title
		}
		return mm, tea.Batch(cmd, app.Say(m.Name(), note))

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

	case app.AskMsg:
		// 호스트가 넘긴 자연어 요청. 기다린다는 표시는 호스트가 한다.
		m.thinking = true
		return m, m.Ask(msg.Prompt)

	case clearQueueMsg:
		m.queue = nil
		m.queueTitle, m.note, m.notice = "", "", ""
		m.jumpTo(secRecent)
		return m, nil

	case jumpMsg:
		m.jumpTo(msg.kind)
		return m, nil

	case errMsg:
		return m, app.SayErr(m.Name(), msg.err)

	case app.ResizeMsg:
		m.bodyH = msg.Height
		m.clampList()
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+g":
			// 권한이 막혀 있을 때 시스템 설정을 연다.
			if m.playerErr == music.ErrPermissionDenied {
				return m, cmdOpenSettings()
			}
		case "up", "ctrl+p":
			m.listIdx = style.Clamp(m.listIdx-1, 0, style.Max(m.rowCount()-1, 0))
			m.clampList()
		case "down", "ctrl+n":
			m.listIdx = style.Clamp(m.listIdx+1, 0, style.Max(m.rowCount()-1, 0))
			m.clampList()
		case "tab":
			m.sectionIdx = (m.sectionIdx + 1) % len(m.sections)
			m.listIdx, m.listTop = 0, 0
		case "shift+tab":
			m.sectionIdx = (m.sectionIdx - 1 + len(m.sections)) % len(m.sections)
			m.listIdx, m.listTop = 0, 0
		case "shift+right":
			return m, cmdNext()
		case "shift+left":
			return m, cmdPrevious()
		case "enter":
			return m.playSelected()
		}
	}
	return m, nil
}

// applyQueue 는 의도 층의 결과를 화면에 앉힌다.
// 큐 섹션으로 옮기고 첫 곡을 튼다 — 요청했으면 소리가 나야 한다.
func (m Model) applyQueue(res intent.Result) (app.App, tea.Cmd) {
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
		return m, app.SayErr(m.Name(), errNoTracks)
	}

	m.queue = items
	m.listIdx, m.listTop = 0, 0
	m.jumpTo(secQueue)

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
	errNoName   = errors.New("플레이리스트 이름이 필요합니다: /save <name>")
)

// 목록에서 고른 곡을 튼다. 실제 재생은 Music.app 이 하고, 화면은 폴링으로 따라간다.
func (m Model) playSelected() (app.App, tea.Cmd) {
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
	h := m.listHeight(m.bodyH)
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

// 본문 세로 예산: 재생바1 + 룰1 + 목록 + 룰1 + 근거(0|1)
func (m Model) listHeight(h int) int {
	reserved := 3
	if _, ok := m.viewGateHint(10); ok {
		reserved++
	}
	return style.Min(style.Max(h-reserved, 3), maxListRows)
}

// View 는 본문을 그린다. 크기는 호스트가 알려주므로 기억하지 않는다.
func (m Model) View(w, h int) string {
	listH := m.listHeight(h)

	var b strings.Builder
	// 지금 재생 중인 곡이 본문의 첫 줄이다.
	b.WriteString(m.viewPlayer(w))
	b.WriteString("\n")
	b.WriteString(style.RuleBrand(w))
	b.WriteString("\n")

	b.WriteString(m.viewList(w, listH))
	b.WriteString("\n")
	b.WriteString(style.Rule(w))

	if hint, ok := m.viewGateHint(w); ok {
		b.WriteString("\n")
		b.WriteString(hint)
	}
	return b.String()
}
