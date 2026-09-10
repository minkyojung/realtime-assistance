package musicapp

import (
	"errors"

	"amcli/tui/internal/api"
	"amcli/tui/internal/app"
	"amcli/tui/internal/data"
	"amcli/tui/internal/intent"
	"amcli/tui/internal/music"
	tea "charm.land/bubbletea/v2"
)

// 큐를 만든 **뒤에** 손대는 길.
//
// 지금까지는 큐를 통째로 새로 쓰는 것과 한 곡 빼는 것뿐이었다. 그래서
// "이 곡 다음에 틀어줘" 나 "이거 뒤에 더 붙여줘" 가 전부 새 큐가 되었고,
// 붙여 달라고 했는데 듣던 곡이 끊기고 처음으로 튀었다.
//
// 애플뮤직이 하는 것과 같게 한다 — Play Next · Play Later · 순서 바꾸기.
// 셋 다 듣던 곡을 끊지 않는다.
//
// 끊기지 않는 조건은 하나다. **지금 나오는 곡을 건드리지 않는 것.**
// 고치는 자리가 그 곡보다 뒤이면 플레이리스트도 재생 중인 곡도 그대로
// 남아서 음악이 이어진다(music.RewriteQueueTail).

var (
	errQueueWaiting = errors.New("The queue is still being set up — try again in a moment")
	errQueueEdge    = errors.New("That track is already as far as it goes")
	errQueueNowPlay = errors.New("That track is playing now")
)

// queueAt 은 큐에서 그 곡의 자리다. 없으면 -1.
func (m Model) queueAt(id int64) int {
	for i, it := range m.queue {
		if it.Track.Id == id {
			return i
		}
	}
	return -1
}

// playingAt 은 지금 나오는 곡의 자리다. 큐에 없으면 -1.
//
// -1 이면 우리 큐가 아닌 것이 흐르고 있다는 뜻이다. 그때는 큐 전체가
// 아직 안 나온 것이므로 어디를 고쳐도 소리를 끊지 않는다.
func (m Model) playingAt() int {
	if m.nowPlayingID == 0 {
		return -1
	}
	return m.queueAt(m.nowPlayingID)
}

// putNext 는 커서의 곡을 지금 나오는 곡 바로 뒤로 보낸다 (Play Next).
func (m Model) putNext() (Model, tea.Cmd) {
	t, err := m.selectedTrack()
	if err != nil {
		return m, app.SayErr(m.Name(), err)
	}
	return m.place(t, m.playingAt()+1, "Playing next: ")
}

// putLater 는 커서의 곡을 큐 맨 뒤로 보낸다 (Play Later).
func (m Model) putLater() (Model, tea.Cmd) {
	t, err := m.selectedTrack()
	if err != nil {
		return m, app.SayErr(m.Name(), err)
	}
	at := len(m.queue)
	// 이미 큐에 있으면 그 자리가 빠지면서 뒤가 한 칸 당겨진다.
	if was := m.queueAt(t.Id); was >= 0 {
		at--
	}
	return m.place(t, at, "Added to the end: ")
}

// selectedTrack 은 커서가 놓인 곡이다. 곡이 아니면 오류.
func (m Model) selectedTrack() (api.Track, error) {
	rows := m.rows()
	if m.listIdx < 0 || m.listIdx >= len(rows) || rows[m.listIdx].track == nil {
		return api.Track{}, errNoTracks
	}
	t := *rows[m.listIdx].track
	if t.PersistentId == nil {
		return api.Track{}, errNoTracks
	}
	return t, nil
}

// place 는 곡을 큐의 at 자리로 옮기거나 새로 끼운다.
//
// 이미 큐에 있으면 옮기는 것이고, 없으면 끼우는 것이다. 애플뮤직에서
// 아무 곡이나 골라 Play Next 를 눌렀을 때와 같다.
func (m Model) place(t api.Track, at int, said string) (Model, tea.Cmd) {
	if len(m.queue) == 0 {
		// 넣을 큐가 없으면 그냥 튼다. 빈 큐에 "다음에 틀어줘" 는
		// "지금 틀어줘" 와 같은 말이다.
		return m.playOne(t)
	}
	next := make([]api.QueueItem, 0, len(m.queue)+1)
	for _, it := range m.queue {
		if it.Track.Id != t.Id {
			next = append(next, it)
		}
	}
	if at < 0 {
		at = 0
	}
	if at > len(next) {
		at = len(next)
	}
	item := api.QueueItem{Track: t, State: api.Pending, Origin: api.QueueItemOriginManual}
	next = append(next[:at], append([]api.QueueItem{item}, next[at:]...)...)
	return m.reseat(next, said+t.Title)
}

// shift 는 커서의 곡을 한 칸 위나 아래로 옮긴다.
func (m Model) shift(by int) (Model, tea.Cmd) {
	t, err := m.selectedTrack()
	if err != nil {
		return m, app.SayErr(m.Name(), err)
	}
	at := m.queueAt(t.Id)
	if at < 0 {
		return m, app.SayErr(m.Name(), errNotInQueue)
	}
	to := at + by
	if to < 0 || to >= len(m.queue) {
		return m, app.SayErr(m.Name(), errQueueEdge)
	}
	next := append([]api.QueueItem{}, m.queue...)
	next[at], next[to] = next[to], next[at]

	m, cmd := m.reseat(next, "")
	if cmd != nil {
		// 커서가 곡을 따라간다. 옮긴 것을 다시 옮기려면 그래야 한다.
		m.listIdx += by
		m.clampList()
	}
	return m, cmd
}

