// Package musicapp 은 호스트가 담는 앱 하나다.
//
// 본문만 그린다. 입력창과 상태줄은 호스트의 것이다 — docs/07-호스트-계약.md.
package musicapp

import (
	"context"
	"errors"
	"image"
	"sort"
	"strconv"
	"strings"
	"time"

	"amcli/tui/internal/api"
	"amcli/tui/internal/app"
	"amcli/tui/internal/applemusic"
	"amcli/tui/internal/data"
	"amcli/tui/internal/intent"
	"amcli/tui/internal/lyrics"
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

	// Music.app 안에 실제로 만들어 둔 큐 플레이리스트의 persistent ID.
	//
	// 비어 있으면 "화면의 큐와 Music.app 의 큐가 다르다"는 뜻이다. 큐를
	// 화면에서만 건드렸을 때(ensureQueued·/clear) 비운다 — 그 상태로
	// 번호를 믿고 재생하면 엉뚱한 곡이 나온다.
	queuePID string

	// 파고든 묶음. nil 이면 섹션이 정한 목록을 그대로 본다.
	//
	// Artists·Albums 는 곡이 아니라 묶음을 보여준다. 그 안으로 들어갈 길이
	// 없으면 커서는 서는데 enter 가 아무 일도 안 하는 줄이 된다 —
	// "고를 수 없는 줄에는 서지 않는다"(docs/07)의 예외가 생겨 버린다.
	drill *data.Group

	// 파고들기 전의 커서. esc 로 그 자리에 돌아온다.
	drillIdx, drillTop int

	// 호스트가 검색어를 넣어준다. 비어 있으면 필터가 없다.
	filter string

	// 재생 상태는 Music.app 폴링으로 채운다. DB 에 없는 값이다.
	queue        []api.QueueItem
	nowPlayingID int64
	positionMs   int
	playing      bool

	// Music.app 이 말해주는 것 그대로. 라이브러리에 없는 곡도 여기 담긴다.
	live music.PlayerState

	// 우리가 곡을 끊었다는 표시. 다음 폴링이 집어 간다 — plays.go
	endHint   data.EndedBy
	endHintAt time.Time

	// 의도 층 — 자연어 한 줄이 큐가 되는 경로.
	//
	// 도는 요청은 turn 한 덩어리로 들고 있는다. 번호·손잡이·기다림이
	// 언제나 함께 움직여야 하기 때문이다 — turn.go
	ask turn

	// 이 턴 동안의 대화와, 모델을 몇 번 불렀는지. 턴을 넘어 살아남지 않는다.
	chat  intent.Chat
	steps int

	// 지금 물음의 문장. 턴이 닫힐 때 우리가 한 말과 짝지어 기억에 적는다.
	asked string

	// 턴을 넘어 살아남는 것은 이것뿐이다 — 주고받은 말.
	//
	// "아까 그거 말고"의 "그거"가 여기서 풀린다. 큐는 매 턴 새로 그려
	// 넘기므로 상태를 여기 담을 이유가 없다 — intent 의 NewChat.
	memory []intent.Exchange

	spinner    spinner.Model
	queueTitle string
	note       string
	intentErr  error
	notice     string
	usage      api.Usage

	// 관문 — 로그인이 아니라 권한과 앱 실행 여부다.
	playerErr error
	polled    bool

	// 라이브러리 — Music.app 을 읽어 채운다. 읽기 전에는 비어 있다.
	syncing bool
	synced  bool

	// 카탈로그 — 라이브러리 밖. 설정이 없으면 cat 이 nil 이고,
	// 그때는 이 기능만 없다. 앱은 그대로 돈다.
	cat      *applemusic.Client
	catTerm  string
	catHits  []api.CatalogTrack
	catSeq   int
	catBusy  bool
	catLogin bool
	catErr   error // 카탈로그가 왜 안 되는가. 다음 행동을 말하는 데 쓴다

	// 인식 — 방 안에서 들린 곡. 한 번도 안 알아맞혔으면 섹션 자체가 없다.
	// shazam.go
	shzHits []api.CatalogTrack
	shzBusy bool
	shzOK   bool // 헬퍼가 옆에 있는가. 없으면 /shazam 을 내지 않는다

	// 앨범 커버. 곡이 바뀔 때만 다시 읽는다 — artwork.go
	art    image.Image
	artPID string

	// 가사. 커버와 같은 규칙으로 곡이 바뀔 때만 받는다 — lyrics.go
	lyrics    *lyrics.Lyrics
	lyricsPID string

	// 마지막 폴링 시각. 폴링 사이를 메워 가사가 제때 넘어가게 한다.
	polledAt time.Time

	// 검색 결과에서 내 라이브러리 구역을 펼쳤나. 검색어가 바뀌면 다시 접힌다.
	searchAll bool

	// 검색어가 마지막으로 바뀐 때. 타이핑이 멎었는지 재는 데만 쓴다.
	filterAt time.Time

	// 담고 트는 중인 곡. 한 번에 하나만 한다.
	adding *addJob

	// 스크롤 계산에만 쓴다. 그리는 것은 언제나 View 의 인자를 따른다.
	bodyH int
	bodyW int
}

