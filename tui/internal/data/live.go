package data

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"time"

	"amcli/tui/internal/api"
)

// 원시 덤프. dump.js 가 뱉는 모양 그대로다.
// persistent ID 로만 말하고, 우리 내부 id 는 여기서 붙인다.
type dumpTrack struct {
	PersistentId string  `json:"persistentId"`
	Title        string  `json:"title"`
	Artist       string  `json:"artist"`
	AlbumTitle   string  `json:"albumTitle"`
	AlbumArtist  string  `json:"albumArtist"`
	Genre        *string `json:"genre"`
	Year         *int    `json:"year"`
	DurationMs   int     `json:"durationMs"`
	PlayCount    int     `json:"playCount"`
	SkipCount    int     `json:"skipCount"`
	Favorited    bool    `json:"favorited"`
	Disliked     bool    `json:"disliked"`
	Source       string  `json:"source"`
	InLibrary    bool    `json:"inLibrary"`
	AddedAt      *string `json:"addedAt"`
	LastPlayedAt *string `json:"lastPlayedAt"`
}

type dumpPlaylist struct {
	PersistentId string   `json:"persistentId"`
	Name         string   `json:"name"`
	TrackIds     []string `json:"trackIds"` // persistent ID 목록
}

type dump struct {
	Error     string         `json:"error"`
	Tracks    []dumpTrack    `json:"tracks"`
	Playlists []dumpPlaylist `json:"playlists"`
}

// TrackID 는 persistent ID(16자리 16진수)를 그대로 int64 로 읽는다.
//
// 해시가 아니라 값 자체다. 충돌이 없고, 다시 읽어도 같은 수가 나오며,
// 되돌릴 수도 있다 — 그래서 스냅샷이 새로 와도 id 가 흔들리지 않는다.
// 일련번호였다면 곡 하나가 늘 때마다 뒤의 번호가 전부 밀려,
// 재생 중인 곡과 큐가 조용히 다른 곡을 가리키게 된다.
//
// 8~F 로 시작하는 pid 는 음수가 된다. 문제 없다 — 0 만 "없음"이다.
func TrackID(persistentID string) int64 {
	u, err := strconv.ParseUint(persistentID, 16, 64)
	if err != nil {
		return 0
	}
	return int64(u)
}

// FromDump 는 Music.app 덤프를 스냅샷으로 바꾼다.
func FromDump(b []byte) (*Library, error) {
	var d dump
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	if d.Error != "" {
		return nil, errors.New("Music.app: " + d.Error)
	}
	if len(d.Tracks) == 0 {
		return nil, errors.New("library is empty")
	}

	// 아티스트·앨범 id 는 이름 안에서만 뜻이 있다. 이름이 같으면 같은 번호를 준다.
	artistIDs := map[string]int64{}
	albumIDs := map[string]int64{}
	idFor := func(m map[string]int64, name string) int64 {
		if id, ok := m[name]; ok {
			return id
		}
		id := int64(len(m) + 1)
		m[name] = id
		return id
	}

	l := &Library{
		Tracks:   make([]api.Track, 0, len(d.Tracks)),
		Version:  libraryVersion,
		SyncedAt: time.Now(),
	}
	for _, t := range d.Tracks {
		id := TrackID(t.PersistentId)
		if id == 0 {
			continue // persistent ID 가 없는 곡은 우리 두 세계를 이을 수 없다
		}
		pid := t.PersistentId
		src := api.Shared
		if t.Source == "file" {
			src = api.File
		}
		track := api.Track{
			Id:           id,
			PersistentId: &pid,
			Title:        t.Title,
			Artist:       api.Artist{Id: idFor(artistIDs, t.Artist), Name: t.Artist},
			Genre:        t.Genre,
			Year:         t.Year,
			DurationMs:   t.DurationMs,
			Source:       src,
			PlayCount:    t.PlayCount,
			SkipCount:    t.SkipCount,
			Favorited:    t.Favorited,
			Disliked:     t.Disliked,
			InLibrary:    t.InLibrary,
			AddedAt:      parseTime(t.AddedAt),
			LastPlayedAt: parseTime(t.LastPlayedAt),
		}
		if t.AlbumTitle != "" {
			track.Album = &api.Album{
				Id:          idFor(albumIDs, t.AlbumTitle),
				Title:       t.AlbumTitle,
				Artist:      api.Artist{Id: idFor(artistIDs, t.AlbumArtist), Name: t.AlbumArtist},
				ReleaseYear: t.Year,
			}
		}
		l.Tracks = append(l.Tracks, track)
	}

	for i, p := range d.Playlists {
		ids := make([]int64, 0, len(p.TrackIds))
		for _, pid := range p.TrackIds {
			if id := TrackID(pid); id != 0 {
				ids = append(ids, id)
			}
		}
		l.Playlists = append(l.Playlists, Playlist{
			Id:         int64(i + 1),
			Name:       p.Name,
			TrackCount: len(ids),
			TrackIds:   ids,
		})
	}
	sort.SliceStable(l.Playlists, func(i, j int) bool {
		return l.Playlists[i].TrackCount > l.Playlists[j].TrackCount
	})

	return l.index(), nil
}

func parseTime(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil
	}
	return &t
}
