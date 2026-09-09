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
