// Package data 는 화면 개발용 더미 데이터를 담는다.
//
// 값은 작성자 본인 라이브러리에서 실제로 뽑은 것이다
// (docs/05-스파이크-라이브러리-실측.md). 스크린샷이 진짜가 되도록.
package data

import (
	"time"

	"amcli/tui/internal/api"
)

func s(v string) *string { return &v }
func i(v int) *int       { return &v }

func artist(id int64, name string) api.Artist {
	return api.Artist{Id: id, Name: name}
}

func album(id int64, title string, a api.Artist, year int) *api.Album {
	return &api.Album{Id: id, Title: title, Artist: a, ReleaseYear: i(year)}
}

var (
	bonIver = artist(1, "Bon Iver")
	akimbo  = artist(2, "Akimbo")

	albBonIver2011 = album(1, "Bon Iver, Bon Iver (10th Anniversary Edition)", bonIver, 2011)
	albBloodBank   = album(2, "Blood Bank (10th Anniversary Edition) - EP", bonIver, 2009)
	albSeoulSori   = album(3, "Red Bull Music Seoul Sori", akimbo, 2018)
)

var addedAt = time.Date(2026, 7, 30, 15, 21, 45, 0, time.Local)

// 전부 playCount 0 · lastPlayedAt nil — 담아두고 한 번도 듣지 않은 곡이다.
func track(id int64, title string, a api.Artist, alb *api.Album, genre string, year, durMs int) api.Track {
	return api.Track{
		Id:         id,
		Title:      title,
		Artist:     a,
		Album:      alb,
		Genre:      s(genre),
		Year:       i(year),
		DurationMs: durMs,
		Source:     api.Shared,
		PlayCount:  0,
		SkipCount:  0,
		AddedAt:    addedAt,
		// LastPlayedAt 를 비워 두는 것이 이 서비스의 전부다.
	}
}

// Session 은 S1 재생 뷰가 그리는 세션이다.
func Session() api.Session {
	title := "조용한 걸로, 담아두고 안 들은 것 위주"
	total := 262079 + 225733 + 239375 + 285440 + 188039 + 380852 + 206742 + 232440

	return api.Session{
		Id:         1,
		Title:      title,
		Prompt:     title,
		Status:     api.Active,
		StartedAt:  time.Now().Add(-18 * time.Minute),
		TrackCount: i(8),
		Usage: api.Usage{
			PromptTokens:     1240,
			CompletionTokens: 380,
			CostUsd:          0.0031,
		},
		Summary: &api.QueueSummary{
			TrackCount:       8,
			TotalDurationMs:  total,
			NeverPlayedCount: 7,
		},
		QueueItems: &[]api.QueueItem{
			qi(1, 1, track(11, "Perth", bonIver, albBonIver2011, "Alternative", 2011, 262079),
				"2026년 7월에 담아두고 한 번도 재생하지 않았습니다", api.Playing),
			qi(2, 2, track(12, "Michicant", bonIver, albBonIver2011, "Alternative", 2011, 225733),
				"같은 앨범에서 이어집니다. 역시 미재생", api.Pending),
			qi(3, 3, track(13, "Jirisan Breeze", akimbo, albSeoulSori, "Hip-Hop/Rap", 2018, 239375),
				"장르는 다르지만 밀도가 비슷합니다", api.Pending),
			qi(4, 4, track(14, "Woods", bonIver, albBloodBank, "Alternative", 2009, 285440),
				"미재생. 목소리만 남는 편성입니다", api.Pending),
			qi(5, 5, track(15, "Towers", bonIver, albBonIver2011, "Alternative", 2011, 188039),
				"미재생. 앞 곡보다 짧게 끊습니다", api.Pending),
		},
	}
}

func qi(id int64, pos int, t api.Track, reason string, st api.QueueItemState) api.QueueItem {
	return api.QueueItem{
		Id:        id,
		SessionId: 1,
		Position:  pos,
		Track:     t,
		Reason:    s(reason),
		State:     st,
		Origin:    api.QueueItemOriginGeneration,
	}
}

// PlaybackPositionMs 는 Music.app 폴링으로 얻는 값이라 DB 에 없다.
// 화면 개발 중에는 고정값을 쓴다.
const PlaybackPositionMs = 82_000

// HiddenQueueCount 는 접힌 큐 항목 수 (ctrl+o 로 펼침).
const HiddenQueueCount = 3
