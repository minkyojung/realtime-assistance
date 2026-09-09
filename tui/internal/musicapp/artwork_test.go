package musicapp

import (
	"image"
	"image/color"
	"regexp"
	"strings"
	"testing"
	"time"

	"amcli/tui/internal/api"
	"amcli/tui/internal/data"
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

// ── 가사 ────────────────────────────────────────────────────────────

func synced() *lyrics.Lyrics {
	return &lyrics.Lyrics{Lines: lyrics.ParseLRC(
		"[00:10.00] one\n[00:20.00] two\n[00:30.00] three\n[00:40.00] four\n[00:50.00] five\n")}
}

// 지금 줄 하나만 밝고, 그 줄이 창의 가운데에 온다.
func TestLyricsHighlightsCurrentLine(t *testing.T) {
	m := playingModel()
	m.lyrics, m.lyricsPID = synced(), "ABC"
	m.positionMs, m.polledAt = 35_000, time.Now()

	rows := m.viewLyrics(40, 5)
	if len(rows) != 5 {
		t.Fatalf("줄 수 %d", len(rows))
	}
	var marked []int
	for i, r := range rows {
		if strings.Contains(r, "▸") {
			marked = append(marked, i)
		}
	}
	if len(marked) != 1 {
		t.Fatalf("표시된 줄이 %d개다 — 하나여야 한다", len(marked))
	}
	if marked[0] != 2 {
		t.Errorf("표시된 줄이 %d번째다 — 가운데(2)여야 한다", marked[0])
	}
	if !strings.Contains(rows[2], "three") {
		t.Errorf("35초에 짚은 줄이 %q 다 — three 여야 한다", plain(rows[2]))
	}
}

// 폴링 사이를 메운다. 안 그러면 가사가 최대 1초 늦게 넘어간다.
func TestPositionInterpolates(t *testing.T) {
	m := playingModel()
	m.positionMs = 10_000
	m.polledAt = time.Now().Add(-800 * time.Millisecond)
	if got := m.nowMs(); got < 10_700 || got > 10_900 {
		t.Errorf("보간한 위치 %d — 10800 근처여야 한다", got)
	}
	// 멈춰 있으면 흐르지 않는다.
	m.playing = false
	if got := m.nowMs(); got != 10_000 {
		t.Errorf("멈췄는데 위치가 %d 로 흘렀다", got)
	}
}

// 시각이 없는 가사는 흐르게 할 근거가 없다. 그냥 보여준다.
func TestPlainLyricsDoNotScroll(t *testing.T) {
	m := playingModel()
	m.lyrics = &lyrics.Lyrics{Plain: []string{"alpha", "beta"}}
	rows := m.viewLyrics(40, 4)
	if !strings.Contains(rows[0], "alpha") || !strings.Contains(rows[1], "beta") {
		t.Errorf("줄글이 안 보인다: %q", rows)
	}
	for _, r := range rows {
		if strings.Contains(r, "▸") {
			t.Error("시각이 없는데 줄을 짚었다")
		}
	}
}

// 가사가 없으면 그 자리를 곡 이력이 쓴다. 비워 두면 고장으로 보인다.
func TestNoLyricsShowsFacts(t *testing.T) {
	m := playingModel()
	l := data.Lib()
	if len(l.Tracks) == 0 {
		t.Skip("라이브러리가 비었다")
	}
	// 한 번도 안 튼 곡을 찾아 세운다 — 이 서비스가 주장하는 문제다.
	for _, tr := range l.Tracks {
		if tr.LastPlayedAt == nil {
			m.queue = []api.QueueItem{{Track: tr}}
			m.nowPlayingID = tr.Id
			break
		}
	}
	out := strings.Join(m.viewLyrics(40, 6), "\n")
	if !strings.Contains(out, "never played") {
		t.Errorf("한 번도 안 튼 곡인데 그 말이 없다:\n%s", plain(out))
	}
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plain(s string) string { return ansi.ReplaceAllString(s, "") }
