package ui

import (
	"time"

	"amcli/tui/internal/music"
	tea "charm.land/bubbletea/v2"
)

// Music.app 은 우리에게 알림을 주지 않으므로 폴링한다.
// 1초는 3초 이내 이탈(skip_early)을 잡기에 충분한 주기다. docs/03 7절.
const pollInterval = time.Second

type tickMsg struct{}
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
