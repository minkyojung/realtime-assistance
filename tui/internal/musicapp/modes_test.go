package musicapp

import (
	"strings"
	"testing"

	"amcli/tui/internal/app"
	"amcli/tui/internal/music"
	tea "charm.land/bubbletea/v2"
)

// 켜져 있는 것은 화면이 말해야 한다.
//
// 켜 두고 잊으면 고장으로 읽힌다 — 순서대로 안 나오는데 왜 그런지 화면
// 어디에도 없으면 사용자가 의심할 것은 앱뿐이다.

// **기본값이면 한 칸도 안 늘어난다.**
//
// "화면에 상시로 자리를 주지 않는다"(docs/07 2절). 사이드바를 없앤 것과
// 같은 규칙이고, 이걸 어기면 평소 화면이 조금씩 계속 붐빈다.
func TestNothingIsDrawnWhenEverythingIsOff(t *testing.T) {
	m := New()
	m.bodyH = 20
	if got := m.marks(); got != "" {
		t.Errorf("전부 꺼졌는데 %q 를 그린다", got)
	}
	if got := m.Status(); strings.ContainsAny(got, "⇄↻♥") {
		t.Errorf("상태줄에 표시가 샜다: %q", got)
	}
}

func TestMarksShowWhatIsOn(t *testing.T) {
	for _, c := range []struct {
		name  string
		state music.PlayerState
		want  string
	}{
		{"섞기", music.PlayerState{Shuffle: true}, "⇄"},
		{"한 곡 반복", music.PlayerState{Repeat: music.RepeatOne}, "↻1"},
		{"큐 반복", music.PlayerState{Repeat: music.RepeatAll}, "↻"},
		{"좋아요", music.PlayerState{Favorited: true}, "♥"},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := New()
			m.bodyH = 20
			m.live = c.state
			if got := m.marks(); !strings.Contains(got, c.want) {
				t.Errorf("%q 가 없다: %q", c.want, got)
			}
			if got := m.Status(); !strings.Contains(got, c.want) {
				t.Errorf("상태줄에 %q 가 없다", c.want)
			}
		})
	}
}

// 반복을 껐으면 그리지 않는다. off 도 값이지만 기본값이다.
func TestRepeatOffDrawsNothing(t *testing.T) {
	m := New()
	m.bodyH = 20
	m.live = music.PlayerState{Repeat: music.RepeatOff}
	if got := m.marks(); got != "" {
		t.Errorf("반복이 꺼졌는데 %q 를 그린다", got)
	}
}

// /shuffle 은 토글이다. 지금 켜져 있으면 끄겠다고 말해야 한다.
func TestShuffleCommandTogglesAndSaysWhich(t *testing.T) {
	m := New()
	m.bodyH = 20

	off := findCmd(t, m, "/shuffle")
	if !strings.Contains(off.Help, "on") {
		t.Errorf("꺼져 있는데 켠다고 안 한다: %q", off.Help)
	}

	m.live = music.PlayerState{Shuffle: true}
	on := findCmd(t, m, "/shuffle")
	if !strings.Contains(on.Help, "off") {
		t.Errorf("켜져 있는데 끈다고 안 한다: %q", on.Help)
	}
}

// 잘못 친 인자는 Music.app 까지 가기 전에 막고, 무엇이 틀렸는지 말한다.
//
// 조용히 무시하면 사용자는 켜졌다고 믿고 다음 행동을 한다.
func TestBadArgumentsSayWhatIsWrong(t *testing.T) {
	for _, c := range []struct{ name, arg string }{
		{"/repeat", "loop"},
		{"/repeat", ""},
		{"/volume", "loud"},
		{"/rate", "많이"},
	} {
		t.Run(c.name+" "+c.arg, func(t *testing.T) {
			m := New()
			m.bodyH = 20
			cmd := findCmd(t, m, c.name).Run(c.arg)
			if cmd == nil {
				t.Fatal("아무 말도 안 한다")
			}
			msg, ok := cmd().(app.SayMsg)
			if !ok {
				t.Fatalf("말이 아니라 %T 가 왔다", cmd())
			}
			if !msg.Err {
				t.Errorf("틀렸는데 실패로 안 알린다: %q", msg.Text)
			}
		})
	}
}

