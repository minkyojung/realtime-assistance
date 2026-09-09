// Package data 는 화면이 보는 라이브러리를 담는다.
//
// 스냅샷 하나가 한 시점의 Music.app 이다. 만들어진 뒤로는 바뀌지 않으므로
// 통째로 갈아끼우면 되고, 읽는 쪽은 잠글 필요가 없다.
package data

import (
	"encoding/json"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"amcli/tui/internal/api"
)

type Playlist struct {
	Id         int64   `json:"id"`
	Name       string  `json:"name"`
	TrackCount int     `json:"trackCount"`
	TrackIds   []int64 `json:"trackIds"`
}

type Library struct {
	Tracks    []api.Track `json:"tracks"`
	Playlists []Playlist  `json:"playlists"`

	// 캐시 파일에만 있는 값이다. 픽스처에는 없어 zero value 로 읽힌다.
	Version  int       `json:"version,omitempty"`
	SyncedAt time.Time `json:"syncedAt,omitempty"`

	byID  map[int64]api.Track
	byPID map[string]int64
}

var current atomic.Pointer[Library]

// Lib 은 지금 스냅샷을 돌려준다. 언제나 nil 이 아니다.
//
// 아직 아무것도 읽지 않았으면 빈 라이브러리다. 남의 라이브러리를 대신
// 보여주지 않는다 — 그 위에 의도 층이 그럴듯한 근거를 지어낸다.
func Lib() *Library {
	if l := current.Load(); l != nil {
		return l
	}
	empty := (&Library{}).index()
	current.CompareAndSwap(nil, empty)
	return current.Load()
}

// Set 은 스냅샷을 통째로 갈아끼운다. 색인은 여기서 만든다 — 빠뜨릴 수 없게.
func Set(l *Library) { current.Store(l.index()) }

// FromJSON 은 {tracks, playlists} 를 읽어 스냅샷을 만든다.
func FromJSON(b []byte) (*Library, error) {
	l := &Library{}
	if err := json.Unmarshal(b, l); err != nil {
		return nil, err
	}
	return l.index(), nil
}

func (l *Library) index() *Library {
	l.byID = make(map[int64]api.Track, len(l.Tracks))
	l.byPID = make(map[string]int64, len(l.Tracks))
	for _, t := range l.Tracks {
		l.byID[t.Id] = t
		if t.PersistentId != nil {
			l.byPID[*t.PersistentId] = t.Id
		}
	}
	return l
}

// ByPersistentID — Music.app 이 알려주는 식별자로 우리 곡을 찾는다.
// 이 값이 두 세계를 잇는 유일한 키다.
//
// 1초마다 불린다. 선형 탐색이면 곡 수만큼 문자열을 비교하게 된다.
func (l *Library) ByPersistentID(pid string) (api.Track, bool) {
	id, ok := l.byPID[pid]
	if !ok {
		return api.Track{}, false
	}
	t, ok := l.byID[id]
	return t, ok
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
	// addedAt 이 없는 곡(라이브러리 밖)은 뒤로 민다.
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].AddedAt, out[j].AddedAt
		if a == nil || b == nil {
			return b == nil && a != nil
		}
		return a.After(*b)
	})
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
