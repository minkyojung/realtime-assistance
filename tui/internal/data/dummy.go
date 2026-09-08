package data

import "amcli/tui/internal/api"

func s(v string) *string { return &v }

// Queue 는 의도 층이 만들어낸 재생 큐다. 서버 연동 전까지는 고정값이지만,
// 곡 자체는 실제 라이브러리(library.json)에서 가져온다.
func Queue() []api.QueueItem {
	l := Lib()
	picks := []struct {
		title  string
		reason string
	}{
		{"Perth", "Added Jul 2026, never played once"},
		{"Michicant", "Continues the same record, also never played"},
		{"Woods", "Never played. Strips down to just the voice"},
		{"Towers", "Never played. Shorter, ends the run"},
		{"Jirisan Breeze", "Different genre, similar density"},
	}

	var out []api.QueueItem
	pos := 1
	for _, p := range picks {
		for _, t := range l.Tracks {
			if t.Title != p.title {
				continue
			}
			state := api.Pending
			if pos == 1 {
				state = api.Playing
			}
			out = append(out, api.QueueItem{
				Id:        int64(pos),
				SessionId: 1,
				Position:  pos,
				Track:     t,
				Reason:    s(p.reason),
				State:     state,
				Origin:    api.QueueItemOriginGeneration,
			})
			pos++
			break
		}
	}
	return out
}

// PlaybackPositionMs 는 Music.app 폴링으로 얻는 값이라 DB 에 없다.
const PlaybackPositionMs = 82_000
