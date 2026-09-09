package host

import (
	"strings"
	"testing"

	"amcli/tui/internal/app"
	tea "charm.land/bubbletea/v2"
)

// 포커스 신호 — 화면 앞에 있는 앱만 장치를 잡게 하는 유일한 통로다.
//
// 카메라가 이것을 쓴다. 신호가 새거나 빠지면 안 보는 동안에도 카메라
// 불이 켜져 있게 되므로, 계약 쪽이 아니라 호스트 쪽에서 확인한다.

type focusApp struct {
	name        string
	focus, blur *int
}

func (f focusApp) Name() string               { return f.name }
func (f focusApp) Description() string        { return f.name }
func (f focusApp) Tagline() string            { return f.name }
func (f focusApp) Init(func(tea.Msg)) tea.Cmd { return nil }
func (f focusApp) Ready() error               { return nil }
func (f focusApp) Badge() int                 { return 0 }
func (f focusApp) Status() string             { return f.name }
func (f focusApp) Filter(string) app.App      { return f }
func (f focusApp) Back() (app.App, bool)      { return f, false }
func (f focusApp) Commands() []app.Command    { return nil }
func (f focusApp) Facts() []app.Fact          { return nil }
func (f focusApp) Ask(string) tea.Cmd         { return nil }
func (f focusApp) View(w, h int) string       { return strings.Repeat("\n", h-1) }
func (f focusApp) Update(msg tea.Msg) (app.App, tea.Cmd) {
	switch msg.(type) {
	case app.FocusMsg:
		*f.focus++
	case app.BlurMsg:
		*f.blur++
	}
	return f, nil
}

func TestFocusFollowsTheVisibleApp(t *testing.T) {
	var aF, aB, bF, bB int
	hm := New(
		focusApp{name: "alpha", focus: &aF, blur: &aB},
		focusApp{name: "beta", focus: &bF, blur: &bB},
	)
	hm.LeaveHome()

	// 처음 보이는 앱도 포커스를 받아야 한다.
	hm.Init()
	if aF != 1 || bF != 0 {
		t.Fatalf("시작 직후 포커스: alpha %d, beta %d — 원한 것 1, 0", aF, bF)
	}

	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 28})
	m, _ = m.Update(switchAppMsg{index: 1})

	if aB != 1 {
		t.Errorf("나간 앱이 blur 를 %d번 받았다 — 원한 것 1", aB)
	}
	if bF != 1 {
		t.Errorf("들어온 앱이 focus 를 %d번 받았다 — 원한 것 1", bF)
	}
	if bB != 0 || aF != 1 {
		t.Errorf("엉뚱한 앱에 신호가 갔다: alpha focus %d, beta blur %d", aF, bB)
	}

	// 되돌아오면 반대로.
	m, _ = m.Update(switchAppMsg{index: 0})
	if bB != 1 || aF != 2 {
		t.Errorf("돌아왔을 때: beta blur %d, alpha focus %d — 원한 것 1, 2", bB, aF)
	}
}

// 범위 밖 인덱스로는 아무 신호도 나가지 않는다.
func TestSwitchOutOfRangeSendsNothing(t *testing.T) {
	var aF, aB int
	hm := New(focusApp{name: "alpha", focus: &aF, blur: &aB})
	hm.LeaveHome()
	hm.Init()

	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 28})
	m, _ = m.Update(switchAppMsg{index: 7})
	if aB != 0 {
		t.Errorf("없는 앱으로 가는데 blur 가 %d번 나갔다", aB)
	}
}
