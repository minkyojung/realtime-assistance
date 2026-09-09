package musicapp

import (
	"strconv"
	"strings"
	"unicode"

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
	// 고정 섹션 → 액션 → 플레이리스트 순. 팔레트는 여덟 줄만 보여주므로
	// (host/overlay.go) 순서가 곧 "`/` 만 쳤을 때 보이는 것"이다.
	// 플레이리스트는 수가 정해져 있지 않아 맨 뒤에 둔다 — 앞에 두면
	// 라이브러리에 따라 액션이 통째로 밀려난다.
	fixed, playlists := m.jumpCommands()
	fixed = append(fixed, m.modeCommands()...)
	return append(append(fixed, []app.Command{
		{Name: "/save", Arg: "<name>", Help: "save the queue as an Apple Music playlist",
			Run: m.saveCmd},
		{Name: "/pause", Help: "play or pause — same as shift+↓",
			Run: func(string) tea.Cmd { return cmdPlayPause() }},
		{Name: "/remove", Help: "drop the selected track from the queue",
			Run: func(string) tea.Cmd { return send(removeSelectedMsg{}) }},
		{Name: "/clear", Help: "empty the queue",
			Run: func(string) tea.Cmd { return send(clearQueueMsg{}) }},
		{Name: "/reload", Help: "read your library from Music.app again",
			Run: m.reloadCmd},
		{Name: "/login", Help: "connect your Apple Music account (opens a browser)",
			Run: m.loginCmd},
		{Name: "/shazam", Help: "listen for a few seconds and name what is playing",
			Run: m.shazamCmd},
	}...), playlists...)
}

// 액션 명령의 이름. 섹션 이름이 여기에 겹치지 않게 하는 데 쓴다.
var actionNames = []string{"/save", "/pause", "/clear", "/reload", "/login", "/shazam"}

// jumpCommands — 섹션마다 곧장 가는 명령을 하나씩 낸다.
//
// tab 은 다음 칸으로만 간다. 되돌아가는 키(shift+tab)를 호스트가 모드
// 전환에 가져가면서, 멀리 있는 섹션에 바로 갈 길이 필요해졌다. 사이드바를
// 두지 않는 이 화면에서 그 길은 `/` 다 — docs/07 2절.
//
// 플레이리스트도 섹션이므로 여기 함께 나온다. 이름은 라이브러리가 정하고
// 우리가 못 고르므로, 명령으로 쓸 수 있는 모양으로 깎는다.
func (m Model) jumpCommands() (fixed, playlists []app.Command) {
	taken := map[string]bool{"/" + m.Name(): true}
	for _, n := range actionNames {
		taken[n] = true
	}

	for _, s := range m.sections {
		// 인식 섹션에는 따로 가는 명령을 내지 않는다. `/shazam` 이 이미
		// 액션이고, 그 액션이 끝나면 알아서 그 섹션으로 데려간다.
		if s.kind == secShazam {
			continue
		}
		name := sectionCommand(s.kind)
		if name == "" {
			// 플레이리스트. 이름을 만들 수 없으면(기호뿐인 이름 등)
			// 명령을 내지 않는다. 그 섹션은 tab 으로 간다.
			sl := slug(s.label)
			if sl == "" {
				continue
			}
			name = "/" + sl
		}
		base := name
		for n := 2; taken[name]; n++ {
			name = base + "-" + strconv.Itoa(n)
		}
		taken[name] = true

		help := sectionHelp[s.kind]
		if help == "" {
			help = "go to " + s.label
		}
		c := app.Command{Name: name, Help: help, Run: m.jumpCmd(s.kind, s.label)}
		if s.kind == secPlaylist {
			playlists = append(playlists, c)
		} else {
			fixed = append(fixed, c)
		}
	}
	return fixed, playlists
}

// 고정 섹션의 명령 이름. 라벨에서 깎으면 `/recently-added` 처럼 길어진다.
func sectionCommand(k sectionKind) string {
	switch k {
	case secRecent:
		return "/recent"
	case secArtists:
		return "/artists"
	case secAlbums:
		return "/albums"
	case secSongs:
		return "/songs"
	case secQueue:
		return "/queue"
	case secUnplayed:
		return "/unplayed"
	case secCatalog:
		return "/catalog"
	}
	return "" // secPlaylist — 이름에서 만든다
}

// 라벨이 못 하는 말만 따로 적는다. 나머지는 "go to <라벨>" 로 충분하다.
var sectionHelp = map[sectionKind]string{
	secUnplayed: "tracks you added but never played",
	secCatalog:  "search Apple Music beyond your library",
}

// slug — 이름을 명령으로 쓸 수 있는 모양으로 깎는다.
//
// 팔레트는 친 것을 소문자로 바꿔 앞글자를 맞춰 보고(host/overlay.go),
// 명령 이름은 공백에서 잘린다(runCommand). 둘 다 지켜야 한다.
// 한글은 그대로 남는다 — 소문자도 공백도 아니므로 칠 수 있다.
func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteRune('-')
			dash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// 명령의 결과는 메시지로 돌아온다. 그래야 Update 한 곳에서만 상태가 바뀐다.
type (
	clearQueueMsg struct{}

	// removeSelectedMsg — 커서가 놓인 곡을 큐에서 뺀다.
	//
	// 명령이 곡을 직접 지목하지 않는 이유는, 목록에서 보고 고른 것이
	// 이미 지목이기 때문이다. 이름을 다시 치게 하면 두 번 고르는 셈이다.
	removeSelectedMsg struct{}
	jumpMsg           struct {
		kind sectionKind
		// 플레이리스트는 종류가 같고 이름만 다르다. 이름 없이 옮기면
		// 언제나 첫 플레이리스트로 간다.
		label string
	}
)

func send(msg tea.Msg) tea.Cmd { return func() tea.Msg { return msg } }

func (m Model) jumpCmd(kind sectionKind, label string) func(string) tea.Cmd {
	return func(string) tea.Cmd { return send(jumpMsg{kind: kind, label: label}) }
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

func (m *Model) jumpTo(kind sectionKind, label string) {
	for i, s := range m.sections {
		if s.kind == kind && (kind != secPlaylist || s.label == label) {
			m.sectionIdx = i
			m.drill = nil
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

// reloadCmd — 라이브러리를 다시 읽는다.
//
// 주기적으로 다시 읽지 않는 이유: Apple 이벤트는 비싸고, 라이브러리는
// 사용자가 바꾼다. 사용자는 자기가 바꾼 때를 안다.
func (m Model) reloadCmd(string) tea.Cmd {
	if m.syncing {
		return send(errMsg{errSyncing})
	}
	return cmdDumpLibrary(true)
}

func (m Model) loginCmd(string) tea.Cmd {
	if m.cat == nil {
		return send(errMsg{errCatalogNotConfigured})
	}
	// 브라우저가 곧 포커스를 가져간다. 그 전에 왜 그런지 로그에 남긴다.
	return tea.Batch(
		send(loginStartedMsg{}),
		app.Say(m.Name(), "Approve the Apple Music sign-in in your browser (up to 3 minutes)"),
		cmdCatalogLogin(m.cat),
	)
}
