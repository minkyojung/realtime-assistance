package musicapp

import (
	"strings"

	"amcli/tui/internal/data"
)

// 섹션 — 목록 패널이 무엇을 보여줄지 정한다.

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

// 섹션 이동은 tab 과 슬래시 명령으로 한다. 사이드바를 두지 않는 이유는
// "갈 수 있는 곳"을 상시로 보여줄 필요가 없기 때문이다. 필요한 것은
// "지금 어디인지"뿐이고, 그것은 상태줄이 말한다.
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
	return append(s,
		section{kind: secQueue, label: "Queue"},
		section{kind: secUnplayed, label: "Never played"})
}
