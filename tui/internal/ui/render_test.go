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

func typeText(m tea.Model, s string) tea.Model {
	for _, r := range s {
		m, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	return m
}

// 입력창은 늘 활성이어야 한다. 이게 깨지면 아무것도 칠 수 없다.
// 그리고 프롬프트 모드에서는 목록을 건드리지 않아야 한다.
func TestPromptDoesNotFilterList(t *testing.T) {
	var m tea.Model = New()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 32})
	m = typeText(m, "something quiet")

	out := m.View().Content
	if !strings.Contains(out, "something quiet") {
		t.Fatal("친 글자가 입력창에 없다 — 입력창이 활성이 아니다")
	}
	if !strings.Contains(out, "Peanut butter Sandwich") {
		t.Error("프롬프트를 치는 중인데 목록이 걸러졌다")
	}
}

// ctrl+f 로 들어간 검색 모드에서만 목록이 걸러져야 한다.
func TestSearchModeFiltersList(t *testing.T) {
	var m tea.Model = New()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 32})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	m = typeText(m, "oasis")

	out := m.View().Content
	if !strings.Contains(out, "⌕ Search") {
		t.Error("검색 중이라는 표시가 없다")
	}
	// 재생 바에 뜨는 곡은 목록과 무관하게 남으므로, 재생 중이 아닌 곡으로 확인한다.
	if strings.Contains(out, "Peanut butter Sandwich") {
		t.Error("검색어를 쳤는데 목록이 걸러지지 않았다")
	}
	if !strings.Contains(out, "Oasis") {
		t.Error("검색 결과에 Oasis 곡이 없다")
	}
}
