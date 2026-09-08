// 헤드리스 렌더 — TTY 없이 화면을 찍어 본다. 레이아웃 확인용.
package main

import (
	"fmt"
	"os"
	"strconv"

	"amcli/tui/internal/ui"
	tea "charm.land/bubbletea/v2"
)

func main() {
	w := 96
	if len(os.Args) > 1 {
		if n, err := strconv.Atoi(os.Args[1]); err == nil {
			w = n
		}
	}
	var m tea.Model = ui.New()
	m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: 40})
	fmt.Println(m.View().Content)
}
