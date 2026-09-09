package musicapp

import (
	"context"
	"time"

	"amcli/tui/internal/api"
	"amcli/tui/internal/data"
	"amcli/tui/internal/intent"
	"amcli/tui/internal/music"
	tea "charm.land/bubbletea/v2"
)

// Music.app 은 우리에게 알림을 주지 않으므로 폴링한다.
// 1초는 3초 이내 이탈(skip_early)을 잡기에 충분한 주기다. docs/03 7절.
const pollInterval = time.Second

type tickMsg struct{}

// queueMsg 는 의도 층이 만들어낸 큐다. 실패도 여기로 온다.
type queueMsg struct {
	res intent.Result
	err error
}

// cmdBuildQueue — 자연어 한 줄을 큐로 바꾼다. 입력창을 막지 않도록 Cmd 로 돈다.
func cmdBuildQueue(prompt string, library []api.Track, cur intent.Current) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		res, err := intent.Build(ctx, prompt, library, cur, time.Now())
		return queueMsg{res: res, err: err}
	}
}

type statusMsg struct {
	state music.PlayerState
	err   error
}

func tick() tea.Cmd {
	return tea.Tick(pollInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

func fetchStatus() tea.Msg {
	st, err := music.Status()
	return statusMsg{state: st, err: err}
}

// 아래 조작은 결과를 기다리지 않는다. 다음 폴링이 진짜 상태를 알려준다.
func cmdPlayPause() tea.Cmd {
	return func() tea.Msg { music.PlayPause(); return fetchStatus() }
}

func cmdPlayTrack(persistentID string) tea.Cmd {
	return func() tea.Msg { music.PlayPersistentID(persistentID); return fetchStatus() }
}

func cmdNext() tea.Cmd {
	return func() tea.Msg { music.Next(); return fetchStatus() }
}

func cmdPrevious() tea.Cmd {
	return func() tea.Msg { music.Previous(); return fetchStatus() }
}

func cmdOpenSettings() tea.Cmd {
	return func() tea.Msg { music.OpenSettings(); return nil }
}

// ProbeStatus 는 헤드리스 확인용이다. 실제 Music.app 상태를 한 번 읽는다.
func ProbeStatus() tea.Msg { return fetchStatus() }

// DrainForQueue 는 헤드리스 확인용이다. Batch 안에서 큐 생성 Cmd 를 찾아 실행한다.
func DrainForQueue(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if b, ok := msg.(tea.BatchMsg); ok {
		for _, c := range b {
			if m := DrainForQueue(c); m != nil {
				return m
			}
		}
		return nil
	}
	if _, ok := msg.(queueMsg); ok {
		return msg
	}
	return nil
}

// savedMsg — 큐를 Apple Music 플레이리스트로 저장한 결과 (F9).
type savedMsg struct {
	name string
	err  error
}

func cmdSavePlaylist(name string, tracks []api.Track) tea.Cmd {
	return func() tea.Msg {
		ids := make([]string, 0, len(tracks))
		for _, t := range tracks {
			if t.PersistentId != nil {
				ids = append(ids, *t.PersistentId)
			}
		}
		return savedMsg{name: name, err: music.CreatePlaylist(name, ids)}
	}
}

// 아래 둘은 테스트에서 앱 안쪽 상태를 흘려보내기 위한 것이다.
// 실제 API 나 Music.app 을 부르지 않고 화면을 확인할 수 있게 한다.

// StatusMsgFor 는 Music.app 폴링 결과를 흉내 낸다.
func StatusMsgFor(state music.PlayerState, err error) tea.Msg {
	return statusMsg{state: state, err: err}
}

// QueueMsgFor 는 의도 층의 결과를 흉내 낸다.
func QueueMsgFor(res intent.Result, err error) tea.Msg {
	return queueMsg{res: res, err: err}
}

// libraryMsg — 라이브러리 스냅샷이 도착했다. 캐시와 Music.app 두 경로로 온다.
type libraryMsg struct {
	lib      *data.Library
	live     bool // Music.app 에서 직접 읽은 것인지
	announce bool // /reload 로 부른 것인지 — 시작할 때마다 로그를 더럽히지 않는다
	err      error
}

// cmdLoadCache 는 지난번에 읽어둔 스냅샷을 꺼낸다. 몇 ms 다.
func cmdLoadCache() tea.Msg {
	l, err := data.LoadCache()
	return libraryMsg{lib: l, err: err}
}

// cmdDumpLibrary 는 Music.app 라이브러리를 통째로 읽는다. 몇 초다.
//
// 읽어낸 것은 캐시에 남긴다. 다음 시작이 즉시 그려지도록.
func cmdDumpLibrary(announce bool) tea.Cmd {
	return func() tea.Msg {
		b, err := music.DumpLibrary(context.Background())
		if err != nil {
			return libraryMsg{live: true, announce: announce, err: err}
		}
		l, err := data.FromDump(b)
		if err != nil {
			return libraryMsg{live: true, announce: announce, err: err}
		}
		_ = data.SaveCache(l) // 실패해도 다음 시작이 조금 느릴 뿐이다
		return libraryMsg{lib: l, live: true, announce: announce}
	}
}
