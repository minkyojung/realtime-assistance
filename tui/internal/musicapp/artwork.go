package musicapp

import (
	"context"
	"image"

	"amcli/tui/internal/music"
	"amcli/tui/internal/style"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// 지금 듣고 있는 것이 화면 위쪽의 주인공이다.
//
// 앨범 커버 · 제목 · 아티스트 · 진행바가 한 덩어리로 위에 앉고, 목록은
// 그 아래로 내려간다. 목록은 "고르는 곳"이지 "보는 곳"이 아니다 —
// 재생 중에 눈이 가는 곳은 지금 나오는 곡이다.

// 커버는 곡이 바뀔 때만 읽는다. 폴링(1초)마다 AppleScript 를 부르면
// Music.app 이 그만큼 일한다. 곡 하나에 한 번이면 충분하다.
type artMsg struct {
	pid string // 어느 곡의 커버인가. 곡이 이미 바뀌었으면 버린다
	img image.Image
	err error
}

func cmdArtwork(pid string) tea.Cmd {
	return func() tea.Msg {
		b, err := music.Artwork(context.Background())
		if err != nil {
			return artMsg{pid: pid, err: err}
		}
		img, err := style.DecodeArtwork(b)
		return artMsg{pid: pid, img: img, err: err}
	}
}

// 커버 크기. 셀이 가로1 세로2 비율이므로 줄 수를 칸 수의 절반으로 두면
// 화면에서 정사각형이다.
//
// 좁아지거나 낮아지면 한 단계씩 물러난다. 마지막은 커버를 아예 접고
// 한 줄짜리 재생 바로 돌아가는 것이다 — 목록이 세 줄도 안 남으면
// 커버가 아무리 예뻐도 쓸 수 없는 화면이다.
func artSize(w, h int) (cols, rows int) {
	// 위아래 룰 둘과 목록 최소 다섯 줄은 커버보다 먼저다.
	room := h - 2 - 5
	switch {
	case w >= 88 && room >= 16:
		return 32, 16
	case w >= 72 && room >= 12:
		return 24, 12
	case w >= 56 && room >= 8:
		return 16, 8
	}
	return 0, 0
}

// viewNowPlaying — 커버와 재생 정보를 한 덩어리로 그린다.
//
// 못 그릴 사정이 하나라도 있으면 false 를 돌려주고, 부르는 쪽이
// 한 줄짜리 재생 바로 돌아간다. 커버가 없는 곡도 그중 하나다.
func (m Model) viewNowPlaying(w, h int) (string, bool) {
	cols, rows := artSize(w, h)
	if cols == 0 || m.art == nil {
		return "", false
	}
	if _, gated := m.viewGate(w); gated {
		return "", false // 관문이 막혀 있으면 그 말이 먼저다
	}
	title, artist, album, durationMs := m.nowPlayingMeta()
	if title == "" {
		return "", false
	}

	const gap = 3
	infoW := w - cols - gap
	if infoW < 24 {
		return "", false
	}

	art := style.Artwork(m.art, cols, rows)
	info := m.nowPlayingInfo(infoW, rows, title, artist, album, durationMs)

	lines := make([]string, rows)
	for i := 0; i < rows; i++ {
		lines[i] = art[i] + spaces(gap) + info[i]
	}
	return joinLines(lines), true
}

// 정보 칸. 커버와 같은 줄 수로 맞춰 돌려준다 — 짧으면 빈 줄로 채운다.
func (m Model) nowPlayingInfo(w, rows int, title, artist, album string, durationMs int) []string {
	icon := style.Brand.Render("▶")
	if !m.playing {
		icon = style.Faint.Render("❚❚")
	}

	pos := style.Clamp(m.nowMs(), 0, durationMs)
	timeLabel := style.MMSS(pos) + " / " + style.MMSS(durationMs)
	ratio := 0.0
	if durationMs > 0 {
		ratio = float64(pos) / float64(durationMs)
	}
	barW := w - lipgloss.Width(timeLabel) - 5
	bar := icon + "  " + style.Faint.Render(timeLabel)
	if barW >= 8 {
		bar = icon + "  " + style.Progress(barW, ratio) + " " + style.Faint.Render(timeLabel)
	}

	out := []string{
		style.Track.Render(style.Truncate(title, w)),
		style.Meta.Render(style.Truncate(artist, w)),
	}
	if album != "" {
		out = append(out, style.Faint.Render(style.Truncate(album, w)))
	}
	out = append(out, "", bar)

	// 남는 자리는 가사가 쓴다. 가사가 없으면 곡 이력이 쓴다 — lyrics.go
	//
	// 두 줄은 띄운다. 진행바에 가사가 붙으면 한 덩어리로 읽혀서, 어디까지가
	// 재생 정보이고 어디부터가 노래인지 눈이 구별하지 못한다.
	const breathe = 1
	if room := rows - len(out) - breathe; room >= 2 {
		for i := 0; i < breathe; i++ {
			out = append(out, "")
		}
		out = append(out, m.viewLyrics(w, room)...)
	}

	for len(out) < rows {
		out = append(out, "")
	}
	return out[:rows]
}

// 지금 나오는 곡의 정보. Music.app 이 말해주는 것이 먼저다 —
// 라이브러리에 없는 곡(카탈로그 스트리밍)도 그 경로로 들어온다.
func (m Model) nowPlayingMeta() (title, artist, album string, durationMs int) {
	title, artist, durationMs = m.live.Title, m.live.Artist, m.live.DurationMs
	if t, ok := m.nowPlaying(); ok {
		album = t.Album.Title
		if title == "" {
			title, artist, durationMs = t.Title, t.Artist.Name, t.DurationMs
		}
	}
	return title, artist, album, durationMs
}

func spaces(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}
