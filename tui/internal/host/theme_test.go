package host

import (
	"strings"
	"testing"

	"amcli/tui/internal/musicapp"
	"amcli/tui/internal/style"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// 터미널 배경은 묻는 것이지 짐작하는 것이 아니다.
//
// 한때 그 값을 한 터미널에서 픽셀로 재서 박아 두었다. 밝은 배경을 쓰는
// 사람에게는 흰 글씨가 흰 바탕에 얹혀 **앱이 통째로 안 보였다.**

func TestWeAskTheTerminalForItsBackground(t *testing.T) {
	hm := New(musicapp.New())
	cmd := hm.Init()
	if cmd == nil {
		t.Fatal("Init 이 아무것도 안 시킨다")
	}
	// 배치 안을 뒤지지 않고 결과로 본다 — 물어봤으면 답이 들어 있다.
	if !hasBackgroundRequest(cmd) {
		t.Error("시작하면서 배경색을 안 물어본다")
	}
}

// 배치를 펼쳐 배경색을 묻는 명령이 있는지 본다.
//
// 타입으로 못 본다. 이 요청은 Bubble Tea 안쪽의 이름 없는 값이고, 런타임이
// 그것을 보고 터미널에 물은 뒤 나중에 BackgroundColorMsg 로 답한다. 대신
// **같은 값인지**로 본다 — 빈 구조체라 비교가 된다.
func hasBackgroundRequest(cmd tea.Cmd) bool {
	want := tea.RequestBackgroundColor()
	msg := cmd()
	b, ok := msg.(tea.BatchMsg)
	if !ok {
		return msg == want
	}
	for _, c := range b {
		if c != nil && c() == want {
			return true
		}
	}
	return false
}

// 답이 오면 팔레트가 그 배경 위에서 다시 계산된다.
func TestTheAnswerRepaintsThePalette(t *testing.T) {
	t.Cleanup(func() { style.Retheme(lipgloss.Color("#1C1B27")) })

	hm := New(musicapp.New())
	hm.LeaveHome()
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 32})
	dark := m.View().Content

	m, _ = m.Update(tea.BackgroundColorMsg{Color: lipgloss.Color("#FFFFFF")})
	if light := m.View().Content; light == dark {
		t.Error("배경이 희다고 알려줬는데 화면이 그대로다")
	}
	// 흰 바탕에 흰 글씨를 쓰지 않는다.
	if strings.Contains(m.View().Content, "232;232;234") {
		t.Error("흰 배경인데 본문이 여전히 밝은 회색이다")
	}
}

// ctrl+z 는 셸로 잠깐 나가는 키다.
//
// 우리가 안 받으면 아무 일도 안 일어난다 — 터미널을 raw 모드로 잡고 있어서
// 신호가 아니라 글자로 들어온다. 그래서 딴 일을 하려면 앱을 통째로 끄는
// 수밖에 없었다.
func TestCtrlZStepsOutToTheShell(t *testing.T) {
	m, _ := twoAppHost()
	handled, _, cmd := state(t, m).handleKey(tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl})
	if !handled || cmd == nil {
		t.Fatal("ctrl+z 를 안 받는다")
	}
	if _, ok := cmd().(tea.SuspendMsg); !ok {
		t.Errorf("셸로 안 나간다: %T", cmd())
	}
}

// ctrl+l 은 깨진 화면을 다시 그린다. 지금까지는 껐다 켜는 것이 유일한 복구였다.
func TestCtrlLRedrawsTheScreen(t *testing.T) {
	m, _ := twoAppHost()
	handled, _, cmd := state(t, m).handleKey(tea.KeyPressMsg{Code: 'l', Mod: tea.ModCtrl})
	if !handled || cmd == nil {
		t.Fatal("ctrl+l 을 안 받는다")
	}
	if cmd() != tea.ClearScreen() {
		t.Errorf("화면을 안 지운다: %T", cmd())
	}
}
