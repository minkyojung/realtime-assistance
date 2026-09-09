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

// 곡별 근거는 본문에 두지 않는다.
//
// 8곡짜리 큐에서 근거 8개를 다 읽는 사람은 없고, 한 줄로 하나만 보여주면
// 나머지 일곱은 어차피 안 보인다. 큐 전체를 설명하는 한 문장은 이미
// 로그에 있으므로(app.Say), 곡마다의 근거는 로그를 펼쳤을 때 볼 것으로 미룬다.
//
// 데이터는 그대로 남아 있다 — queue_item.reason.

// Status 는 상태줄에 들어갈 한 줄이다. 배경에 있어도 호출된다.
//
// 사이드바를 없앴으므로 **"지금 어디인지"를 말해줄 곳이 여기밖에 없다.**
// 갈 수 있는 곳은 `/` 가 보여주므로 여기서는 말하지 않는다.
func (m Model) Status() string {
	where := m.sections[m.sectionIdx].label
	// 파고들었으면 어디서 들어왔는지까지 말한다. 이름만 쓰면 같은 이름의
	// 플레이리스트와 구별이 안 된다.
	if m.drill != nil {
		where += " › " + m.drill.Name
	}
	if m.searching() {
		where = "Search"
	}
	left := style.Dim.Render(where) +
		style.Faint.Render(fmt.Sprintf(" · %d", m.rowCount()))

	// 빈 목록이 왜 비었는지를 목록 대신 여기가 말한다.
	// 한 번이라도 읽은 뒤에는 말하지 않는다 — 다시 읽는 것은 티가 안 나야 한다.
	if !m.synced && m.rowCount() == 0 {
		left += style.Faint.Render("   ⟳ syncing")
	}

	if n := len(m.queue); n > 0 && m.sections[m.sectionIdx].kind != secQueue {
		left += style.Faint.Render(fmt.Sprintf("   queue %d", n))
	}
	if m.sections[m.sectionIdx].kind == secCatalog && m.catTerm != "" {
		left += style.Faint.Render("   " + style.Truncate("\""+m.catTerm+"\"", 30))
	}
	if m.catBusy {
		left += style.Faint.Render("   searching…")
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

// viewCatalogHint — 카탈로그를 보고 있을 때 근거 자리를 이 문장이 쓴다.
//
// 관문 > 카탈로그 > 근거 순이다. 셋이 동시에 뜨는 일은 없으므로
// 세로 예산은 그대로다.
func (m Model) viewCatalogHint(w int) (string, bool) {
	if m.catLogin {
		return m.spinner.View() + " " +
			style.Dim.Render("Waiting for you to approve in the browser… (up to 3 minutes)"), true
	}
	if m.adding != nil {
		return style.Brand.Render("▸ ") + style.Dim.Render(style.Truncate(
			"Adding \""+m.adding.track.Title+"\" — will play once it shows up in Music", w-2)), true
	}
	if m.sectionIdx >= len(m.sections) || m.sections[m.sectionIdx].kind != secCatalog {
		return "", false
	}
	return style.Faint.Render(style.Truncate(
		"Not in your library yet · enter adds it and plays", w)), true
}
