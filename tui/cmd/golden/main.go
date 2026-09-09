package main

import (
	"fmt"
	"os"
	"strings"

	"amcli/tui/internal/data"
	"amcli/tui/internal/data/fixture"
	"amcli/tui/internal/host"
	"amcli/tui/internal/musicapp"
	tea "charm.land/bubbletea/v2"
)

// 리팩터링 전후를 비교할 골든 파일을 만든다.
//
// **아래 시나리오 목록은 internal/host/golden_test.go 와 같아야 한다.**
// 실제 Music.app 상태에 의존하지 않도록 폴링 결과는 넣지 않는다.
func main() {
	// 골든은 기계·시각과 무관해야 한다. 픽스처를 고정으로 심는다.
	data.Set(fixture.Lib())

	var b strings.Builder
	for _, c := range []struct {
		name string
		keys string
	}{
		{"home", ""},
		{"default", ""},
		{"search", "\x06oasis"}, // ctrl+f + oasis
		{"commands", "/"},
		{"help", "?"},
		{"prompt", "something quiet"},
		// 묶음 파고들기 — tab 으로 Artists 에 가서 enter.
		{"drill", "\t\n"},
	} {
		hm := host.New(musicapp.New())
		if c.name != "home" {
			hm.LeaveHome()
		}
		var m tea.Model = &hm
		m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 28})
		for _, r := range c.keys {
			switch r {
			case '\x06':
				m, _ = m.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
			case '\t':
				m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
			case '\n':
				m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			default:
				m, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
			}
		}
		fmt.Fprintf(&b, "=== %s ===\n%s\n", c.name, m.View().Content)
	}
	os.WriteFile("internal/host/testdata/golden.txt", []byte(b.String()), 0o644)
	fmt.Println("wrote", len(b.String()), "bytes")
}
