package musicapp

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"amcli/tui/internal/api"
	"amcli/tui/internal/app"
	"amcli/tui/internal/applemusic"
	"amcli/tui/internal/data"
	"amcli/tui/internal/secrets"
	tea "charm.land/bubbletea/v2"
)

// 이 앱이 등록하는 슬래시 명령.
//
// 앱은 자기 키 바인딩을 만들지 않는다. 하고 싶은 것이 있으면 여기 등록한다.
// 호스트가 전부 모아 하나의 팔레트로 보여준다 — docs/07-호스트-계약.md 3-1.
//
// 명령이 모델을 거치지 않는 것이 체감 속도를 지탱한다.
// 팔레트에 먼저 보일 순서.
//
// `/` 만 치면 여덟 줄이 보인다(host/overlay.go). **그 여덟 줄이 "이 앱으로
// 무엇을 할 수 있는가"의 답이다.** 예전에는 여덟 줄이 전부 섹션 이동이라,
// 정작 할 수 있는 일(/remove·/love·/shuffle)이 아래로 밀려 보이지 않았다.
//
// 기준은 **`/` 말고 다른 길이 있는가**다. 섹션은 tab 으로도 가고 /pause 는
// shift+↓ 로도 된다. 여기서만 갈 수 있는 것을 앞에 둔다.
//
// 여기 없는 이름은 이 뒤에 원래 순서대로 붙는다 — 섹션, 그다음 플레이리스트.
var paletteOrder = []string{
	"/next", "/later", "/remove", "/up", "/down",
	"/love", "/shuffle", "/save", "/shazam", "/clear", "/repeat", "/rate",
	"/volume", "/pause", "/reload", "/ai", "/setup", "/login",
}

func paletteRank(name string) int {
	for i, n := range paletteOrder {
		if n == name {
			return i
		}
	}
	return len(paletteOrder)
}

// offCommands — 지금 켜면 되는 것들의 명령 이름.
//
// 무엇이 꺼졌는지 아는 것은 앱이다. 첫 화면도 팔레트도 같은 이 답을 쓴다 —
// 두 군데서 따로 판단하면 한쪽만 고쳤을 때 조용히 어긋난다.
func (m Model) offCommands() map[string]bool {
	out := map[string]bool{}
	if !secrets.HasOpenAIKey() {
		out["/ai"] = true
	}
	if m.cat == nil {
		out["/setup"] = true
	} else if m.cat.UserToken == "" {
		out["/login"] = true
	}
	return out
}

func (m Model) Commands() []app.Command {
	// 플레이리스트는 수가 정해져 있지 않아 맨 뒤에 둔다 — 앞에 두면
	// 라이브러리에 따라 나머지가 통째로 밀려난다.
	jumps, playlists := m.jumpCommands()
	actions := []app.Command{
		{Name: "/save", Arg: "<name>", Help: "save the queue as an Apple Music playlist",
			Run: m.saveCmd},
		{Name: "/pause", Help: "play or pause — same as shift+↓",
			Run: func(string) tea.Cmd { return cmdPlayPause() }},
		{Name: "/next", Help: "play the selected track right after this one",
			Run: func(string) tea.Cmd { return send(reorderMsg{kind: putNext}) }},
		{Name: "/later", Help: "add the selected track to the end of the queue",
			Run: func(string) tea.Cmd { return send(reorderMsg{kind: putLater}) }},
		{Name: "/remove", Help: "drop the selected track from the queue",
			Run: func(string) tea.Cmd { return send(removeSelectedMsg{}) }},
		{Name: "/up", Help: "move the selected track one place earlier",
			Run: func(string) tea.Cmd { return send(reorderMsg{kind: moveUp}) }},
		{Name: "/down", Help: "move the selected track one place later",
			Run: func(string) tea.Cmd { return send(reorderMsg{kind: moveDown}) }},
		{Name: "/clear", Help: "empty the queue",
			Run: func(string) tea.Cmd { return send(clearQueueMsg{}) }},
		{Name: "/reload", Help: "read your library from Music.app again",
			Run: m.reloadCmd},
		{Name: "/ai", Arg: "<key>", Help: "turn on AI · your provider API key",
			Run: aiKeyCmd, Secret: true},
		{Name: "/setup", Arg: "<team ID>", Help: "connect Apple Music · your Apple Developer team ID",
			Run: m.setupCmd},
		{Name: "/login", Help: "connect your Apple Music account (opens a browser)",
			Run: m.loginCmd},
	}
	// 헬퍼가 없으면 내지 않는다. 배포판이 그렇다 — 못 하는 일을 팔레트에
	// 올려 두면 눌러 본 사람에게 빨간 줄로 답하게 된다.
	if m.shzOK {
		actions = append(actions, app.Command{
			Name: "/shazam", Help: "listen for a few seconds and name what is playing",
			Run: m.shazamCmd,
		})
	}
	actions = append(actions, m.modeCommands()...)
	// 순위대로 세운다. 같은 순위는 원래 자리를 지킨다.
	//
	// **아직 안 켠 것이 있으면 그것이 맨 앞이다.** 첫 화면이 "Type / to
	// set up" 이라고 시켜 놓고 `/` 를 치면 큐 조작 명령만 여덟 줄 나왔다 —
	// 시킨 대로 했는데 시킨 것이 없는 화면이었다. 팔레트는 여덟 줄에서
	// 끊기므로 순서가 곧 보이느냐 마느냐다.
	off := m.offCommands()
	sort.SliceStable(actions, func(i, j int) bool {
		ri, rj := paletteRank(actions[i].Name), paletteRank(actions[j].Name)
		if oi, oj := off[actions[i].Name], off[actions[j].Name]; oi != oj {
			return oi // 꺼진 쪽이 앞
		}
		return ri < rj
	})
	return append(append(actions, jumps...), playlists...)
}

