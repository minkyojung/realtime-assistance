package musicapp

import (
	"image"
	"image/color"
	"strings"
	"testing"

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
	if len(lines) != 12 {
		t.Errorf("줄 수 %d, 기대 12", len(lines))
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
		{100, 30, 24}, // 넉넉하면 큰 것
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

// 커버가 자리를 먹은 만큼 목록이 줄어야 한다. 안 그러면 화면 밖으로 넘친다.
func TestListShrinksUnderArtwork(t *testing.T) {
	m := playingModel()
	if n := strings.Count(m.View(100, 30), "\n") + 1; n > 30 {
		t.Errorf("본문이 %d줄로 받은 높이(30)를 넘었다", n)
	}
	small := playingModel()
	small.art = nil
	big := strings.Count(m.View(100, 30), "\n")
	one := strings.Count(small.View(100, 30), "\n")
	if big <= one {
		t.Error("커버를 그렸는데 화면이 안 커졌다")
	}
}
