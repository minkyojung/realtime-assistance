package ui

import (
	"fmt"
	"strings"

	"amcli/tui/internal/data"
	"charm.land/lipgloss/v2"
)

// 사이드바. Apple Music 과 같은 구조로 두어 처음 쓰는 사람도 어디에
// 무엇이 있는지 짐작할 수 있게 한다.

type sectionKind int

const (
	secRecent sectionKind = iota
	secArtists
	secAlbums
	secSongs
	secPlaylist
	secQueue
)

type section struct {
	kind     sectionKind
	label    string
	playlist *data.Playlist // secPlaylist 일 때만
}

const sidebarWidth = 16

func buildSections(l *data.Library) []section {
	s := []section{
		{kind: secRecent, label: "Recently Added"},
		{kind: secArtists, label: "Artists"},
		{kind: secAlbums, label: "Albums"},
		{kind: secSongs, label: "Songs"},
	}
	for i := range l.Playlists {
		p := l.Playlists[i]
		if p.TrackCount == 0 {
			continue
		}
		s = append(s, section{kind: secPlaylist, label: strings.TrimSpace(p.Name), playlist: &p})
	}
	s = append(s, section{kind: secQueue, label: "Queue"})
	return s
}

func (m Model) viewSidebar(height int) string {
	var b strings.Builder
	write := func(s string) { b.WriteString(s + "\n") }

	write(stFaint.Render("LIBRARY"))
	for i, s := range m.sections {
		if s.kind == secPlaylist && (i == 0 || m.sections[i-1].kind != secPlaylist) {
			write("")
			write(stFaint.Render("PLAYLISTS"))
		}
		if s.kind == secQueue {
			write("")
		}

		label := truncate(s.label, sidebarWidth-2)
		switch {
		case i == m.sectionIdx && m.focus == focusSidebar:
			// 선택 + 포커스 — 브랜드 레일
			write(stBrand.Render("▌") + stBrandBold.Render(" "+label))
		case i == m.sectionIdx:
			write(stBrand.Render("▌") + stBody.Render(" "+label))
		default:
			write("  " + stDim.Render(label))
		}
	}

	lines := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lipgloss.NewStyle().Width(sidebarWidth).Render(
		strings.Join(lines[:height], "\n"))
}

// 목록 패널의 제목줄 — 무엇을 보고 있는지와 몇 개인지.
func (m Model) listHeader(w int) string {
	s := m.sections[m.sectionIdx]
	title := s.label
	if m.searching() {
		title = "Search"
	}
	count := ""
	if n := m.rowCount(); n > 0 {
		count = fmt.Sprintf("%d", n)
	}
	return row(stTitle.Render(truncate(title, w-10)), stFaint.Render(count), w)
}
