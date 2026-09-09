package musicapp

import (
	"amcli/tui/internal/api"
	"amcli/tui/internal/app"
	"amcli/tui/internal/data"
	tea "charm.land/bubbletea/v2"
)

// 이 앱이 등록하는 슬래시 명령.
//
// 앱은 자기 키 바인딩을 만들지 않는다. 하고 싶은 것이 있으면 여기 등록한다.
// 호스트가 전부 모아 하나의 팔레트로 보여준다 — docs/07-호스트-계약.md 3-1.
//
// 명령이 모델을 거치지 않는 것이 체감 속도를 지탱한다.
func (m Model) Commands() []app.Command {
	return []app.Command{
		{Name: "/queue", Help: "jump to the current queue",
			Run: m.jumpCmd(secQueue)},
		{Name: "/unplayed", Help: "tracks you added but never played",
			Run: m.jumpCmd(secUnplayed)},
		{Name: "/save", Arg: "<name>", Help: "save the queue as an Apple Music playlist",
			Run: m.saveCmd},
		{Name: "/clear", Help: "empty the queue",
			Run: func(string) tea.Cmd { return send(clearQueueMsg{}) }},
	}
}

// 명령의 결과는 메시지로 돌아온다. 그래야 Update 한 곳에서만 상태가 바뀐다.
type (
	clearQueueMsg struct{}
	jumpMsg       struct{ kind sectionKind }
)

func send(msg tea.Msg) tea.Cmd { return func() tea.Msg { return msg } }

func (m Model) jumpCmd(kind sectionKind) func(string) tea.Cmd {
	return func(string) tea.Cmd { return send(jumpMsg{kind: kind}) }
}

func (m Model) saveCmd(arg string) tea.Cmd {
	if arg == "" {
		return send(errMsg{errNoName})
	}
	if len(m.queue) == 0 {
		return send(errMsg{errNoQueue})
	}
	return cmdSavePlaylist(arg, m.queueTracks())
}

type errMsg struct{ err error }

func (m *Model) jumpTo(kind sectionKind) {
	for i, s := range m.sections {
		if s.kind == kind {
			m.sectionIdx = i
			m.listIdx, m.listTop = 0, 0
			return
		}
	}
}

// unplayedTracks — 담아두고 한 번도 재생하지 않은 곡.
// 이 서비스가 말하려는 것이 목록 하나로 보이는 자리다.
func unplayedTracks(l *data.Library) []listRow {
	var out []listRow
	for _, t := range l.Songs() {
		if t.PlayCount == 0 && t.LastPlayedAt == nil {
			tt := t
			out = append(out, listRow{track: &tt})
		}
	}
	return out
}

var _ = api.Track{}
