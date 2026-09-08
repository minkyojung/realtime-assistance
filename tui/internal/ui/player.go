package ui

import (
	"fmt"

	"amcli/tui/internal/api"
	"charm.land/lipgloss/v2"
)

// 하단 재생 바. 어떤 목록을 보고 있든 항상 같은 자리에 있다.
// 화면에서 유일하게 절대 바뀌지 않는 요소이므로 여기가 기준점이 된다.

func (m Model) viewPlayer(w int) string {
	t, ok := m.nowPlaying()
	if !ok {
		return stFaint.Render(truncate("Nothing playing · type what you want to hear below", w))
	}

	icon := stBrand.Render("▶")
	if !m.playing {
		icon = stFaint.Render("❚❚")
	}

	label := stBody.Render(truncate(t.Title, 28)) +
		stFaint.Render(" — "+truncate(t.Artist.Name, 20))

	pos := clamp(m.positionMs, 0, t.DurationMs)
	timeLabel := fmt.Sprintf(" %s / %s", mmss(pos), mmss(t.DurationMs))
	left := icon + "  " + label

	barW := w - lipgloss.Width(left) - lipgloss.Width(timeLabel) - 3
	if barW < 8 {
		return row(left, stFaint.Render(timeLabel), w)
	}

	ratio := 0.0
	if t.DurationMs > 0 {
		ratio = float64(pos) / float64(t.DurationMs)
	}
	return left + "  " + progress(barW, ratio) + stFaint.Render(timeLabel)
}

// 선정 근거 줄. 의도 층이 만든 것이므로 재생 바 바로 위에 붙인다.
// 근거가 없으면 줄 자체가 없다 — 빈 줄을 남기지 않는다.
func (m Model) viewReason(w int) (string, bool) {
	it, ok := m.nowPlayingItem()
	if !ok || it.Reason == nil || *it.Reason == "" {
		return "", false
	}
	return stBrand.Render("▸ ") + stDim.Render(truncate(*it.Reason, w-2)), true
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