// 컴파일 타임에 계약을 지키는지 확인한다.
var _ app.App = Model{}

func New() Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(style.ColBrand)

	return Model{
		// 여기서는 아직 아무것도 읽지 않는다. Init 이 캐시와 실물을 부른다.
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

// 못 하는 것을 말한다. 카탈로그는 뒤지지 않는다.
func (m Model) Tagline() string { return "only what you own" }

// Init — send 는 이벤트 루프 밖에서 메시지를 넣는 통로다.
// 음악은 폴링이라 아직 쓰지 않지만, 계약이 그렇게 되어 있다.
func (m Model) Init(send func(tea.Msg)) tea.Cmd {
	// 캐시는 몇 ms, 실물은 몇 초다. 둘 다 띄우고 먼저 오는 것을 그린다.
	return tea.Batch(m.spinner.Tick, fetchStatus, tick(), cmdLoadCache, cmdDumpLibrary(false),
		cmdCatalogInit, cmdShazamInit)
}

func (m Model) Ready() error { return m.playerErr }

func (m Model) Badge() int { return 0 }

// Ask — 자연어 한 줄을 큐로 바꾼다. 라우터가 이 앱을 지목했을 때 불린다.
//
// 호스트는 AskMsg 로 보내므로 실제 경로는 Update 이고, 거기서 취소 손잡이를
// 붙든다. 계약이 요구하는 이 진입점은 손잡이를 둘 곳이 없어 그냥 버린다 —
// 이 길로 들어온 요청은 esc 로 못 끊는다.
func (m Model) Ask(prompt string) tea.Cmd {
	_, cmd := m.startAsk(prompt)
	return cmd
}

// startAsk 는 물음 하나를 연다.
//
// 곧장 선곡으로 가지 않는다. 무엇을 시키는 말인지 먼저 모델에게 고르게 하고,
// 필요한 도구가 있으면 그때 부른다(agent.go). 라이브러리를 읽는 값은
// build_queue 안에서만 치른다.
func (m Model) startAsk(prompt string) (Model, tea.Cmd) {
	var ctx context.Context
	m.ask, ctx = m.ask.start(askTimeout)
	m.chat = intent.NewChat(prompt, m.current(), m.memory)
	m.asked = prompt
	m.steps = 0
	return m, cmdStep(ctx, m.ask.seq, m.chat, m.toolDefs())
}

// current 는 모델에게 보여줄 "지금 화면의 큐"다.
func (m Model) current() intent.Current {
	return intent.Current{
		Title:   m.queueTitle,
		Items:   m.queue,
		Playing: m.nowPlayingID,
	}
}

// stopAsk 는 도는 요청을 버린다. 번호를 올려 늦게 오는 답도 못 앉게 한다.
func (m Model) stopAsk() Model {
	m.ask = m.ask.stop()
	return m
}

// Thinking 은 호스트가 스피너를 대신 그릴지 판단할 때 쓴다.
func (m Model) Thinking() bool { return m.ask.live }

// Filter 는 검색어를 받는다. 즉시 반영되어야 하므로 Cmd 를 돌려주지 않는다.
// Filter 는 계약상 Cmd 를 못 돌려준다 — 글자마다 불리기 때문이다.
// 그래서 여기서는 "언제 바뀌었는지"만 적어두고, 바깥을 찾는 일은
// 이미 도는 틱이 타이핑이 멎은 것을 보고 시작한다 (maybeSearchCatalog).
func (m Model) Filter(q string) app.App {
	if m.filter != q {
		m.listIdx, m.listTop = 0, 0
		m.filterAt = time.Now()
		m.searchAll = false // 새 검색어는 접힌 채로 시작한다
	}
	m.filter = q
	return m
}

// 타이핑이 이만큼 멎으면 바깥까지 찾는다.
// 폴링 주기가 1초라 실제 지연은 이 값과 1초 사이다.
const catalogDebounce = 400 * time.Millisecond

// maybeSearchCatalog — 틱마다 불린다. 조건이 맞을 때만 요청이 나간다.
func (m *Model) maybeSearchCatalog() tea.Cmd {
	q := strings.TrimSpace(m.filter)
	if m.cat == nil || q == "" || m.catBusy {
		return nil
	}
	if m.catTerm == q {
		return nil // 이미 이 검색어의 결과를 갖고 있다
	}
	if time.Since(m.filterAt) < catalogDebounce {
		return nil // 아직 치는 중이다
	}
	m.catSeq++
	m.catBusy = true
	return cmdCatalogSearch(m.cat, q, m.catSeq)
}

func (m Model) searching() bool { return strings.TrimSpace(m.filter) != "" }

func (m Model) Update(msg tea.Msg) (app.App, tea.Cmd) {
	if mm, cmd, handled := m.applyCatalog(msg); handled {
		return mm, cmd
	}
	if mm, cmd, handled := m.applyShazam(msg); handled {
		return mm, cmd
	}

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

	case stepMsg:
		// 그만뒀거나 더 새 물음이 떠 있으면 늦게 온 답이다. 버린다.
		if !m.ask.fresh(msg.seq) {
			return m, nil
		}
		return m.applyStep(msg)

	case queueMsg:
		// 그만뒀거나 더 새 물음이 떠 있으면 늦게 온 답이다. 버린다.
		if !m.ask.fresh(msg.seq) {
			return m, nil
		}
		m.ask = m.ask.done()
		if msg.err != nil {
			// 그만둔 것은 실패가 아니다. 호스트가 이미 로그에 남겼다.
			if errors.Is(msg.err, context.Canceled) {
				return m, nil
			}
			return m, app.SayErr(m.Name(), msg.err)
		}
		mm, cmd := m.applyQueue(msg.res)
		// 큐 전체에 대한 한 문장은 로그에, 곡마다의 근거는 펼쳤을 때 보이도록
		// 상세에 담는다. 평소에는 한 줄이고 ctrl+o 로 열어 본다.
		note := msg.res.Note
		if strings.TrimSpace(note) == "" {
			note = msg.res.Title
		}
		return mm.(Model).remember(note), tea.Batch(cmd, app.SayWith(m.Name(), note, queueDetail(msg.res)))

	case libraryMsg:
		return m.applyLibrary(msg)

	case tickMsg:
		// 한 줄로 쓰지 않는다. `return m, tea.Batch(…, m.maybeSearchCatalog())`
		// 는 m 을 복사하는 시점과 메서드가 m 을 고치는 시점의 순서를 Go 명세가
		// 보장하지 않는다. 어긋나면 검색 번호가 모델에 안 남아 답이 조용히
		// 버려진다 — 증상은 "카탈로그 검색이 안 된다"로만 보인다.
		search := m.maybeSearchCatalog()
		return m, tea.Batch(fetchStatus, tick(), search)

	case statusMsg:
		m.polled = true
		m.polledAt = time.Now()
		m.playerErr = msg.err
		if msg.err == nil {
			// m.live 를 갈아끼우기 **전에** 부른다. 직전 곡이 얼마나
			// 흘렀는지는 옛 상태에만 있다(plays.go).
			var noted tea.Cmd
			m, noted = m.notePlayback(msg.state)
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
			// 곡이 바뀌었으면 커버와 가사를 새로 받는다. 폴링마다 하면
			// Music.app 이 1초에 한 번씩 1200×1200 을 퍼내고, 남의 무료
			// 서버를 1초에 한 번씩 두드리게 된다.
			if id := msg.state.PersistentID; id != m.artPID {
				m.artPID, m.art = id, nil
				m.lyricsPID, m.lyrics = id, nil
				if id != "" {
					title, artist, album, dur := m.nowPlayingMeta()
					return m, tea.Batch(
						noted,
						cmdArtwork(id),
						cmdLyrics(id, artist, title, album, dur),
					)
				}
			}
			return m, noted
		}
		return m, nil

	case lyricsMsg:
		if msg.pid != m.lyricsPID {
			return m, nil // 받는 사이에 곡이 바뀌었다
		}
		// 없는 것도 답이다. 그때는 곡 이력이 그 자리를 쓴다(lyrics.go).
		m.lyrics = msg.l
		return m, nil

	case artMsg:
		// 답이 오는 사이에 곡이 바뀌었으면 버린다.
		if msg.pid != m.artPID {
			return m, nil
		}
		// 커버가 없는 곡도 있다. 실패가 아니라 상태이므로 조용히 접는다 —
		// 화면은 한 줄짜리 재생 바로 돌아간다.
		m.art = msg.img
		return m, nil

	case app.AskMsg:
		// 호스트가 넘긴 자연어 요청. 기다린다는 표시는 호스트가 한다.
		return m.startAsk(msg.Prompt)

	case queueWrittenMsg:
		if msg.err != nil {
			return m, app.SayErr(m.Name(), msg.err)
		}
		m.queuePID = msg.pid
		return m, nil

	case app.CancelMsg:
		// esc — 방금 시킨 일에서 물러난다. 화면은 그대로 두고 요청만 끊는다.
		return m.stopAsk(), nil

	case removeSelectedMsg:
		rows := m.rows()
		if m.listIdx < 0 || m.listIdx >= len(rows) || rows[m.listIdx].track == nil {
			return m, send(errMsg{errNotInQueue})
		}
		return m.removeFromQueue(rows[m.listIdx].track.Id)

	case clearQueueMsg:
		m.queue = nil
		m.queuePID = ""
		m.queueTitle, m.note, m.notice = "", "", ""
		m.jumpTo(secRecent, "")
		return m, nil

	case jumpMsg:
		m.jumpTo(msg.kind, msg.label)
		return m, nil

	case errMsg:
		return m, app.SayErr(m.Name(), msg.err)

	case app.ResizeMsg:
		m.bodyH, m.bodyW = msg.Height, msg.Width
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
			m.move(-1)
		case "down", "ctrl+n":
			m.move(1)
		case "tab":
			// 다음 칸으로만 간다. 되돌아가는 shift+tab 은 호스트가
			// 모드 전환에 쓴다. 먼 섹션에는 `/` 로 곧장 간다(command.go).
			m.sectionIdx = (m.sectionIdx + 1) % len(m.sections)
			m.drill = nil
			m.listIdx, m.listTop = 0, 0
		case "shift+down":
			// 재생 제어는 shift+화살표 한 가족이다. 수식키+화살표라
			// 입력창도 한글 조합도 건드리지 않는다 — 알파벳이나 space 를
			// 쓸 수 없는 이유가 그것이다(docs/07 2절).
			return m, cmdPlayPause()
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
	m.jumpTo(secQueue, "")

	// 큐를 Music.app 안에 실제로 만든다.
	//
	// 예전에는 첫 곡만 틀었다. 나머지는 Music.app 이 몰라서, 첫 곡이 끝나면
	// 그 곡이 속한 앨범의 다음 트랙이 흘렀다 — 화면의 큐는 영수증이었다.
	ids := make([]string, 0, len(items))
	start, pos := 0, 0
	for _, it := range items {
		if it.Track.PersistentId == nil {
			continue // 담을 수 없는 곡. 화면에는 남고 재생만 건너뛴다
		}
		ids = append(ids, *it.Track.PersistentId)
		// 재편성으로 듣던 곡이 살아남았으면 그 자리에서 잇는다.
		// 듣던 곡이 처음으로 되감기는 것만큼 짜증나는 것이 없다.
		if it.Track.Id == m.nowPlayingID {
			start, pos = len(ids), m.positionMs/1000
		}
	}
	if len(ids) == 0 {
		return m, app.SayErr(m.Name(), errNoTracks)
	}
	if start == 0 {
		start = 1
		m.nowPlayingID = items[0].Track.Id
	}
	// 플레이리스트를 새로 쓸 때까지는 화면과 Music.app 이 어긋난 상태다.
	m.queuePID = ""
	// **우리가 끊는 것이다.** 표시해 두지 않으면 다음 폴링이 이것을
	// "사용자가 넘겼다"로 읽고, 우리가 만든 행동이 취향으로 쌓인다.
	return m.hintEnd(data.EndedRequeue), cmdWriteQueue(ids, start, pos)
}

var (
	errNoTracks   = errors.New("None of those tracks are in your library")
	errNoQueue    = errors.New("Nothing in the queue to save")
	errSyncing    = errors.New("Already reading your library")
	errNoName     = errors.New("Needs a playlist name: /save <name>")
	errNotInQueue = errors.New("That track is not in the queue")
)

// 목록에서 고른 곡을 튼다. 실제 재생은 Music.app 이 하고, 화면은 폴링으로 따라간다.
func (m Model) playSelected() (app.App, tea.Cmd) {
	rows := m.rows()
	if m.listIdx < 0 || m.listIdx >= len(rows) {
		return m, nil
	}
	// 접힌 구역을 편다. 커서는 펼친 자리에 그대로 둔다.
	if rows[m.listIdx].isMore() {
		m.searchAll = true
		m.clampList()
		return m, nil
	}
	// 카탈로그 곡은 아직 내 것이 아니다. 담아야 틀 수 있다.
	if ct := rows[m.listIdx].catalog; ct != nil {
		return m.playCatalog(*ct)
	}
	// 묶음은 트는 것이 아니라 여는 것이다. 안에 곡이 들어 있다.
	if g := rows[m.listIdx].group; g != nil {
		return m.enterGroup(*g)
	}
	if rows[m.listIdx].track == nil {
		return m, nil
	}
	t := *rows[m.listIdx].track
	// 듣던 곡은 사용자가 다른 것을 골라서 끝난다. 넘긴 것과 뜻이 다르다.
	m = m.hintEnd(data.EndedPicked)

	// 큐 안의 곡이면 플레이리스트의 그 자리에서 튼다. 곡 하나만 틀면
	// 끝나는 순간 Music.app 이 큐 밖으로 나가 버린다.
	if m.queuePID != "" {
		for i, it := range m.queue {
			if it.Track.Id == t.Id {
				m.nowPlayingID, m.positionMs, m.playing = t.Id, 0, true
				return m, cmdPlayQueueAt(m.queuePID, i+1)
			}
		}
	}

	m.nowPlayingID = t.Id
	m.positionMs = 0
	m.playing = true
	m.ensureQueued(t)

	if t.PersistentId != nil {
		return m, cmdPlayTrack(*t.PersistentId)
	}
	return m, nil
}

// removeFromQueue 는 큐에서 곡 하나를 뺀다.
//
// **사람과 AI 가 같이 쓰는 진입점이다.** 지금은 /remove 가 부르고, 나중에
// 의도 층이 "이 곡 빼줘"를 여기로 바로 보낸다 — 그러면 큐를 통째로 다시
// 고르지 않아도 되므로 7초가 0초가 된다.
//
// 통째로 다시 쓰지 않는다. 다시 쓰면 듣던 곡까지 사라져 음악이 끊긴다.
// 한 줄만 지우면 앞쪽을 빼든 뒤쪽을 빼든 나머지는 그대로 흐른다.
func (m Model) removeFromQueue(id int64) (Model, tea.Cmd) {
	at := -1
	for i, it := range m.queue {
		if it.Track.Id == id {
			at = i
			break
		}
	}
	if at < 0 {
		return m, send(errMsg{errNotInQueue})
	}
	title := m.queue[at].Track.Title
	m, cmd := m.dropAt(at)
	return m, tea.Batch(cmd, app.Say(m.Name(), "Removed "+title))
}

// dropAt 은 큐에서 한 줄을 빼고 Music.app 에도 반영한다. 말은 하지 않는다 —
// 여러 곡을 뺄 때 줄마다 말하면 로그가 시끄럽다.
func (m Model) dropAt(at int) (Model, tea.Cmd) {
	// 지금 나오는 곡을 빼면 다음 곡으로 넘어간다. 조용해지는 것이 아니다 —
	// "이거 별로야"는 다음 걸 틀라는 뜻이다.
	playing := m.queue[at].Track.Id == m.nowPlayingID
	if playing {
		// 곡을 지목해서 뺀 것이다. 넘긴 것보다 분명한 거절이다.
		m = m.hintEnd(data.EndedRemoved)
	}

	m.queue = append(append([]api.QueueItem{}, m.queue[:at]...), m.queue[at+1:]...)
	m.clampList()

	// 화면과 Music.app 이 어긋나 있으면 번호를 믿을 수 없다. 화면만 고친다.
	if m.queuePID == "" {
		return m, nil
	}
	return m, cmdRemoveQueueTrack(m.queuePID, at+1, playing)
}

// removeTracks 는 곡 여럿을 큐에서 뺀다. AI 가 지목했을 때 쓴다.
//
// 뒤에서부터 뺀다. 앞에서 빼면 그 뒤 곡들의 자리번호가 밀려, 두 번째
// 곡을 지울 때 엉뚱한 줄을 가리키게 된다.
func (m Model) removeTracks(ids []int64, note string) (app.App, tea.Cmd) {
	at := make([]int, 0, len(ids))
	for _, id := range ids {
		for i, it := range m.queue {
			if it.Track.Id == id {
				at = append(at, i)
				break
			}
		}
	}
	if len(at) == 0 {
		return m, app.SayErr(m.Name(), errNotInQueue)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(at)))

	cmds := make([]tea.Cmd, 0, len(at)+1)
	for _, i := range at {
		next, cmd := m.dropAt(i)
		m = next
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if strings.TrimSpace(note) != "" {
		cmds = append(cmds, app.Say(m.Name(), note))
	}
	return m, tea.Batch(cmds...)
}

// enterGroup — 묶음 안으로 한 단계 들어간다.
//
// 곡 목록은 이미 그 행이 들고 있다(data.Group.Tracks). 다시 훑을 필요가 없다.
func (m Model) enterGroup(g data.Group) (app.App, tea.Cmd) {
	if len(g.Tracks) == 0 {
		return m, nil
	}
	m.drillIdx, m.drillTop = m.listIdx, m.listTop
	m.drill = &g
	m.listIdx, m.listTop = 0, 0
	m.clampList()
	return m, nil
}

// Back — esc 가 왔다. 파고든 것이 있으면 그 한 단계를 물러난다.
//
// 나온 자리에 커서를 되돌린다. 훑던 중이었으므로 목록 맨 위로 튕기면
// 어디를 보고 있었는지 잃는다.
func (m Model) Back() (app.App, bool) {
	if m.drill == nil {
		return m, false
	}
	m.drill = nil
	m.listIdx, m.listTop = m.drillIdx, m.drillTop
	m.clampList()
	return m, true
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
	// 화면에서만 끼워 넣었으므로 Music.app 의 플레이리스트와 번호가 어긋났다.
	m.queuePID = ""
}

// clampList — 선택 행이 늘 보이도록 스크롤 위치를 맞춘다.
// move — 커서를 옮긴다. 구역 머리글은 건너뛴다.
//
// 머리글에 커서가 서면 enter 가 아무것도 안 하는 자리가 생긴다.
// 고를 수 없는 줄에는 서지 않는 것이 낫다.
func (m *Model) move(d int) {
	rows := m.rows()
	i := m.listIdx
	for {
		i += d
		if i < 0 || i >= len(rows) {
			return // 끝에 닿았다. 커서를 그대로 둔다
		}
		if rows[i].selectable() {
			m.listIdx = i
			m.clampList()
			return
		}
	}
}

func (m *Model) clampList() {
	h := m.listWindow()
	n := m.rowCount()
	m.listIdx = style.Clamp(m.listIdx, 0, style.Max(n-1, 0))
	// 첫 줄이 머리글이면 그 다음 곡으로 내린다.
	if rows := m.rows(); m.listIdx < len(rows) && !rows[m.listIdx].selectable() {
		for i := m.listIdx + 1; i < len(rows); i++ {
			if rows[i].selectable() {
				m.listIdx = i
				break
			}
		}
	}
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
// 본문 세로 예산: 머리 + 룰1 + 목록 + 룰1 + 근거(0|1)
// 머리가 몇 줄을 먹었나. 한 줄짜리 재생 바면 1 이다.
func headHeight(head string) int { return strings.Count(head, "\n") + 1 }

// headView 는 본문 맨 위, 지금 듣고 있는 것을 그린다.
//
// 커버를 못 그리는 사정이면(좁거나·낮거나·커버가 없거나·관문) 한 줄짜리
// 재생 바로 물러난다. artwork.go
func (m Model) headView(w, h int) string {
	if head, big := m.viewNowPlaying(w, h); big {
		return head
	}
	return m.viewPlayer(w)
}

// listWindow 는 목록이 지금 실제로 쓰는 줄 수다.
//
// **그리는 쪽과 커서를 옮기는 쪽이 반드시 같은 수를 봐야 한다.** 달랐을 때
// 커서가 창 밖으로 나가도 목록이 따라오지 않았다 — 커버가 뜨면 창이 절반으로
// 줄어드는데, 스크롤 계산은 커버가 없던 시절의 큰 창을 그대로 썼기 때문이다.
//
// 그래서 View 도 clampList 도 이 한 곳을 통한다.
func (m Model) listWindow() int {
	return m.listHeight(m.bodyH - headHeight(m.headView(m.bodyW, m.bodyH)) + 1)
}

// listHeight — 목록은 남는 자리를 전부 먹는다.
//
// 한때 열네 줄 상한이 있었다. 명령 팔레트가 뜰 때 목록이 밀리지 않게 하려던
// 것이었는데, 입력창을 바닥에 붙이면서 팔레트가 남는 자리를 쓰게 되어
// 근거가 사라졌다. 상한만 남아서 **창을 키워도 목록이 안 늘고, 그 아래가
// 스무 줄씩 비었다.**
//
// 고정 줄 수를 두는 TUI 는 없다. lazygit·k9s·ncmpcpp 어디도 그러지 않는다.
// 칸은 남는 자리의 몫으로 정해지지, 숫자로 정해지지 않는다.
func (m Model) listHeight(h int) int {
	reserved := 3
	if _, ok := m.viewGateHint(10); ok {
		reserved++
	} else if _, ok := m.viewCatalogHint(10); ok {
		reserved++
	}
	return style.Max(h-reserved, 3)
}

// View 는 본문을 그린다. 크기는 호스트가 알려주므로 기억하지 않는다.
func (m Model) View(w, h int) string {
	head := m.headView(w, h)
	listH := m.listHeight(h - headHeight(head) + 1)

	var b strings.Builder
	// 지금 듣고 있는 것이 본문의 주인공이다. 목록은 그 아래다.
	b.WriteString(head)
	b.WriteString("\n")
	b.WriteString(style.RuleBrand(w))
	b.WriteString("\n")

	b.WriteString(m.viewList(w, listH))
	b.WriteString("\n")
	b.WriteString(style.Rule(w))

	// 관문이 있으면 그것이 이 자리를 쓴다. 둘 다 뜨는 일은 없다.
	hint, ok := m.viewGateHint(w)
	if !ok {
		hint, ok = m.viewCatalogHint(w)
	}
	if ok {
		b.WriteString("\n")
		b.WriteString(hint)
	}
	return b.String()
}

// applyLibrary — 새 스냅샷을 받아들인다.
//
// 캐시와 실물이 경주한다. 늦게 도착한 캐시가 실물을 덮으면 안 된다.
func (m Model) applyLibrary(msg libraryMsg) (app.App, tea.Cmd) {
	if msg.live {
		m.syncing = false
	}
	if msg.err != nil {
		// 관문(E1·E2)은 이미 1초마다 같은 말을 하고 있다. 두 번 말하지 않는다.
		if errors.Is(msg.err, music.ErrNotRunning) || errors.Is(msg.err, music.ErrPermissionDenied) {
			return m, nil
		}
		// 캐시가 없는 것은 첫 실행의 정상 상태다.
		if !msg.live {
			return m, nil
		}
		return m, app.SayErr(m.Name(), msg.err)
	}
	if msg.lib == nil || (!msg.live && m.synced) {
		return m, nil
	}

	m.synced = m.synced || msg.live
	data.Set(msg.lib)
	m.resync(msg.lib)

	if msg.announce {
		return m, app.Say(m.Name(), "Read "+strconv.Itoa(len(msg.lib.Songs()))+" songs from your library")
	}
	return m, nil
}
