package musicapp

import (
	"amcli/tui/internal/style"
	"fmt"
	"strings"

	"amcli/tui/internal/api"
	"amcli/tui/internal/data"
	"charm.land/lipgloss/v2"
)

// 가운데 목록 패널. 사이드바가 무엇을 고르든 이 패널 하나가 다 그린다.
// 그래서 화면이 늘어나지 않는다.

// style.Row 한 줄이 곡일 수도, 묶음(아티스트·앨범)일 수도 있다.
type listRow struct {
	track *api.Track
	group *data.Group
}

func (m Model) rows() []listRow {
	l := data.Lib()

	if m.searching() {
		return trackRows(l.Search(m.filter))
	}

	s := m.sections[m.sectionIdx]
	switch s.kind {
	case secRecent:
		return trackRows(l.RecentlyAdded())
	case secSongs:
		return trackRows(l.Songs())
	case secArtists:
		return groupRows(l.Artists())
	case secAlbums:
		return groupRows(l.Albums())
	case secPlaylist:
		if s.playlist != nil {
			return trackRows(l.PlaylistTracks(*s.playlist))
		}
	case secQueue:
		return trackRows(m.queueTracks())
	case secUnplayed:
		return unplayedTracks(l)
	}
	return nil
}

func trackRows(ts []api.Track) []listRow {
	out := make([]listRow, len(ts))
	for i := range ts {
		out[i] = listRow{track: &ts[i]}
	}
	return out
}

func groupRows(gs []data.Group) []listRow {
	out := make([]listRow, len(gs))
	for i := range gs {
		out[i] = listRow{group: &gs[i]}
	}
	return out
}

func (m Model) rowCount() int { return len(m.rows()) }

// viewList 는 세로 스크롤을 직접 계산한다. 선택 행이 늘 창 안에 있게 한다.
func (m Model) viewList(w, h int) string {
	rows := m.rows()
	if len(rows) == 0 {
		msg := "Nothing here yet"
		if m.searching() {
			msg = "No matches"
		}
		return lipgloss.NewStyle().Width(w).Height(h).Render(style.Faint.Render(msg))
	}

	start := m.listTop
	if start > len(rows)-h {
		start = len(rows) - h
	}
	if start < 0 {
		start = 0
	}
	end := start + h
	if end > len(rows) {
		end = len(rows)
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		b.WriteString(m.renderRow(rows[i], i == m.listIdx, w))
		if i < end-1 {
			b.WriteString("\n")
		}
	}
	out := b.String()
	for n := end - start; n < h; n++ {
		out += "\n"
	}
	return out
}

func (m Model) renderRow(r listRow, selected bool, w int) string {
	// 선택 표시는 왼쪽 레일이다. 배경을 채우면 채워진 빨강 = 오류 규칙과 부딪힌다.
	rail := "  "
	if selected {
		rail = style.Brand.Render("▌ ")
	}
	inner := w - 2

	if g := r.group; g != nil {
		left := style.Body.Render(style.Truncate(g.Name, inner-24))
		right := style.Faint.Render(fmt.Sprintf("%d tracks · %s", g.TrackCount, g.Subtitle))
		return rail + style.Row(left, right, inner)
	}

	t := r.track
	nowPlaying := m.nowPlayingID != 0 && t.Id == m.nowPlayingID

	title := style.Truncate(t.Title, style.Max(inner-36, 12))
	if nowPlaying {
		title = style.BrandBold.Render(title)
	} else if t.PlayCount == 0 {
		// 한 번도 안 들은 곡은 흐리게. 색이 없다는 게 곧 침묵이다.
		title = style.Dim.Render(title)
	} else {
		title = style.Body.Render(title)
	}

	artist := style.Faint.Render(style.Truncate(t.Artist.Name, 22))
	dur := style.Faint.Render(style.MMSS(t.DurationMs))

	playCol := style.Faint.Render(fmt.Sprintf("%3d", t.PlayCount))
	if t.PlayCount == 0 {
		playCol = style.Faint.Render("  ·")
	}

	left := title
	right := artist + "   " + playCol + "  " + dur
	return rail + style.Row(left, right, inner)
}
