package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// 한글은 표시폭이 2다. 폭 계산이 어긋나면 테두리와 정렬이 전부 깨지므로
// 어느 줄도 화면 폭을 넘지 않는지 자동으로 확인한다.
func TestViewFitsWidth(t *testing.T) {
	for _, w := range []int{60, 80, 96, 120, 200} {
		var m tea.Model = New()
		m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: 40})

		for i, line := range strings.Split(m.View().Content, "\n") {
			if got := lipgloss.Width(line); got > w {
				t.Errorf("터미널 폭 %d: %d번째 줄이 %d칸으로 넘침\n%q", w, i+1, got, line)
			}
		}
	}
}

// 이 서비스가 주장하는 것이 화면에 실제로 있는지 확인한다.
func TestViewShowsReason(t *testing.T) {
	var m tea.Model = New()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 40})
	out := m.View().Content

	for _, want := range []string{
		"한 번도 재생하지 않았습니다", // F2 선정 근거
		"7 never played",       // 상태줄 요약
		"● Playing",            // 헤더 상태
	} {
		if !strings.Contains(out, want) {
			t.Errorf("화면에 %q 가 없다", want)
		}
	}
}
