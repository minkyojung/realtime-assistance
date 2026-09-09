// 프롬프트 → Thinking → 큐 까지의 흐름을 TTY 없이 확인한다.
package main

import (
	"fmt"
	"os"
	"strings"

	"amcli/tui/internal/host"
	"amcli/tui/internal/musicapp"
	tea "charm.land/bubbletea/v2"
)

func main() {
	prompt := "요즘 안 듣던 것 위주로 30분"
	if len(os.Args) > 1 {
		prompt = strings.Join(os.Args[1:], " ")
	}

	hm := host.New(musicapp.New())
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 28})
	m, _ = m.Update(musicapp.ProbeStatus())

	for _, r := range prompt {
		m, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	fmt.Println("──────── enter 직후 ────────")
	fmt.Println(m.View().Content)

	// Batch 안의 큐 생성 Cmd 를 찾아 실행한다.
	msg := musicapp.DrainForQueue(cmd)
	if msg == nil {
		fmt.Println("큐 생성 Cmd 를 찾지 못했습니다")
		return
	}
	m, playCmd := m.Update(msg)
	if playCmd != nil {
		playCmd() // 실제로 첫 곡을 튼다
	}

	fmt.Println("\n──────── 큐 도착 ────────")
	fmt.Println(m.View().Content)
}
