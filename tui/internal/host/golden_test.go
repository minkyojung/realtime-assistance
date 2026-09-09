package host

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"amcli/tui/internal/musicapp"
	tea "charm.land/bubbletea/v2"
)

// 화면을 통째로 박아둔다. 리팩터링으로 한 바이트라도 달라지면 여기서 걸린다.
//
// 골든을 다시 뜨려면: go run ./cmd/golden
//
// **아래 시나리오 목록은 cmd/golden/main.go 와 같아야 한다.** 한쪽만 고치면
// 골든을 다시 떠도 테스트가 계속 깨진다. 합치지 못하는 이유는 이 파일이
// 테스트 전용이고, 저쪽은 main 이라 서로를 부를 수 없기 때문이다 —
// 호스트가 musicapp 을 알면 안 되므로 host 패키지로 끌어올릴 수도 없다.
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
		{"home", ""},
		{"default", ""},
		{"search", "\x06oasis"},
		{"commands", "/"},
		{"help", "?"},
		{"prompt", "something quiet"},
		// 묶음 파고들기 — tab 으로 Artists 에 가서 enter.
		{"drill", "\t\n"},
	} {
		hm := New(musicapp.New())
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
	return b.String()
}
