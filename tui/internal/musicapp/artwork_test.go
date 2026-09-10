package musicapp

import (
	"image"
	"image/color"
	"regexp"
	"strings"
	"testing"

	"amcli/tui/internal/lyrics"
	"amcli/tui/internal/music"
	"charm.land/lipgloss/v2"
)

// 커버 자리에 아무 그림이나 세운다. 색이 아니라 모양을 검사한다.
func swatch() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x * 30), G: uint8(y * 30), B: 90, A: 255})
		}
	}
	return img
}

func playingModel() Model {
	m := New()
	m.polled = true
	m.playing = true
	m.positionMs = 60_000
	m.live = music.PlayerState{
		Playing: true, PersistentID: "ABC", Title: "Sitting, Waiting, Wishing",
		Artist: "Jack Johnson", PositionMs: 60_000, DurationMs: 184_000,
	}
	m.art, m.artPID = swatch(), "ABC"
	return m
}

// 커버 덩어리는 정해진 줄 수를 쓰고, 어느 줄도 폭을 넘지 않는다.
func TestNowPlayingPanelShape(t *testing.T) {
	m := playingModel()
	out, ok := m.viewNowPlaying(100, 30)
	if !ok {
		t.Fatal("커버를 그릴 수 있는 크기인데 안 그렸다")
	}
	lines := strings.Split(out, "\n")
	if len(lines) != 16 {
		t.Errorf("줄 수 %d, 기대 16", len(lines))
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > 100 {
			t.Errorf("%d번째 줄이 폭을 넘었다: %d", i, w)
		}
	}
	if !strings.Contains(out, "Sitting, Waiting, Wishing") {
		t.Error("제목이 없다")
	}
	if !strings.Contains(out, "1:00 / 3:04") {
		t.Error("재생 위치가 없다")
	}
}

// 좁아지거나 낮아지면 한 단계씩 물러나고, 끝에는 접는다.
func TestArtSizeSteps(t *testing.T) {
	for _, c := range []struct{ w, h, cols int }{
		{100, 30, 32}, // 넉넉하면 제일 큰 것
		{100, 22, 24}, // 낮아지면 한 단계
		{60, 30, 16},  // 좁으면 작은 것
		{100, 16, 16}, // 낮으면 작은 것
		{100, 14, 0},  // 더 낮으면 접는다 — 목록에 다섯 줄은 남겨야 한다
		{40, 30, 0},   // 더 좁아도 접는다
	} {
		if cols, _ := artSize(c.w, c.h); cols != c.cols {
			t.Errorf("%dx%d → %d칸, 기대 %d칸", c.w, c.h, cols, c.cols)
		}
	}
}

// 커버가 없는 곡은 조용히 한 줄짜리 재생 바로 돌아간다.
func TestNoArtworkFallsBack(t *testing.T) {
	m := playingModel()
	m.art = nil
	if _, ok := m.viewNowPlaying(100, 30); ok {
		t.Error("커버가 없는데 덩어리를 그렸다")
	}
	if out := m.View(100, 30); !strings.Contains(out, "Sitting, Waiting, Wishing") {
		t.Error("물러난 화면에도 곡 제목은 있어야 한다")
	}
}

// 관문이 막혀 있으면 커버보다 그 말이 먼저다.
func TestGateBeatsArtwork(t *testing.T) {
	m := playingModel()
	m.playerErr = music.ErrPermissionDenied
	if _, ok := m.viewNowPlaying(100, 30); ok {
		t.Error("권한이 막혔는데 커버를 그렸다")
	}
}

// 커버가 있든 없든 본문은 받은 높이를 정확히 채운다.
//
// 커버가 자리를 먹으면 목록이 그만큼 줄고, 커버가 없으면 목록이 그 자리까지
// 먹는다. 어느 쪽이든 **빈 줄이 남으면 안 된다** — 목록에 열네 줄 상한이
// 있던 시절에는 그 아래가 스무 줄씩 비었다.
func TestBodyFillsExactly(t *testing.T) {
	withArt := playingModel()
	bare := playingModel()
	bare.art = nil

	for _, h := range []int{20, 30, 42, 60} {
		for name, m := range map[string]Model{"커버 있음": withArt, "커버 없음": bare} {
			if n := strings.Count(m.View(78, h), "\n") + 1; n != h {
				t.Errorf("%s h=%d → %d줄 (%d줄 어긋남)", name, h, n, h-n)
			}
		}
	}
	// 커버가 있으면 목록이 그만큼 짧아야 한다.
	long := strings.Count(bare.viewList(78, bare.listHeight(30)), "\n")
	short := strings.Count(withArt.viewList(78, withArt.listHeight(30-15)), "\n")
	if short >= long {
		t.Error("커버가 자리를 먹었는데 목록이 안 줄었다")
	}
}

// 커버가 좁아지면 앨범명과 빈 줄을 먼저 버리고 가사를 지킨다.
func TestLyricsSurviveNarrowPanel(t *testing.T) {
	base := playingModel()
	base.lyrics = synced()
	for _, c := range []struct {
		name string
		w, h int
	}{
		{"넓음", 100, 30},
		{"낮음", 100, 22},
		{"좁음", 60, 20},
	} {
		m := base
		out, ok := m.viewNowPlaying(c.w, c.h)
		if !ok {
			t.Errorf("%s (%dx%d) — 덩어리를 못 그렸다", c.name, c.w, c.h)
			continue
		}
		if !strings.Contains(plain(out), "three") {
			t.Errorf("%s (%dx%d) — 가사가 사라졌다", c.name, c.w, c.h)
		}
	}
}

// ── 테스트 거들 ──────────────────────────────────────────────────────

func synced() *lyrics.Lyrics {
	return &lyrics.Lyrics{Lines: lyrics.ParseLRC(
		"[00:10.00] one\n[00:20.00] two\n[00:30.00] three\n[00:40.00] four\n[00:50.00] five\n")}
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plain(s string) string { return ansi.ReplaceAllString(s, "") }
