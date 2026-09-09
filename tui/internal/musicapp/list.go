package musicapp

import (
	"fmt"
	"strings"

	"amcli/tui/internal/api"
	"amcli/tui/internal/data"
	"amcli/tui/internal/style"
	"charm.land/lipgloss/v2"
)

// 목록 패널. 사이드바가 없으므로 본문 전체를 쓴다.
//
// 지금 어디를 보고 있는지는 상태줄이 말한다. 갈 곳은 `/` 가 보여준다.
// 화면에 상시로 자리를 주지 않는다 — docs/07-호스트-계약.md

// listRow 한 줄은 곡일 수도, 묶음(아티스트·앨범)일 수도 있다.
type listRow struct {
	track *api.Track
	group *data.Group
}

func (m Model) rows() []listRow {
	l := data.Lib()

	if m.searching() {
		return trackRows(l.Search(m.filter))
	}

	switch s := m.sections[m.sectionIdx]; s.kind {
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

// 곡 목록의 칸 선언. 좁아지면 재생수 → 아티스트 → 길이 순으로 사라지고,
// 남는 공간은 제목이 가져간다.
var trackCols = []style.Col{
	{Min: 14, Weight: 3, Max: 58},          // 제목 — 절대 안 버린다
	{Min: 10, Weight: 1, Max: 26, Drop: 2}, // 아티스트
	{Min: 3, Drop: 3},                      // 재생 횟수
	{Min: 4, Drop: 1},                      // 길이
}

const colGap = 2

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

	start := style.Clamp(m.listTop, 0, style.Max(len(rows)-h, 0))
	end := style.Min(start+h, len(rows))

	// 이름이 이어지는 동안에는 아티스트를 다시 쓰지 않는다.
	// Bon Iver 가 열 줄 연속으로 같은 자리를 먹는 것이 낭비였다.
	widths := style.Columns(w-2, colGap, trackCols)

	var b strings.Builder
	prevArtist := ""
	if start > 0 && rows[start-1].track != nil {
		prevArtist = rows[start-1].track.Artist.Name
	}
	for i := start; i < end; i++ {
		b.WriteString(m.renderRow(rows[i], i == m.listIdx, w, widths, &prevArtist))
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

func (m Model) renderRow(r listRow, selected bool, w int, widths []int, prevArtist *string) string {
	// 선택 표시는 왼쪽 레일이다. 배경을 채우면 "채워진 빨강은 오류" 규칙과 부딪힌다.
	rail := "  "
	if selected {
		rail = style.Brand.Render("▌ ")
	}

	if g := r.group; g != nil {
		*prevArtist = ""
		left := style.Body.Render(style.Truncate(g.Name, style.Max(w-30, 12)))
		right := style.Faint.Render(fmt.Sprintf("%d tracks · %s", g.TrackCount, g.Subtitle))
		return rail + style.Row(left, right, w-2)
	}

	t := r.track
	cells := make([]string, 0, 4)
	pad := func(s string, width int) string {
		if width <= 0 {
			return ""
		}
		s = style.Truncate(s, width)
		if d := width - lipgloss.Width(s); d > 0 {
			s += strings.Repeat(" ", d)
		}
		return s
	}
	rpad := func(s string, width int) string {
		if width <= 0 {
			return ""
		}
		s = style.Truncate(s, width)
		if d := width - lipgloss.Width(s); d > 0 {
			s = strings.Repeat(" ", d) + s
		}
		return s
	}

	// 제목 — 재생 중이면 브랜드색, 한 번도 안 들었으면 흐리게.
	title := pad(t.Title, widths[0])
	switch {
	case m.nowPlayingID != 0 && t.Id == m.nowPlayingID:
		title = style.BrandBold.Render(title)
	case t.PlayCount == 0:
		title = style.Dim.Render(title)
	default:
		title = style.Body.Render(title)
	}
	cells = append(cells, title)

	if widths[1] > 0 {
		name := t.Artist.Name
		if name == *prevArtist {
			name = "" // 이어지는 같은 아티스트는 비워 둔다
		}
		cells = append(cells, style.Faint.Render(pad(name, widths[1])))
	}
	*prevArtist = t.Artist.Name

	if widths[2] > 0 {
		plays := fmt.Sprint(t.PlayCount)
		if t.PlayCount == 0 {
			plays = "·"
		}
		cells = append(cells, style.Faint.Render(rpad(plays, widths[2])))
	}
	if widths[3] > 0 {
		cells = append(cells, style.Faint.Render(rpad(style.MMSS(t.DurationMs), widths[3])))
	}

	return rail + strings.Join(cells, strings.Repeat(" ", colGap))
}
