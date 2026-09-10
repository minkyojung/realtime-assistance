package host

import (
	"strings"
	"testing"

	"amcli/tui/internal/musicapp"
	tea "charm.land/bubbletea/v2"
)

// 붙여넣기는 입력창의 것이다.
//
// 손으로 못 치는 것이 이 길로 온다 — API 키가 그렇다. 한때 호스트가
// tea.KeyPressMsg 만 보고 붙여넣기는 앱으로 흘려보내서, `/ai ` 까지 치고
// 키를 붙여넣으면 아무 일도 안 일어났다. 켜는 길이 그것 하나뿐인데.
func TestPasteReachesTheInput(t *testing.T) {
	const key = "sk-proj-0123456789abcdefghijklmnopqrstuvwxyz0123456789"

	hm := New(musicapp.New())
	hm.LeaveHome()
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 32})
	for _, r := range "/ai " {
		m, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	m, _ = m.Update(tea.PasteMsg{Content: key})

	got := m.(Model).input.Value()
	if !strings.Contains(got, key) {
		t.Errorf("붙여넣은 것이 입력창에 없다: %q", got)
	}
}

// 여러 줄짜리를 붙여도 실행되지 않는다. 붙여넣기는 글자이지 엔터가 아니다.
func TestPasteDoesNotRun(t *testing.T) {
	hm := New(musicapp.New())
	hm.LeaveHome()
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 32})
	m, _ = m.Update(tea.PasteMsg{Content: "hello\nworld"})

	if v := m.(Model).input.Value(); v == "" {
		t.Error("붙여넣기가 통째로 사라졌다")
	}
}
