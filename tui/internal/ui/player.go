package ui

import (
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
		return stErrorBadge.Render("PERMISSION") + " " +
			stBody.Render(truncate("Allow Terminal to control Music, then it just works", w-14)), true
	case music.ErrNotRunning:
		return stErrorBadge.Render("MUSIC APP") + " " +
			stBody.Render(truncate("Music is not running. Open it to start playback", w-13)), true
	case nil:
		return "", false
	default:
		return stWarn.Render("! ") + stDim.Render(truncate(m.playerErr.Error(), w-2)), true
	}
}

// 관문 아래 한 줄로 다음에 뭘 하면 되는지 알려준다.
func (m Model) viewGateHint(w int) (string, bool) {
	switch m.playerErr {
	case music.ErrPermissionDenied:
		return stFaint.Render(truncate("ctrl+g  open System Settings › Privacy & Security › Automation", w)), true
	case music.ErrNotRunning:
		return stFaint.Render(truncate("open -a Music     · no sign-in needed, it uses the app you already use", w)), true
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
			return stFaint.Render("Connecting to Music…")
		}
		return stFaint.Render(truncate("Nothing playing · pick a track and press enter", w))
	}

	icon := stBrand.Render("▶")
	if !m.playing {
		icon = stFaint.Render("❚❚")
	}

	label := stBody.Render(truncate(title, 28)) +
		stFaint.Render(" — "+truncate(artist, 20))

	pos := clamp(m.positionMs, 0, durationMs)
	timeLabel := fmt.Sprintf(" %s / %s", mmss(pos), mmss(durationMs))
	left := icon + "  " + label

	barW := w - lipgloss.Width(left) - lipgloss.Width(timeLabel) - 3
	if barW < 8 {
		return row(left, stFaint.Render(timeLabel), w)
	}

	ratio := 0.0
	if durationMs > 0 {
		ratio = float64(pos) / float64(durationMs)
	}
	return left + "  " + progress(barW, ratio) + stFaint.Render(timeLabel)
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
		return m.spinner.View() + stDim.Render(" Thinking…"), true
	}
	if m.intentErr != nil {
		return stErrorBadge.Render("FAILED") + " " +
			stDim.Render(truncate(m.intentErr.Error(), w-9)), true
	}
	if m.notice != "" {
		return stBrandSoft.Render("· ") + stDim.Render(truncate(m.notice, w-2)), true
	}
	it, ok := m.nowPlayingItem()
	if !ok || it.Reason == nil || *it.Reason == "" {
		return "", false
	}
	return stBrand.Render("▸ ") + stDim.Render(truncate(*it.Reason, w-2)), true
}

// 하단 상태줄 — 왼쪽은 큐 요약, 오른쪽은 누적 사용량.
// 사용량을 상시 노출하는 것은 agentic CLI 의 관례이자,
// 사용자가 AI 사용량을 스스로 통제할 수 있게 하는 장치다.
func (m Model) viewStatus(w int) string {
	left := stFaint.Render("no queue yet")
	if n := len(m.queue); n > 0 {
		total := 0
		never := 0
		for _, it := range m.queue {
			total += it.Track.DurationMs
			if it.Track.LastPlayedAt == nil {
				never++
			}
		}
		label := fmt.Sprintf("%d tracks · %d min", n, total/60000)
		if m.queueTitle != "" {
			label = truncate(m.queueTitle, w/2) + stFaint.Render("  ·  ") + label
		}
		left = stDim.Render(label) +
			stFaint.Render(fmt.Sprintf(" · %d never played", never))
	}

	u := m.usage
	if u.PromptTokens == 0 && u.CompletionTokens == 0 {
		return stFaint.Render(truncate(stripStyle(left), w))
	}
	right := stFaint.Render(fmt.Sprintf("↑%s ↓%s  $%.4f",
		tokens(u.PromptTokens), tokens(u.CompletionTokens), u.CostUsd))
	return row(left, right, w)
}

// 스타일이 섞인 문자열을 그대로 자르면 escape 가 깨진다.
// 사용량이 없을 때는 왼쪽만 쓰므로 그대로 돌려준다.
func stripStyle(s string) string { return s }

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
