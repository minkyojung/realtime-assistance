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
// 한 줄은 곡일 수도, 묶음일 수도, 카탈로그 곡일 수도, 구역 머리글일 수도 있다.
// 넷은 서로 배타적이다. 카탈로그 곡은 아직 내 것이 아니라는 점이 다르다.
type listRow struct {
	track   *api.Track
	group   *data.Group
	catalog *api.CatalogTrack
	header  string // 검색 결과를 두 구역으로 가르는 줄. 고를 수 없다
}

// selectable — 머리글에는 커서가 서지 않는다.
func (r listRow) selectable() bool { return r.header == "" }

func (m Model) rows() []listRow {
	l := data.Lib()

	if m.searching() {
		return m.searchRows(l)
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
	case secCatalog:
		return catalogRows(m.catHits)
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
		switch {
		case m.searching() && m.cat != nil:
			// 발견은 여기서 일어난다. 자동으로 바깥까지 찾아 주지는 않는다 —
			// 라이브러리를 벗어나는 것은 사용자가 정할 일이다.
			msg = "Nothing in Your Library  ·  searching Apple Music…"
		case m.searching():
			msg = "No matches"
		case !m.synced && m.playerErr == nil:
			msg = "Reading Your Library from Music…"
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

	// 구역 머리글 — 레일 자리를 비워 곡 줄과 구분한다.
	if r.header != "" {
		return "  " + style.Faint.Render(style.Truncate(r.header, w-2))
	}

	if ct := r.catalog; ct != nil {
		return rail + m.catalogCells(*ct, widths, prevArtist)
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

// catalogCells — 카탈로그 곡 한 줄.
//
// 칸은 라이브러리 곡과 같은 것을 쓴다. 따로 정의하면 숫자 넷이 두 벌이 되고,
// tab 으로 오갈 때 제목 열이 튄다. 같은 격자에 다른 내용을 넣는다:
// 재생수 자리에는 소속 표시를, 길이 자리에는 연도를.
func (m Model) catalogCells(ct api.CatalogTrack, widths []int, prevArtist *string) string {
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

	// 아직 내 것이 아닌 곡은 흐리다. 담긴 곡은 라이브러리 곡과 같은 밝기가 된다.
	titleStyle, mark := style.Dim, style.Faint.Render(rpad("+", widths[2]))
	if inLibrary(ct) {
		titleStyle = style.Body
		mark = style.Brand.Render(rpad("✓", widths[2]))
	}
	if m.adding != nil && m.adding.track.AppleMusicId == ct.AppleMusicId {
		mark = style.Brand.Render(rpad("⋯", widths[2]))
	}

	cells := []string{titleStyle.Render(pad(ct.Title, widths[0]))}
	if widths[1] > 0 {
		name := ct.ArtistName
		if name == *prevArtist {
			name = ""
		}
		*prevArtist = ct.ArtistName
		cells = append(cells, style.Faint.Render(pad(name, widths[1])))
	}
	if widths[2] > 0 {
		cells = append(cells, mark)
	}
	if widths[3] > 0 {
		year := ""
		if ct.Year != nil {
			year = fmt.Sprint(*ct.Year)
		}
		cells = append(cells, style.Faint.Render(rpad(year, widths[3])))
	}
	return strings.Join(cells, "  ")
}

// searchRows — 한 번의 검색이 두 곳을 보여준다.
//
// 위는 이미 내 것, 아래는 아직 내 것이 아닌 것. 같은 화면에 두어야
// "내가 가진 게 이것뿐이구나"와 "바깥에는 이런 게 있구나"가 한눈에 붙는다.
// 아래쪽은 타이핑을 멈춘 뒤에야 채워진다 — 글자마다 요청을 보낼 수는 없다.
func (m Model) searchRows(l *data.Library) []listRow {
	mine := trackRows(l.Search(m.filter))

	// 카탈로그가 없거나(미설정) 아직 안 왔으면 내 것만 보여준다.
	// 지금 화면의 검색어와 결과의 검색어가 다르면 낡은 결과다.
	if m.cat == nil || m.catTerm != strings.TrimSpace(m.filter) || len(m.catHits) == 0 {
		return mine
	}

	out := make([]listRow, 0, len(mine)+len(m.catHits)+2)
	if len(mine) > 0 {
		out = append(out, listRow{header: "Your Library"})
		out = append(out, mine...)
	}
	out = append(out, listRow{header: "Apple Music"})
	return append(out, catalogRows(m.catHits)...)
}
