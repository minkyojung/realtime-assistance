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
	for _, w := range []int{70, 80, 96, 120, 200} {
		var m tea.Model = New()
		m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: 32})

		for i, line := range strings.Split(m.View().Content, "\n") {
			if got := lipgloss.Width(line); got > w {
				t.Errorf("터미널 폭 %d: %d번째 줄이 %d칸으로 넘침\n%q", w, i+1, got, line)
			}
		}
	}
}

// 기본 화면에 라이브러리와 재생 바가 실제로 그려지는지.
func TestViewShowsLibraryAndPlayer(t *testing.T) {
	var m tea.Model = New()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 32})
	out := m.View().Content

	for _, want := range []string{
		"LIBRARY",   // 사이드바
		"Songs",     // 라이브러리 섹션
		"PLAYLISTS", // 플레이리스트 섹션
		"Perth",     // 재생 바의 현재 곡
		"Bon Iver",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("화면에 %q 가 없다", want)
		}
	}
}

// tab 으로 섹션을 옮기면 목록이 실제로 바뀌는지.
func TestTabSwitchesSection(t *testing.T) {
	var m tea.Model = New()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 32})
	first := m.View().Content

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.View().Content == first {
		t.Error("tab 을 눌렀는데 목록이 그대로다")
	}
}

// 글자를 치면 입력창에 들어가고 목록이 걸러지는지.
// 입력창이 늘 활성이어야 한다 — 이게 깨지면 아무것도 칠 수 없다.
func TestTypingFiltersList(t *testing.T) {
	var m tea.Model = New()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 32})

	for _, r := range "oasis" {
		m, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	out := m.View().Content
	if !strings.Contains(out, "oasis") {
		t.Fatal("친 글자가 입력창에 없다 — 입력창이 활성이 아니다")
	}
	if strings.Contains(out, "Please Wait for Me") {
		t.Error("검색어를 쳤는데 목록이 걸러지지 않았다")
	}
}
