package ui

import (
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
	secUnplayed
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
	s = append(s,
		section{kind: secQueue, label: "Queue"},
		section{kind: secUnplayed, label: "Never played"})
	return s
}

func (m Model) viewSidebar(height int) string {
	var b strings.Builder
	write := func(s string) { b.WriteString(s + "\n") }

	// 검색·명령·도움말 중에는 사이드바 맨 위에 임시 항목이 뜬다.
	// 목록에 왜 다른 것이 보이는지가 여기서 설명된다.
	switch {
	case m.showHelp:
		write(stBrand.Render("▌") + stBrandBold.Render(" ? Help"))
		write("")
	case m.commanding():
		write(stBrand.Render("▌") + stBrandBold.Render(" / Commands"))
		write("")
	case m.mode == modeSearch:
		write(stBrand.Render("▌") + stBrandBold.Render(" ⌕ Search"))
		write("")
	}

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
		// 다른 것을 보고 있을 때는 섹션 선택 표시를 죽인다.
		if i == m.sectionIdx && !m.overlaying() {
			write(stBrand.Render("▌") + stBrandBold.Render(" "+label))
		} else {
			write("  " + stFaint.Render(label))
		}
	}

	lines := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lipgloss.NewStyle().Width(sidebarWidth).Render(
		strings.Join(lines[:height], "\n"))
}
