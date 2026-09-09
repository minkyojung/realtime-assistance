package musicapp

import (
	"amcli/tui/internal/style"
	"fmt"

	"amcli/tui/internal/api"
	"amcli/tui/internal/music"
	"charm.land/lipgloss/v2"
)

// 하단 재생 바. 어떤 목록을 보고 있든 항상 같은 자리에 있다.
// 화면에서 유일하게 절대 바뀌지 않는 요소이므로 여기가 기준점이 된다.

// 첫 실행 관문. 로그인이 아니라 권한과 앱 실행 여부다.
// 둘 다 통과하면 이 줄은 다시는 보이지 않는다.
func (m Model) viewGate(w int) (string, bool) {
	switch m.playerErr {
	case music.ErrPermissionDenied:
		return style.ErrorBadge.Render("PERMISSION") + " " +
			style.Body.Render(style.Truncate("Allow Terminal to control Music, then it just works", w-14)), true
	case music.ErrNotRunning:
		return style.ErrorBadge.Render("MUSIC APP") + " " +
			style.Body.Render(style.Truncate("Music is not running. Open it to start playback", w-13)), true
	case nil:
		return "", false
	default:
		return style.Warn.Render("! ") + style.Dim.Render(style.Truncate(m.playerErr.Error(), w-2)), true
	}
}

// 관문 아래 한 줄로 다음에 뭘 하면 되는지 알려준다.
func (m Model) viewGateHint(w int) (string, bool) {
	switch m.playerErr {
	case music.ErrPermissionDenied:
		return style.Faint.Render(style.Truncate("ctrl+g  open System Settings › Privacy & Security › Automation", w)), true
	case music.ErrNotRunning:
		return style.Faint.Render(style.Truncate("open -a Music     · no sign-in needed, it uses the app you already use", w)), true
	}
	return "", false
}

func (m Model) viewPlayer(w int) string {
	if gate, ok := m.viewGate(w); ok {
		return gate
	}
	// Music.app 이 말해주는 것을 그대로 그린다.
	// 라이브러리에 없는 곡(카탈로그 스트리밍)도 이 경로로 보인다.
	title, artist, durationMs := m.live.Title, m.live.Artist, m.live.DurationMs
	if title == "" {
		if t, ok := m.nowPlaying(); ok {
			title, artist, durationMs = t.Title, t.Artist.Name, t.DurationMs
		}
	}
	if title == "" {
		if !m.polled {
			return style.Faint.Render("Connecting to Music…")
		}
		return style.Faint.Render(style.Truncate("Nothing playing · pick a track and press enter", w))
	}

	icon := style.Brand.Render("▶")
	if !m.playing {
		icon = style.Faint.Render("❚❚")
	}

	label := style.Body.Render(style.Truncate(title, 28)) +
		style.Faint.Render(" — "+style.Truncate(artist, 20))

	pos := style.Clamp(m.positionMs, 0, durationMs)
	timeLabel := fmt.Sprintf(" %s / %s", style.MMSS(pos), style.MMSS(durationMs))
	left := icon + "  " + label

	barW := w - lipgloss.Width(left) - lipgloss.Width(timeLabel) - 3
	if barW < 8 {
		return style.Row(left, style.Faint.Render(timeLabel), w)
	}

	ratio := 0.0
	if durationMs > 0 {
		ratio = float64(pos) / float64(durationMs)
	}
	return left + "  " + style.Progress(barW, ratio) + style.Faint.Render(timeLabel)
}

// 입력창 바로 위 한 줄. 상황에 따라 무엇이 오는지가 다르다.
//
//	요청 중  → 스피너
//	실패     → 사유
//	평소     → 지금 곡의 선정 근거 (있을 때만)
//
// 셋 다 없으면 줄 자체가 없다. 빈 줄을 남기지 않는다.
func (m Model) viewReason(w int) (string, bool) {
	if m.thinking {
		return m.spinner.View() + style.Dim.Render(" Thinking…"), true
	}
	if m.intentErr != nil {
		return style.ErrorBadge.Render("FAILED") + " " +
			style.Dim.Render(style.Truncate(m.intentErr.Error(), w-9)), true
	}
	if m.notice != "" {
		return style.BrandSoft.Render("· ") + style.Dim.Render(style.Truncate(m.notice, w-2)), true
	}
	it, ok := m.nowPlayingItem()
	if !ok || it.Reason == nil || *it.Reason == "" {
		return "", false
	}
	return style.Brand.Render("▸ ") + style.Dim.Render(style.Truncate(*it.Reason, w-2)), true
}

// Status 는 상태줄에 들어갈 한 줄이다. 배경에 있어도 호출된다.
//
// 사이드바를 없앴으므로 **"지금 어디인지"를 말해줄 곳이 여기밖에 없다.**
// 갈 수 있는 곳은 `/` 가 보여주므로 여기서는 말하지 않는다.
func (m Model) Status() string {
	where := m.sections[m.sectionIdx].label
	if m.searching() {
		where = "Search"
	}
	left := style.Dim.Render(where) +
		style.Faint.Render(fmt.Sprintf(" · %d", m.rowCount()))

	if n := len(m.queue); n > 0 && m.sections[m.sectionIdx].kind != secQueue {
		left += style.Faint.Render(fmt.Sprintf("   queue %d", n))
	}
	if m.queueTitle != "" && m.sections[m.sectionIdx].kind == secQueue {
		left += style.Faint.Render("   " + style.Truncate(m.queueTitle, 40))
	}

	u := m.usage
	if u.PromptTokens > 0 || u.CompletionTokens > 0 {
		left += style.Faint.Render(fmt.Sprintf("   ↑%s ↓%s  $%.4f",
			style.Tokens(u.PromptTokens), style.Tokens(u.CompletionTokens), u.CostUsd))
	}
	return left
}

func (m Model) nowPlaying() (api.Track, bool) {
	it, ok := m.nowPlayingItem()
	if !ok {
		return api.Track{}, false
	}
	return it.Track, true
}

func (m Model) nowPlayingItem() (api.QueueItem, bool) {
	for _, it := range m.queue {
		if it.Track.Id == m.nowPlayingID {
			return it, true
		}
	}
	return api.QueueItem{}, false
}

func (m Model) queueTracks() []api.Track {
	out := make([]api.Track, 0, len(m.queue))
	for _, it := range m.queue {
		out = append(out, it.Track)
	}
	return out
}
