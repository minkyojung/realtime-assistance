package ui

import (
	"fmt"
	"strings"

	"amcli/tui/internal/api"
	"amcli/tui/internal/data"
	"charm.land/lipgloss/v2"
)

// S1 재생 뷰의 본문.
//
// 이 화면이 주장하는 것은 하나다 — 고른 이유를 말한다(F2).
// 곡 제목보다 근거 한 줄이 더 오래 읽혀야 한다.

func viewPlaying(s api.Session, w int) string {
	var b strings.Builder

	items := []api.QueueItem{}
	if s.QueueItems != nil {
		items = *s.QueueItems
	}
	if len(items) == 0 {
		return stDim.Render("Queue is empty")
	}

	cur := items[0]
	b.WriteString(nowPlaying(cur, w))
	b.WriteString("\n\n")
	b.WriteString(upNext(items[1:], w))

	return b.String()
}

// nowPlaying — 앨범 아트 + 곡 정보 + 선정 근거 + 진행 바
func nowPlaying(it api.QueueItem, w int) string {
	t := it.Track

	const artW = 10
	infoW := w - artW - 3
	if infoW < 10 {
		infoW = 10
	}

	// 아티스트·장르·연도가 먼저다. 앨범명은 길어서 잘리기 쉬우므로 아랫줄로 뺀다.
	meta := []string{t.Artist.Name}
	if t.Genre != nil {
		meta = append(meta, *t.Genre)
	}
	if t.Year != nil {
		meta = append(meta, fmt.Sprint(*t.Year))
	}

	lines := []string{
		stTrack.Render(truncate(t.Title, infoW)),
		stMeta.Render(truncate(strings.Join(meta, " · "), infoW)),
	}
	if t.Album != nil {
		lines = append(lines, stFaint.Render(truncate(t.Album.Title, infoW)))
	}
	lines = append(lines, "")

	// 선정 근거. 이 한 줄이 이 제품의 핵심 주장이다.
	if it.Reason != nil {
		lines = append(lines, stBrand.Render("▸ ")+stDim.Render(truncate(*it.Reason, infoW-2)))
	} else {
		// L4 실패 시 근거 없이 곡만 표시한다. 재생은 막지 않는다.
		lines = append(lines, stFaint.Render("▸ (no reason generated)"))
	}

	pos := data.PlaybackPositionMs
	timeLabel := fmt.Sprintf("  %s / %s", mmss(pos), mmss(t.DurationMs))
	barW := infoW - lipgloss.Width(timeLabel)
	if barW < 4 {
		barW = 4
	}
	lines = append(lines,
		progress(barW, float64(pos)/float64(t.DurationMs))+stDim.Render(timeLabel))

	// 아트 높이를 정보 줄 수에 맞춘다. 한쪽만 튀어나오면 눈에 띈다.
	art := albumArt(artShades, artW, len(lines))
	info := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.JoinHorizontal(lipgloss.Top, art, "   ", info)
}

// upNext — 다음 곡들. 각 줄 오른쪽에 근거를 흐리게 붙인다.
func upNext(rest []api.QueueItem, w int) string {
	var b strings.Builder
	b.WriteString(stFaint.Render("Up next"))
	b.WriteString("\n")

	for _, it := range rest {
		left := stTrackHead.Render(fmt.Sprintf("%d. ", it.Position)) +
			stBody.Render(it.Track.Title) +
			stMeta.Render(" — "+it.Track.Artist.Name)

		reason := ""
		if it.Reason != nil {
			reason = *it.Reason
		}
		// 근거는 남는 폭만큼만 보여준다. 곡 제목이 우선이다.
		space := w - lipgloss.Width(left) - 2
		if space > 12 {
			left = row(left, stFaint.Render(truncate(reason, space)), w)
		}
		b.WriteString(left)
		b.WriteString("\n")
	}

	if n := data.HiddenQueueCount; n > 0 {
		b.WriteString(stFaint.Render(fmt.Sprintf("… %d more · ctrl+o to expand", n)))
	}
	return b.String()
}
