package musicapp

import (
	"amcli/tui/internal/api"
	"amcli/tui/internal/app"
	"amcli/tui/internal/data"
	tea "charm.land/bubbletea/v2"
)

// 목록에서 enter 를 눌렀을 때.
//
// **뜻은 하나다 — "고른 곡부터 이 목록을 이어서 튼다."**
//
// 예전에는 자리마다 뜻이 달랐다. 라이브러리 곡은 그 곡 하나, 큐의 곡은 큐의
// 그 자리, 카탈로그 곡은 담기. 어디에 있느냐에 따라 결과가 달라서 누르기
// 전에 무엇이 일어날지 알 수 없었다.
//
// 그리고 "곡 하나를 튼다"는 길은 **애초에 동작하지 않았다.** Music.app 에
// 곡 객체 하나를 주면 그것만 틀고 멈춘다 — 이어서 나올 것이 없기 때문이다.
// 이어 들으려면 담을 것(플레이리스트)을 줘야 한다. 그래서 한 곡을 트는 길을
// 없애는 것이 곧 그 버그를 없애는 일이었다.

// 고른 곡부터 큐에 담을 곡 수.
//
// 목록 전체를 담지 않는 이유는 곡마다 AppleScript duplicate 가 한 번씩
// 돌기 때문이다. 202곡이면 몇 초가 통째로 든다. 서른이면 두 시간쯤이라
// 한 자리에서 듣기에 모자라지 않고, 모자라면 다시 고르면 된다.
const maxPickQueue = 30

// playFrom 은 고른 줄부터 아래로 이 목록을 큐로 만들고 튼다.
//
// 큐를 갈아끼운다. 목록에서 곡을 고르는 것은 "이걸 지금부터 듣겠다"는
// 뜻이고, 그것은 새 한 자리이기 때문이다 — 음악 앱들이 그렇게 한다.
func (m Model) playFrom(rows []listRow, at int, label string) (Model, tea.Cmd) {
	items := make([]api.QueueItem, 0, maxPickQueue)
	ids := make([]string, 0, maxPickQueue)
	for i := at; i < len(rows) && len(ids) < maxPickQueue; i++ {
		t := rows[i].track
		if t == nil || t.PersistentId == nil {
			continue // 머리글·묶음·아직 내 것이 아닌 곡은 담을 수 없다
		}
		items = append(items, api.QueueItem{
			Id: int64(len(items) + 1), SessionId: 1, Position: len(items) + 1,
			Track: *t, State: api.Pending, Origin: api.QueueItemOriginManual,
		})
		ids = append(ids, *t.PersistentId)
	}
	if len(ids) == 0 {
		return m, app.SayErr(m.Name(), errNoTracks)
	}

	// 듣던 곡은 사용자가 다른 것을 골라서 끝난다. 넘긴 것과 뜻이 다르다.
	m = m.hintEnd(data.EndedPicked)

	m.queue = items
	m.queueTitle = label
	// 사람이 목록에서 직접 골랐다. 청해서 나온 곡이 아니므로 요청을 떼어낸다 —
	// **0 은 빈 값이 아니라 "부탁받지 않았다"는 뜻이다.**
	m.turnID = 0
	m.nowPlayingID = items[0].Track.Id
	m.positionMs, m.playing = 0, true
	// 플레이리스트를 새로 쓸 때까지는 화면과 Music.app 이 어긋난 상태다.
	m.queuePID = ""
	return m, cmdWriteQueue(ids, 1, 0)
}

// pickLabel 은 이 재생이 어디서 시작됐는지다. 상태줄과 기록에 남는다.
func (m Model) pickLabel() string {
	if m.drill != nil {
		return m.drill.Name
	}
	if m.searching() {
		return "Search " + m.filter
	}
	return m.sections[m.sectionIdx].label
}

// playOne 은 곡 하나로 큐를 만들고 튼다.
//
// 카탈로그에서 방금 담은 곡처럼 뒤에 이어 붙일 것이 없을 때 쓴다. 곡 객체를
// 그냥 틀지 않는 이유는 위와 같다 — Music.app 은 그것을 "이 곡만"으로 읽는다.
func (m Model) playOne(t api.Track) (Model, tea.Cmd) {
	if t.PersistentId == nil {
		return m, app.SayErr(m.Name(), errNoTracks)
	}
	return m.playFrom([]listRow{{track: &t}}, 0, t.Title)
}

// Hint 는 지금 커서가 놓인 줄에서 enter 가 하는 일이다.
//
// 자리마다 뜻이 다르던 시절에는 누르기 전에 무엇이 일어날지 알 수 없었다.
// 뜻은 이제 하나지만(위), **그 하나가 무엇인지는 화면이 말해야 한다** —
// 카탈로그 줄에서 "담고 재생"임을 모르면 눌러 보기 전에는 알 길이 없다.
//
// 큐에서만 /remove 를 덧붙인다. 커서가 가리키는 것을 빼는 일이라 그 자리에서만
// 뜻이 있고, 다른 목록에서 내놓으면 무엇이 빠지는지가 모호해진다.
func (m Model) Hint() string {
	rows := m.rows()
	if m.listIdx < 0 || m.listIdx >= len(rows) {
		return ""
	}
	switch r := rows[m.listIdx]; {
	case r.isMore():
		return "enter  show the rest"
	case r.catalog != nil:
		if inLibrary(*r.catalog) {
			return "enter  play"
		}
		return "enter  add it and play"
	case r.group != nil:
		return "enter  open"
	case r.track != nil:
		if m.sections[m.sectionIdx].kind == secQueue {
			return "enter  play from here     /remove  drop it"
		}
		return "enter  play from here"
	}
	return ""
}
