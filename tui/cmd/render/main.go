// 헤드리스 렌더 — TTY 없이 화면을 찍어 본다. 레이아웃 확인용.
package main

import (
	"fmt"
	"os"
	"strconv"

	"amcli/tui/internal/host"
	"amcli/tui/internal/musicapp"
	tea "charm.land/bubbletea/v2"
)

func main() {
	w := 96
	if len(os.Args) > 1 {
		if n, err := strconv.Atoi(os.Args[1]); err == nil {
			w = n
		}
	}
	hm := host.New(musicapp.New())
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: 26})
	// 실제 Music.app 상태를 한 번 읽어 반영한다.
	if cmd := m.Init(); cmd != nil {
		if msg := cmd(); msg != nil {
			m, _ = m.Update(msg)
		}
	}
	fmt.Println(m.View().Content)
}
