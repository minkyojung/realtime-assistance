// Apple Music CLI — 터미널 안의 앱 하나.
//
// 호스트가 껍데기를 갖고, 앱이 본문을 그린다. docs/07-호스트-계약.md
package main

import (
	"fmt"
	"os"

	"amcli/tui/internal/host"
	"amcli/tui/internal/musicapp"
	tea "charm.land/bubbletea/v2"
)

func main() {
	// 앱은 컴파일 타임에 등록한다. 플러그인 시스템을 두지 않는 이유는
	// 확장성을 여는 순간 Zellij 의 무게가 따라오기 때문이다 — docs/07 6절.
	m := host.New(musicapp.New())
	p := tea.NewProgram(&m)
	// 앱이 이벤트 루프 밖에서 메시지를 넣을 통로를 준다.
	m.SetSend(func(msg tea.Msg) { p.Send(msg) })

	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "실행 실패:", err)
		os.Exit(1)
	}
}
