// Package data 는 화면 개발용 데이터를 담는다.
//
// library.json 은 작성자 본인 Music.app 에서 실제로 뽑은 157곡이다
// (docs/05-스파이크-라이브러리-실측.md). 서버 연동 전까지 이걸로 그린다.
package data

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"

	"amcli/tui/internal/api"
)

//go:embed library.json
var libraryJSON []byte

type Playlist struct {
	Id         int64   `json:"id"`
	Name       string  `json:"name"`
	TrackCount int     `json:"trackCount"`
	TrackIds   []int64 `json:"trackIds"`
}

type Library struct {
	Tracks    []api.Track `json:"tracks"`
	Playlists []Playlist  `json:"playlists"`

	byID map[int64]api.Track
}

var lib *Library

// Lib 은 임베드된 라이브러리를 한 번만 읽어 돌려준다.
func Lib() *Library {
	if lib != nil {
		return lib
	}
	l := &Library{}
	if err := json.Unmarshal(libraryJSON, l); err != nil {
		panic("library.json 을 읽을 수 없다: " + err.Error())
	}
	l.byID = make(map[int64]api.Track, len(l.Tracks))
	for _, t := range l.Tracks {
		l.byID[t.Id] = t
	}
	lib = l
	return lib
}

// ByPersistentID — Music.app 이 알려주는 식별자로 우리 곡을 찾는다.
// 이 값이 두 세계를 잇는 유일한 키다.
func (l *Library) ByPersistentID(pid string) (api.Track, bool) {
	for _, t := range l.Tracks {
		if t.PersistentId != nil && *t.PersistentId == pid {
			return t, true
		}
	}
	return api.Track{}, false
}

func (l *Library) Track(id int64) (api.Track, bool) {
	t, ok := l.byID[id]
	return t, ok
}

// inLibrary 는 라이브러리에 담긴 곡만 고른다.
//
// 플레이리스트에만 담고 라이브러리에는 추가하지 않은 곡이 실제로 있다
// (실측: CH 30곡 중 28곡). 그 곡들은 플레이리스트에서만 보여야 한다.
func (l *Library) inLibrary() []api.Track {
	out := make([]api.Track, 0, len(l.Tracks))
	for _, t := range l.Tracks {
		if t.InLibrary {
			out = append(out, t)
		}
	}
	return out
}

// Songs 는 제목순 전곡이다.
func (l *Library) Songs() []api.Track {
	out := l.inLibrary()
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out
}

// RecentlyAdded 는 담은 순 역순이다. Apple Music 의 기본 화면.
func (l *Library) RecentlyAdded() []api.Track {
	out := l.inLibrary()
	sort.Slice(out, func(i, j int) bool { return out[i].AddedAt.After(out[j].AddedAt) })
	return out
}

type Group struct {
	Name       string
	Subtitle   string
	TrackCount int
	Tracks     []api.Track
}

// Artists 는 아티스트별 묶음이다. 담은 곡이 많은 순.
func (l *Library) Artists() []Group {
	m := map[string][]api.Track{}
	for _, t := range l.inLibrary() {
		m[t.Artist.Name] = append(m[t.Artist.Name], t)
	}
	return groups(m, func(ts []api.Track) string { return plays(ts) })
}

// Albums 는 앨범별 묶음이다.
func (l *Library) Albums() []Group {
	m := map[string][]api.Track{}
	key := map[string]string{}
	for _, t := range l.inLibrary() {
		if t.Album == nil {
			continue
		}
		k := t.Album.Title
		m[k] = append(m[k], t)
		key[k] = t.Album.Artist.Name
	}
	gs := groups(m, func(ts []api.Track) string { return plays(ts) })
	for i := range gs {
		gs[i].Subtitle = key[gs[i].Name] + " · " + gs[i].Subtitle
	}
	return gs
}

func groups(m map[string][]api.Track, sub func([]api.Track) string) []Group {
	out := make([]Group, 0, len(m))
	for name, ts := range m {
		sort.Slice(ts, func(i, j int) bool { return ts[i].Title < ts[j].Title })
		out = append(out, Group{Name: name, Subtitle: sub(ts), TrackCount: len(ts), Tracks: ts})
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i].Tracks) != len(out[j].Tracks) {
			return len(out[i].Tracks) > len(out[j].Tracks)
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func plays(ts []api.Track) string {
	n := 0
	for _, t := range ts {
		n += t.PlayCount
	}
	return itoa(n) + " plays"
}

// PlaylistTracks 는 플레이리스트에 담긴 곡을 순서대로 돌려준다.
func (l *Library) PlaylistTracks(p Playlist) []api.Track {
	out := make([]api.Track, 0, len(p.TrackIds))
	for _, id := range p.TrackIds {
		if t, ok := l.Track(id); ok {
			out = append(out, t)
		}
	}
	return out
}

// Search 는 제목·아티스트·앨범을 대소문자 무시하고 부분 일치로 찾는다.
func (l *Library) Search(q string) []api.Track {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil
	}
	// 검색은 플레이리스트에만 있는 곡도 찾는다. 담아둔 곳이 어디든 내 음악이다.
	var out []api.Track
	for _, t := range l.Tracks {
		hay := strings.ToLower(t.Title + " " + t.Artist.Name)
		if t.Album != nil {
			hay += " " + strings.ToLower(t.Album.Title)
		}
		if strings.Contains(hay, q) {
			out = append(out, t)
		}
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
