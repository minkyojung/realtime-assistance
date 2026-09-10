package host

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// 도움말은 **묶여 있는 키만** 약속한다.
//
// 한때 여기가 손으로 적은 표였고, 코드와 아무 관계가 없어 셋이 어긋났다.
// 그중 ctrl+o 는 받는 코드가 아예 없는데 도움말만 계속 약속했다 — 눌러 본
// 사람은 고장을 봤고, 한 줄이 틀리면 나머지 줄도 못 믿게 된다.
//
// 아래 테스트들이 지키는 것은 문구가 아니라 **어긋날 수 없다는 성질**이다.

// 도움말의 모든 줄은 실제 묶음이나 키가 아닌 안내 중 하나에서 나온다.
func TestEveryHelpRowComesFromABinding(t *testing.T) {
	m, _ := twoAppHost()
	st := state(t, m)

	known := map[string]bool{}
	for _, r := range notKeys {
		known[r.key] = true
	}
	for _, b := range append(keys.helpOrder(), st.app().Keys()...) {
		known[b.Help().Key] = true
	}
	for _, r := range st.helpRows() {
		if !known[r.key] {
			t.Errorf("%q 가 도움말에 있는데 묶인 데가 없다", r.key)
		}
	}
}

// 앱이 내는 키가 도움말에 이어 붙는다.
//
// 예전에는 호스트가 앱의 키를 문자열로 베껴 적었다. 앱이 바꿔도 호스트는
// 몰랐고, 그 베끼기가 ctrl+o 를 남긴 길이다.
func TestAppKeysReachTheHelp(t *testing.T) {
	m, _ := twoAppHost()
	st := state(t, m)
	sa := st.apps[st.current].(stubApp)
	sa.keys = []key.Binding{key.NewBinding(
		key.WithKeys("ctrl+y"), key.WithHelp("ctrl+y", "yank the thing"),
	)}
	st.apps[st.current] = sa

	var found bool
	for _, r := range st.helpRows() {
		if r.key == "ctrl+y" && r.what == "yank the thing" {
			found = true
		}
	}
	if !found {
		t.Error("앱이 낸 키가 도움말에 안 나온다")
	}
}

// 앱이 키를 거두면 도움말에서도 사라진다. 손으로 지울 자리가 없다.
func TestWithdrawnAppKeysLeaveTheHelp(t *testing.T) {
	m, _ := twoAppHost()
	st := state(t, m)
	sa := st.apps[st.current].(stubApp)
	sa.keys = nil
	st.apps[st.current] = sa

	for _, r := range st.helpRows() {
		if r.key == "ctrl+y" {
			t.Error("앱이 안 내는 키가 도움말에 남아 있다")
		}
	}
}

// 꺼진 묶음은 안 낸다. 도움말이 스스로 관리된다는 것이 이 구조의 값이다.
func TestDisabledBindingsAreNotPromised(t *testing.T) {
	m, _ := twoAppHost()
	st := state(t, m)
	sa := st.apps[st.current].(stubApp)
	off := key.NewBinding(key.WithKeys("ctrl+y"), key.WithHelp("ctrl+y", "off"))
	off.SetEnabled(false)
	sa.keys = []key.Binding{off}
	st.apps[st.current] = sa

	for _, r := range st.helpRows() {
		if r.key == "ctrl+y" {
			t.Error("꺼진 키를 약속했다")
		}
	}
}

// 도움말이 적은 키는 실제로 무언가를 해야 한다.
//
// ctrl+o 가 남긴 교훈이다. 호스트가 자기 것이라고 적은 키를 눌렀는데
// handleKey 가 모른 척하면, 그 줄은 거짓말이다.
func TestKeysTheHostPromisesAreHandled(t *testing.T) {
	m, _ := twoAppHost()
	st := state(t, m)

	for _, b := range keys.helpOrder() {
		for _, k := range b.Keys() {
			// `?` 는 입력창이 비었을 때만 뜻이 있다. 그 상태로 본다.
			handled, _, _ := st.handleKey(pressOf(k))
			if !handled {
				t.Errorf("도움말이 약속한 %q 를 호스트가 안 받는다", k)
			}
		}
	}
}

// pressOf 는 "ctrl+f" 같은 이름을 실제 키 이벤트로 되돌린다.
func pressOf(name string) tea.KeyPressMsg {
	switch name {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	}
	if r, ok := strings.CutPrefix(name, "ctrl+"); ok && len(r) == 1 {
		return tea.KeyPressMsg{Code: rune(r[0]), Mod: tea.ModCtrl}
	}
	return tea.KeyPressMsg{Code: rune(name[0]), Text: name}
}
