// Apple Music CLI — 터미널 클라이언트
//
// 지금은 화면만 있다. 서버·Music.app 연동 없이 더미 데이터로 렌더한다.
// 목적은 산출물 ①(PDF)에 넣을 와이어프레임 캡처를 뽑는 것이다.
package main

import (
	"fmt"
	"os"

	"amcli/tui/internal/ui"
	tea "charm.land/bubbletea/v2"
)

func main() {
	if _, err := tea.NewProgram(ui.New()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "실행 실패:", err)
		os.Exit(1)
	}
}
