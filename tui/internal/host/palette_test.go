package host

import (
	"strings"
	"testing"

	"amcli/tui/internal/musicapp"
	tea "charm.land/bubbletea/v2"
)

// 팔레트는 목록이다. 목록이 하는 일은 세로로 훑히는 것이다.

// openPalette 는 `/` 를 친 상태를 만든다.
func openPalette(t *testing.T, w int) tea.Model {
	t.Helper()
	hm := New(musicapp.New())
	hm.LeaveHome()
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: 32})
	// AI 키가 없으면 Search 로 시작한다. 명령은 Ask 모드에서만 열린다.
	if state(t, m).mode != modePrompt {
		m, _ = m.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	if !state(t, m).commanding() {
		t.Fatal("`/` 를 쳤는데 팔레트가 안 열렸다")
	}
	return m
}

// 설명이 이름 바로 뒤에서 시작한다.
//
// 양끝 정렬이던 시절에는 창이 넓을수록 둘이 멀어져, 짝을 맞추려고 눈이
// 화면을 가로질러야 했다. 고정폭이면 설명이 늘 같은 자리에서 시작한다.
func TestDescriptionsStartAtTheSameColumn(t *testing.T) {
	hs := state(t, openPalette(t, 160))
	rows := hs.overlayRows(150)
	if len(rows) < 3 {
		t.Fatalf("줄이 %d개뿐이다", len(rows))
	}

	at := -1
	for i, r := range rows {
		// 글자 수로 센다. 고른 줄의 레일(▌)은 한 칸이지만 세 바이트다.
		p := []rune(plain(r))
		col := descColumn(p)
		if col < 0 {
			continue
		}
		if at == -1 {
			at = col
		} else if col != at {
			t.Errorf("%d번째 줄의 설명이 %d칸에서 시작한다 — 앞줄은 %d칸\n%s", i+1, col, at, string(p))
		}
	}
	// 창이 넓다고 설명이 끝으로 밀려나면 안 된다.
	if at > cmdNameCol+4 {
		t.Errorf("설명이 %d칸에서 시작한다 — 이름 칸(%d) 바로 뒤여야 한다", at, cmdNameCol)
	}
}

// descColumn 은 이름 뒤 설명이 시작하는 칸이다. 없으면 -1.
func descColumn(p []rune) int {
	for i := 2; i+1 < len(p); i++ {
		if p[i] == ' ' && p[i+1] == ' ' {
			for i < len(p) && p[i] == ' ' {
				i++
			}
			if i < len(p) {
				return i
			}
			return -1
		}
	}
	return -1
}

// **↓ 로 끝까지 갈 수 있어야 한다.**
//
// 예전에는 여덟 줄에서 멈추고 "+N more — keep typing" 이라고 했다. 안내는
// 있었지만 눌러 본 사람에게는 멈춘 것으로 보였고, 나머지에 닿는 길이
// 타이핑뿐이었다.
func TestArrowReachesEveryCommand(t *testing.T) {
	m := openPalette(t, 120)
	total := state(t, m).pickCount()
	if total <= maxOverlayRows {
		t.Skipf("명령이 %d개뿐이라 넘길 것이 없다", total)
	}

	for i := 0; i < total+5; i++ {
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if got := state(t, m).pick; got != total-1 {
		t.Errorf("끝까지 내렸는데 %d번째다 — 마지막은 %d번째", got, total-1)
	}
}

// 고른 줄은 언제나 화면 안에 있어야 한다. 창이 따라 내려간다.
func TestSelectedCommandStaysVisible(t *testing.T) {
	m := openPalette(t, 120)
	total := state(t, m).pickCount()

	for i := 0; i < total; i++ {
		hs := state(t, m)
		rows := hs.overlayRows(110)
		if len(rows) > maxOverlayRows {
			t.Fatalf("%d줄을 그렸다 — 최대 %d줄이다", len(rows), maxOverlayRows)
		}
		found := false
		for _, r := range rows {
			if strings.Contains(r, "▌") {
				found = true
			}
		}
		if !found {
			t.Fatalf("%d번째 줄을 골랐는데 화면에 없다", hs.pick)
		}
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
}