// reseat 는 새 순서를 화면과 Music.app 에 앉힌다.
//
// **처음으로 달라지는 자리부터** 다시 쓴다. 그 앞은 손대지 않으므로,
// 지금 나오는 곡이 거기 있으면 음악이 이어진다.
func (m Model) reseat(next []api.QueueItem, said string) (Model, tea.Cmd) {
	at := firstDiff(m.queue, next)
	if at < 0 {
		return m, nil // 달라진 것이 없다
	}
	// 지금 나오는 곡을 지웠다 다시 붙이면 소리가 끊긴다. 애플뮤직도
	// 재생 중인 곡은 옮기지 못한다.
	if p := m.playingAt(); p >= 0 && at <= p {
		return m, app.SayErr(m.Name(), errQueueNowPlay)
	}
	// 화면과 Music.app 이 어긋나 있으면 자리번호를 믿을 수 없다.
	if m.queuePID == "" {
		return m, app.SayErr(m.Name(), errQueueWaiting)
	}

	ids := make([]string, 0, len(next)-at)
	for _, it := range next[at:] {
		if it.Track.PersistentId == nil {
			return m, app.SayErr(m.Name(), errNoTracks)
		}
		ids = append(ids, *it.Track.PersistentId)
	}

	m.queue = next
	m.renumber()
	m.clampList()

	cmd := cmdRewriteQueueTail(m.queuePID, at+1, ids)
	if said != "" {
		cmd = tea.Batch(cmd, app.Say(m.Name(), said))
	}
	return m, cmd
}

// renumber 는 자리번호를 다시 매긴다. 화면이 그것으로 줄을 센다.
func (m *Model) renumber() {
	for i := range m.queue {
		m.queue[i].Id = int64(i + 1)
		m.queue[i].SessionId = 1
		m.queue[i].Position = i + 1
	}
}

// firstDiff 는 두 목록이 처음으로 달라지는 자리다. 같으면 -1.
func firstDiff(a, b []api.QueueItem) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i].Track.Id != b[i].Track.Id {
			return i
		}
	}
	if len(a) != len(b) {
		return min(len(a), len(b))
	}
	return -1
}

// cmdRewriteQueueTail 은 큐의 꼬리를 다시 쓴다. 앞은 그대로라 음악이 안 끊긴다.
func cmdRewriteQueueTail(pid string, from int, ids []string) tea.Cmd {
	return func() tea.Msg {
		if err := music.RewriteQueueTail(pid, from, ids); err != nil {
			return queueWrittenMsg{err: err}
		}
		return queueWrittenMsg{pid: pid}
	}
}

// reorderKind — 큐를 만든 뒤에 손대는 네 가지.
type reorderKind int

const (
	putNext reorderKind = iota
	putLater
	moveUp
	moveDown
)

func (m Model) reorder(k reorderKind) (Model, tea.Cmd) {
	switch k {
	case putNext:
		return m.putNext()
	case putLater:
		return m.putLater()
	case moveUp:
		return m.shift(-1)
	case moveDown:
		return m.shift(1)
	}
	return m, nil
}

// appendQueue 는 선곡 결과를 큐에 **붙인다.** 갈아끼우지 않는다.
//
// "이거 뒤에 몇 곡 더" 는 지금 듣는 것을 지키겠다는 말이다. 그런데 AI 에게는
// 갈아끼우는 도구밖에 없어서, 붙여 달라는 말이 늘 처음으로 튀는 결과가 됐다.
// 사람이 /later 로 하던 일을 AI 도 하게 하는 것이 이 함수다.
func (m Model) appendQueue(res intent.Result, atEnd bool) (app.App, tea.Cmd) {
	m = m.noteTurn(res)
	m.usage.PromptTokens += res.Usage.PromptTokens
	m.usage.CompletionTokens += res.Usage.CompletionTokens
	m.usage.CostUsd += res.Usage.CostUsd

	// 붙일 큐가 없으면 붙이는 것이 곧 만드는 것이다. 빈 큐에 "더 틀어줘" 는
	// "틀어줘" 와 같은 말이라 여기서 갈라 두면 두 경로가 같은 일을 한다.
	if len(m.queue) == 0 {
		return m.applyQueue(res)
	}

	l := data.Lib()
	add := make([]api.QueueItem, 0, len(res.Picks))
	for _, p := range res.Picks {
		t, ok := l.Track(p.TrackID)
		if !ok {
			continue // 스키마가 막지만, 없는 id 는 조용히 버린다
		}
		// 이미 큐에 있는 곡은 넘어간다. 같은 곡이 두 자리를 차지하면
		// 자리번호로 곡을 찾는 길이 전부 흔들린다(queueAt).
		if m.queueAt(t.Id) >= 0 {
			continue
		}
		reason := p.Reason
		add = append(add, api.QueueItem{
			Track: t, Reason: &reason,
			State: api.Pending, Origin: api.QueueItemOriginGeneration,
		})
	}
	if len(add) == 0 {
		return m, app.SayErr(m.Name(), errNoTracks)
	}

	at := len(m.queue)
	if !atEnd {
		at = m.playingAt() + 1
	}
	next := append([]api.QueueItem{}, m.queue[:at]...)
	next = append(next, add...)
	next = append(next, m.queue[at:]...)

	// 제목은 갈아엎지 않는다. 큐는 여전히 원래 요청의 것이고, 붙인 것은
	// 그 위에 얹힌 것이다.
	m.note = res.Note
	mm, cmd := m.reseat(next, "")
	// 붙인 것을 보여준다. 어디에 얹혔는지 안 보이면 정말 얹혔는지 알 수 없다.
	(&mm).jumpTo(secQueue, "")
	return mm, cmd
}
