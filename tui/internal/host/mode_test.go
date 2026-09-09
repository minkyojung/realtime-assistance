package host

import (
	"strings"
	"testing"

	"amcli/tui/internal/musicapp"
	tea "charm.land/bubbletea/v2"
)

var shiftTab = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}

// shift+tab 은 두 모드를 왕복한다. 한 키로 들어가고 나온다.
//
// ctrl+f 로 잠깐 옮겼다가 되돌렸다. 터미널에서 ctrl+f 는 "커서 오른쪽"
// 이라, 손버릇대로 누른 사람이 치던 문장을 통째로 잃었다(keys.go).
func TestShiftTabTogglesMode(t *testing.T) {
	hm := New(musicapp.New())
	hm.LeaveHome()
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 32})

	m, _ = m.Update(shiftTab)
	m = typeText(m, "oasis")
	if out := m.View().Content; strings.Contains(out, "Peanut butter Sandwich") {
		t.Error("shift+tab 을 눌러 검색어를 쳤는데 목록이 걸러지지 않았다")
	}

	m, _ = m.Update(shiftTab)
	out := m.View().Content
	if strings.Contains(out, "oasis") {
		t.Error("모드를 나왔는데 검색어가 입력창에 남아 있다")
	}
	if !strings.Contains(out, "Peanut butter Sandwich") {
		t.Error("모드를 나왔는데 목록이 걸러진 채로 있다")
	}
}

// 모드는 입력창 자신이 말한다 — 커서 앞 글자가 곧 모드다.
func TestPromptSaysMode(t *testing.T) {
	hm := New(musicapp.New())
	hm.LeaveHome()
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 32})

	if out := m.View().Content; !strings.Contains(out, "Ask AI") {
		t.Error("입력창 앞에 Ask AI 가 없다")
	}
	m, _ = m.Update(shiftTab)
	out := m.View().Content
	if !strings.Contains(out, "Search") {
		t.Error("검색 모드인데 입력창 앞이 Search 가 아니다")
	}
	if strings.Contains(out, "Ask AI") {
		t.Error("검색 모드인데 Ask AI 가 남아 있다")
	}
}

// 입력창에 테두리가 있고, 그 색이 모드를 따라간다.
func TestInputBorderFollowsMode(t *testing.T) {
	hm := New(musicapp.New())
	hm.LeaveHome()
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 32})

	ask := m.View().Content
	if !strings.Contains(ask, "╭") || !strings.Contains(ask, "╰") {
		t.Fatal("입력창에 테두리가 없다")
	}
	m, _ = m.Update(shiftTab)
	if search := m.View().Content; borderLine(ask) == borderLine(search) {
		t.Error("모드를 바꿨는데 테두리 색이 그대로다")
	}
}

// 테두리 윗줄만 잘라낸다. 색은 여기 ANSI 로 들어 있다.
func borderLine(view string) string {
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(l, "╭") {
			return l
		}
	}
	return ""
}

// 모드는 앱 안에서만 뜻이 있다. 홈은 그 키를 삼킨다.
func TestModeKeyOnHomeDoesNothing(t *testing.T) {
	hm := New(musicapp.New())
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 32})
	before := m.View().Content

	m, _ = m.Update(shiftTab)
	if state(t, m).mode == modeSearch {
		t.Error("홈인데 검색 모드로 들어갔다")
	}
	if m.View().Content != before {
		t.Error("홈에서 shift+tab 에 화면이 달라졌다")
	}
}

// **ctrl+f 는 우리 것이 아니다.**
//
// 터미널에서 ctrl+f 는 "커서 오른쪽" 이고 textarea 의 기본 키맵에도 그렇게
// 들어 있다. 호스트가 먼저 가로채면 손버릇대로 누른 사람은 커서가 아니라
// 모드를 바꾸게 되고, setMode 가 입력을 비우므로 치던 문장을 통째로 잃는다.
func TestCtrlFDoesNotEatWhatYouTyped(t *testing.T) {
	hm := New(musicapp.New())
	hm.LeaveHome()
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 32})
	m = typeText(m, "quiet")

	before := state(t, m).mode
	m, _ = m.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})

	if got := state(t, m).input.Value(); got != "quiet" {
		t.Errorf("ctrl+f 에 치던 글이 %q 가 됐다", got)
	}
	if state(t, m).mode != before {
		t.Error("ctrl+f 가 모드를 바꿨다 — 그 키는 커서를 옮기는 것이다")
	}
}