// 액션 명령의 이름. 섹션 이름이 여기에 겹치지 않게 하는 데 쓴다.
var actionNames = []string{"/save", "/pause", "/clear", "/reload", "/login", "/shazam", "/setup", "/ai",
	"/next", "/later", "/up", "/down"}

// jumpCommands — 섹션마다 곧장 가는 명령을 하나씩 낸다.
//
// tab 은 다음 칸으로만 간다. 한 칸씩 걸어서는 멀리 있는 섹션이 너무 멀고,
// 사이드바를 두지 않는 이 화면에서 그 길은 `/` 다 — docs/07 2절.
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

	// reorderMsg — 큐를 만든 뒤에 손대는 일들. removeSelectedMsg 와 같이
	// 커서가 곧 지목이다 (reorder.go).
	reorderMsg struct{ kind reorderKind }
	jumpMsg    struct {
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

// setupCmd — Team ID 를 설정 파일에 적고 카탈로그를 다시 켠다.
//
// 사람이 적어야 하는 유일한 값이다. p8 경로와 Key ID 는 설정 폴더에서
// 저절로 찾는다 — 애플이 키 파일을 AuthKey_<KEYID>.p8 로 내려주기 때문이다.
//
// 환경변수로 하지 않는 이유는 그것이 셸에만 살기 때문이다. 터미널을 새로
// 열거나 셸을 안 거치고 띄우면 사라진다.
func (m Model) setupCmd(arg string) tea.Cmd {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return send(errMsg{errNoTeamID})
	}
	if err := applemusic.SaveTeamID(arg); err != nil {
		return send(errMsg{err})
	}
	// 적었으니 다시 읽는다. 앱을 껐다 켜게 하지 않는다.
	return tea.Batch(
		app.Say(m.Name(), "Saved. Connecting to Apple Music…"),
		func() tea.Msg { return cmdCatalogInit() },
	)
}

var errNoTeamID = errors.New(
	"which team? · /setup <team ID> — developer.apple.com › Membership")

// aiKeyCmd — AI 키를 키체인에 넣고 켠다.
//
// 명령 이름을 벤더가 아니라 **역할**로 부른다. 지금은 OpenAI 하나뿐이지만,
// 이름에 벤더를 박아두면 프로바이더가 바뀌거나 늘 때 사용자에게 보이는
// 것까지 전부 바꿔야 한다. 역할 이름은 추상화를 미리 만드는 것이 아니라
// **이름에 벤더를 안 박는 것**이라 비용이 0이다.
//
// 안에서 쓰는 이름은 벤더 그대로 둔다(secrets.AccountOpenAI). 그건 실제로
// OpenAI 키가 맞고, 두 번째가 생기면 따로 두는 것이 맞다.
//
// 환경변수로 받지 않는 이유는 그것이 셸에만 살기 때문이다. 터미널을 새로
// 열거나 셸을 안 거치고 띄우면 사라진다 — 애플 뮤직 설정에서 이미 겪었다.
//
// 키체인인 이유는 **비밀이기 때문이다.** Key ID·Team ID 는 식별자라 설정
// 파일에 두지만, 이 키는 뽑히면 남의 돈이 나간다(internal/secrets).
func aiKeyCmd(arg string) tea.Cmd {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return send(errMsg{errNoAPIKey})
	}
	if err := secrets.SaveOpenAIKey(arg); err != nil {
		return send(errMsg{err})
	}
	// 넣었는데 셸이 덮고 있으면 **그 자리에서** 말한다.
	//
	// "AI is on." 이라고만 답하면 방금 넣은 키로 나간다고 믿게 된다. 셸의
	// 옛 키가 잔액이 없으면 사용자는 자기가 방금 넣은 키를 의심한다 —
	// 화면이 맞다고 한 것을. gh 가 GH_TOKEN 이 있을 때 하는 말과 같다.
	if secrets.EnvOverridesKeychain() {
		return app.SayErr("music", errors.New("saved — but "+secrets.EnvOpenAIKey+
			" in your shell is used instead · unset it to use this one"))
	}
	// 저장 즉시 켜진다. 다음 요청부터 이 키로 나간다 — 앱을 껐다 켜지 않는다.
	return app.Say("music", "AI is on.")
}

var errNoAPIKey = errors.New(
	"which key? · /ai <key> — platform.openai.com › API keys")
