package host

import (
	"strings"
	"testing"

	"amcli/tui/internal/app"
	tea "charm.land/bubbletea/v2"
)

// 비밀은 화면에 안 남는다.
//
// 시연이나 화면 공유에서 한 번 찍히면 지워지지 않는다. 그래서 "저장은
// 키체인에" 만큼이나 "치는 동안 안 보인다"가 값이다.

const fakeKey = "sk-proj-ZZTOPSECRETZZ0123456789"

// `/ai` 만 치고 엔터하면 가려진 자리가 열리고, 키를 쳐도 화면에 안 나온다.
func TestSecretIsNeverDrawn(t *testing.T) {
	m := typeSecret(t, fakeKey)

	if v := m.(Model).input.Value(); v != fakeKey {
		t.Fatalf("모델이 진짜 값을 안 들고 있다: %q", v)
	}
	screen := plain(m.View().Content)
	if strings.Contains(screen, fakeKey) {
		t.Error("키가 화면에 그대로 있다")
	}
	// 조각으로도 새면 안 된다. 앞머리만 봐도 어떤 키인지 알아본다.
	if strings.Contains(screen, "ZZTOPSECRET") {
		t.Error("키의 일부가 화면에 있다")
	}
	if !strings.Contains(screen, secretDot) {
		t.Error("가림 문자가 하나도 안 보인다 — 친 것이 들어갔는지 알 길이 없다")
	}
}

// 엔터를 치면 그 명령에게 진짜 값이 간다. 가리는 것은 화면뿐이다.
func TestSecretReachesTheCommand(t *testing.T) {
	var got string
	m := typeSecretInto(t, fakeKey, func(arg string) tea.Cmd {
		got = arg
		return nil
	})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if got != fakeKey {
		t.Errorf("명령이 받은 것: %q", got)
	}
	if v := m.(Model).input.Value(); v != "" {
		t.Errorf("넣고 나서 입력창에 남았다: %q", v)
	}
	if m.(Model).asking() {
		t.Error("넣었는데 아직 묻고 있다")
	}
}

// esc 로 무르면 친 것이 사라진다. 반쯤 친 키가 입력창에 남아 있으면
// 다음에 엔터를 칠 때 그것이 문장으로 나간다.
func TestSecretIsWipedOnCancel(t *testing.T) {
	m := typeSecret(t, fakeKey)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if v := m.(Model).input.Value(); v != "" {
		t.Errorf("무른 뒤에도 남았다: %q", v)
	}
	if m.(Model).asking() {
		t.Error("무른 뒤에도 묻고 있다")
	}
}

// 붙여넣기도 같은 길이다. API 키는 손으로 치는 것이 아니다.
func TestSecretAcceptsPaste(t *testing.T) {
	var got string
	m := openSecret(t, func(arg string) tea.Cmd { got = arg; return nil })
	m, _ = m.Update(tea.PasteMsg{Content: fakeKey})

	if strings.Contains(plain(m.View().Content), fakeKey) {
		t.Error("붙여넣은 키가 화면에 그대로 있다")
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if got != fakeKey {
		t.Errorf("붙여넣은 것이 명령까지 안 갔다: %q", got)
	}
}

// 인자를 이어 친 사람은 그대로 간다. 아는 사람의 길을 막지 않는다.
func TestSecretWithInlineArgStillRuns(t *testing.T) {
	var got string
	hm := newSecretHost(func(arg string) tea.Cmd { got = arg; return nil })
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 32})
	for _, r := range "/secret " + fakeKey {
		m, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		cmd()
	}
	if got != fakeKey {
		t.Errorf("이어 친 인자가 안 갔다: %q", got)
	}
	if m.(Model).asking() {
		t.Error("이어 쳤는데 또 물어본다")
	}
}

// ── 도구 ────────────────────────────────────────────────────────────

func typeSecret(t *testing.T, key string) tea.Model {
	t.Helper()
	return typeSecretInto(t, key, func(string) tea.Cmd { return nil })
}

func typeSecretInto(t *testing.T, key string, run func(string) tea.Cmd) tea.Model {
	t.Helper()
	m := openSecret(t, run)
	for _, r := range key {
		m, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	return m
}

// openSecret 은 `/secret` 을 치고 엔터한 자리까지 데려간다.
func openSecret(t *testing.T, run func(string) tea.Cmd) tea.Model {
	t.Helper()
	hm := newSecretHost(run)
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 32})
	for _, r := range "/secret" {
		m, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		cmd()
	}
	if !m.(Model).asking() {
		t.Fatal("엔터를 쳤는데 가려진 자리가 안 열렸다")
	}
	return m
}

func newSecretHost(run func(string) tea.Cmd) Model {
	hm := New(secretApp{stubApp: stubApp{name: "alpha"}, run: run})
	hm.LeaveHome()
	return hm
}

// 비밀 인자를 받는 명령 하나만 갖는 앱.
type secretApp struct {
	stubApp
	run func(string) tea.Cmd
}

func (s secretApp) Commands() []app.Command {
	return []app.Command{{
		Name: "/secret", Arg: "<key>", Help: "hand over a secret",
		Run: s.run, Secret: true,
	}}
}

// stubApp.Update 는 안쪽 타입을 돌려주므로 막지 않으면 첫 Update 에
// 바깥 타입이 날아간다 — 그러면 /secret 이 사라진다.
func (s secretApp) Update(msg tea.Msg) (app.App, tea.Cmd) {
	_, cmd := s.stubApp.Update(msg)
	return s, cmd
}