// 다섯 개가 다 팔레트에 있어야 한다. 없으면 있는 줄도 모른다.
func TestAllFiveAreInThePalette(t *testing.T) {
	m := New()
	m.bodyH = 20
	for _, name := range []string{"/shuffle", "/repeat", "/volume", "/love", "/rate"} {
		findCmd(t, m, name)
	}
}

func findCmd(t *testing.T, m Model, name string) app.Command {
	t.Helper()
	for _, c := range m.Commands() {
		if c.Name == name {
			if c.Run == nil {
				t.Fatalf("%s 가 아무것도 하지 않는다", name)
			}
			return c
		}
	}
	t.Fatalf("팔레트에 %s 가 없다", name)
	return app.Command{}
}

var _ tea.Cmd = cmdShuffle(true)

// `/` 만 쳤을 때 보이는 여덟 줄이 "이 앱으로 무엇을 할 수 있는가"의 답이다.
//
// 예전에는 여덟 줄이 전부 섹션 이동이라, 정작 할 수 있는 일이 아래로 밀려
// 보이지 않았다. 되돌아가기 쉬운 자리라 못 박아 둔다.
func TestPaletteShowsActionsBeforeNavigation(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test")
	m := New()
	m.bodyH = 20

	const visible = 8 // host/overlay.go 의 maxOverlayRows
	cmds := m.Commands()
	if len(cmds) < visible {
		t.Fatalf("명령이 %d개뿐이다", len(cmds))
	}

	// 첫 여덟 줄에 섹션 이동이 끼어 있으면 안 된다.
	jumps, _ := m.jumpCommands()
	isJump := map[string]bool{}
	for _, j := range jumps {
		isJump[j.Name] = true
	}
	for i, c := range cmds[:visible] {
		if isJump[c.Name] {
			t.Errorf("%d번째 줄이 섹션 이동이다: %s — tab 으로도 갈 수 있는 것은 뒤로", i+1, c.Name)
		}
	}

	// 그리고 여기서만 갈 수 있는 것들이 보여야 한다.
	//
	// 아직 안 켠 것이 있으면 그것이 맨 앞을 가져간다(command.go). 여기서
	// 보려는 것은 그 다음의 순서이므로 켜 두고 잰다.
	seen := map[string]bool{}
	for _, c := range cmds[:visible] {
		seen[c.Name] = true
	}
	for _, want := range []string{"/remove", "/love", "/shuffle"} {
		if !seen[want] {
			t.Errorf("%s 가 첫 여덟 줄에 없다 — `/` 말고는 길이 없는 명령이다", want)
		}
	}
}

// 다른 길이 있는 것은 뒤로 간다.
func TestCommandsWithAKeyRankLower(t *testing.T) {
	// /pause 는 shift+↓ 로도 된다. /remove 는 `/` 뿐이다.
	if paletteRank("/pause") <= paletteRank("/remove") {
		t.Error("키가 있는 /pause 가 키가 없는 /remove 보다 앞이다")
	}
}

// 플레이리스트는 수가 정해져 있지 않아 맨 뒤여야 한다.
// 앞에 두면 라이브러리에 따라 나머지가 통째로 밀려난다.
func TestPlaylistsStayLast(t *testing.T) {
	m := New()
	m.bodyH = 20
	_, playlists := m.jumpCommands()
	if len(playlists) == 0 {
		t.Skip("픽스처에 플레이리스트가 없다")
	}
	cmds := m.Commands()
	first := len(cmds) - len(playlists)
	for i, c := range cmds {
		if c.Name == playlists[0].Name && i != first {
			t.Errorf("플레이리스트가 %d번째에 있다 — 맨 뒤(%d)여야 한다", i, first)
		}
	}
}
