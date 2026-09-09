package musicapp

import (
	"fmt"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// 화면을 통째로 박아둔다. 리팩터링으로 한 바이트라도 달라지면 여기서 걸린다.
//
// 골든을 다시 뜨려면: go run ./cmd/golden
func TestGolden(t *testing.T) {
	want, err := os.ReadFile("testdata/golden.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got := renderAll(); got != string(want) {
		t.Errorf("화면이 달라졌다.\n--- got ---\n%s", got)
	}
}

func renderAll() string {
	var b strings.Builder
	for _, c := range []struct{ name, keys string }{
		{"default", ""},
		{"search", "\x06oasis"},
		{"commands", "/"},
		{"help", "?"},
		{"prompt", "something quiet"},
	} {
		var m tea.Model = New()
		m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 28})
		for _, r := range c.keys {
			if r == '\x06' {
				m, _ = m.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
				continue
			}
			m, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		}
		fmt.Fprintf(&b, "=== %s ===\n%s\n", c.name, m.View().Content)
	}
	return b.String()
}
