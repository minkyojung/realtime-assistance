package musicapp

import (
	"testing"

	"amcli/tui/internal/music"
	tea "charm.land/bubbletea/v2"
)

// 우리가 도움말에 내놓은 키는 실제로 무언가를 해야 한다.
//
// 눌러도 아무 일이 없는 키를 약속하면, 한 번 데인 사람은 도움말 전체를
// 못 믿게 된다 — ctrl+o 가 그렇게 남았다.
func TestKeysWeOfferDoSomething(t *testing.T) {
	m := atSection(t, secSongs)
	m.bodyH = 20
	// 커서를 가운데 둔다. 맨 위에서는 위로 갈 데가 없어서, 안 움직이는
	// 것이 옳은데도 안 묶인 것처럼 보인다.
	if len(m.rows()) < 4 {
		t.Skip("픽스처에 줄이 모자라다")
	}
	m.listIdx = 2

	for _, b := range m.Keys() {
		for _, k := range b.Keys() {
			before := m
			next, cmd := before.Update(pressOf(k))
			if cmd == nil && sameScreen(before, next.(Model)) {
				t.Errorf("도움말에 낸 %q 를 눌러도 아무 일이 없다", k)
			}
		}
	}
}

// 지금 뜻이 없는 키는 안 내놓는다.
//
// ctrl+g 는 권한이 막혔을 때만 설정을 연다. 평소에 도움말에 적어 두면
// 그 자리가 다시 거짓말이 된다.
func TestConditionalKeysAreOfferedOnlyWhenTheyWork(t *testing.T) {
	m := New()
	if hasKey(m, "ctrl+g") {
		t.Error("권한 문제가 없는데 설정 여는 키를 내놨다")
	}

	m.playerErr = music.ErrPermissionDenied
	if !hasKey(m, "ctrl+g") {
		t.Error("권한이 막혔는데 여는 법을 안 알려준다")
	}
}

func hasKey(m Model, want string) bool {
	for _, b := range m.Keys() {
		for _, k := range b.Keys() {
			if k == want {
				return true
			}
		}
	}
	return false
}

// 화면에 보이는 것이 달라졌는가. 명령을 안 내는 키를 가리는 데 쓴다.
func sameScreen(a, b Model) bool {
	return a.sectionIdx == b.sectionIdx && a.listIdx == b.listIdx &&
		a.listTop == b.listTop && a.drill == b.drill
}

func pressOf(name string) tea.KeyPressMsg {
	switch name {
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "shift+up":
		return tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift}
	case "shift+down":
		return tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModShift}
	case "shift+left":
		return tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModShift}
	case "shift+right":
		return tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift}
	}
	if len(name) == 6 && name[:5] == "ctrl+" {
		return tea.KeyPressMsg{Code: rune(name[5]), Mod: tea.ModCtrl}
	}
	return tea.KeyPressMsg{Code: rune(name[0]), Text: name}
}
