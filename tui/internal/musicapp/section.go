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
	secCatalog
	secShazam
)

type section struct {
	kind     sectionKind
	label    string
	playlist *data.Playlist // secPlaylist 일 때만
}

// 섹션 이동은 tab 과 슬래시 명령으로 한다. 사이드바를 두지 않는 이유는
// "갈 수 있는 곳"을 상시로 보여줄 필요가 없기 때문이다. 필요한 것은
// "지금 어디인지"뿐이고, 그것은 상태줄이 말한다.
// catalog·shazam 이 false 면 섹션 자체가 없다. 쓸 수 없는 곳으로 tab 이
// 가면 안 된다. 카탈로그는 설정이 있어야 생기고, 인식은 한 번 알아맞혀야 생긴다.
func buildSections(l *data.Library) []section { return buildSectionsWith(l, false, false) }

func buildSectionsWith(l *data.Library, catalog, shazam bool) []section {
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
	// 끝에만 붙인다. 중간에 끼우면 보고 있던 섹션 번호가 어긋난다.
	if catalog {
		s = append(s, section{kind: secCatalog, label: "Apple Music"})
	}
	if shazam {
		s = append(s, section{kind: secShazam, label: "Shazam"})
	}
	return s
}

// resync — 새 스냅샷 위에 화면 위치를 다시 앉힌다.
//
// 섹션 목록이 통째로 바뀌므로 "어디를 보고 있었는지"를 종류와 이름으로 되찾는다.
// 번호로 기억하면 플레이리스트가 하나 늘어난 순간 엉뚱한 곳을 보게 된다.
func (m *Model) resync(l *data.Library) {
	kind, label := secRecent, ""
	if m.sectionIdx < len(m.sections) {
		kind, label = m.sections[m.sectionIdx].kind, m.sections[m.sectionIdx].label
	}

	m.sections = buildSectionsWith(l, m.cat != nil, len(m.shzHits) > 0)
	m.sectionIdx = 0
	for i, s := range m.sections {
		if s.kind == kind && (kind != secPlaylist || s.label == label) {
			m.sectionIdx = i
			break
		}
	}

	// 파고든 목록은 옛 스냅샷의 곡을 들고 있다. 새로 읽었으면 그것을
	// 지킬 근거가 없으므로 섹션으로 되돌린다.
	m.drill = nil

	// 큐는 곡을 값으로 갖는다. id 는 안 바뀌므로 메타데이터만 새로 입힌다.
	// (재생 횟수가 늘었을 수 있다)
	for i := range m.queue {
		if t, ok := l.Track(m.queue[i].Track.Id); ok {
			m.queue[i].Track = t
		}
	}

	m.clampList()
}
