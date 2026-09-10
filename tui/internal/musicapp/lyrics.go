package musicapp

import (
	"context"
	"fmt"
	"time"

	"amcli/tui/internal/lyrics"
	"amcli/tui/internal/style"
	tea "charm.land/bubbletea/v2"
)

// 가사는 커버 오른쪽, 재생 정보 아래에 앉는다.
//
// 애플에서는 못 가져온다(internal/lyrics 머리말). LRCLIB 에서 받는데
// 실측 커버리지가 싱크 68% · 줄글 23% · 없음 8% 라, **셋 다 화면이
// 있어야 한다.** 없을 때 자리가 휑해지면 있을 때의 값까지 깎아먹는다.
//
//	싱크 있음  지금 줄을 가운데 두고 위아래로 흘린다
//	줄글만     처음 몇 줄을 그대로. 흐르지 않는다
//	없음       곡 이력이 그 자리를 쓴다 (nowPlayingFacts)

type lyricsMsg struct {
	pid string
	l   *lyrics.Lyrics
}

func cmdLyrics(pid, artist, title, album string, durationMs int) tea.Cmd {
	return func() tea.Msg {
		l, err := lyrics.Fetch(context.Background(), artist, title, album, durationMs)
		if err != nil {
			return lyricsMsg{pid: pid} // 없는 것도 답이다
		}
		return lyricsMsg{pid: pid, l: l}
	}
}

// nowMs — 지금 이 순간의 재생 위치.
//
// 폴링은 1초에 한 번이라 그대로 쓰면 가사가 최대 1초 늦게 넘어간다.
// 노래에서 1초는 티가 난다. 마지막으로 받은 위치에 그 뒤로 흐른 시간을
// 더해서 메운다 — 다음 폴링이 오면 다시 맞춰진다.
//
// 화면은 호스트의 스피너 덕분에 초당 열 번 다시 그려지므로, 이 계산이
// 그 주기로 반영된다.
func (m Model) nowMs() int {
	if !m.playing || m.polledAt.IsZero() {
		return m.positionMs
	}
	return m.positionMs + int(time.Since(m.polledAt).Milliseconds())
}

// viewLyrics — 가사 칸. rows 줄을 정확히 채워 돌려준다.
//
// 지금 줄을 가운데에 두고 위아래를 채운다. 위로 붙이면 노래가 진행될수록
// 눈이 아래로 내려가야 하고, 그러면 시선이 한 줄에 머물지 못한다.
func (m Model) viewLyrics(w, rows int) []string {
	if rows < 1 {
		return nil
	}
	out := make([]string, 0, rows)
	switch {
	case m.lyrics.Synced():
		cur := m.lyrics.At(time.Duration(m.nowMs()) * time.Millisecond)
		top := cur - (rows-1)/2
		for i := 0; i < rows; i++ {
			out = append(out, m.lyricLine(top+i, cur, w))
		}
	case m.lyrics != nil && len(m.lyrics.Plain) > 0:
		// 시각이 없으면 흐르게 할 근거가 없다. 처음 몇 줄을 그대로 둔다.
		for i := 0; i < rows; i++ {
			if i < len(m.lyrics.Plain) {
				out = append(out, style.Faint.Render(style.Truncate(m.lyrics.Plain[i], w)))
				continue
			}
			out = append(out, "")
		}
	default:
		out = append(out, m.nowPlayingFacts(w, rows)...)
	}
	return out[:rows]
}

func (m Model) lyricLine(i, cur, w int) string {
	if i < 0 || i >= len(m.lyrics.Lines) {
		return ""
	}
	text := m.lyrics.Lines[i].Text
	if text == "" {
		return "" // 간주. 빈 줄이 그대로 쉼이 된다
	}
	if i == cur {
		return style.Brand.Render("▸ ") + style.Body.Render(style.Truncate(text, w-2))
	}
	// 지나간 줄과 올 줄을 구별하지 않는다. 지금 줄 하나만 밝으면 된다.
	return "  " + style.Faint.Render(style.Truncate(text, w-2))
}

// nowPlayingFacts — 가사가 없을 때 그 자리를 채우는 것.
//
// 비워 두면 커버만 덩그러니 남아 화면이 고장 난 것처럼 보인다. 대신
// **우리가 확실히 아는 것**을 놓는다 — 언제 담았고 몇 번 들었고 마지막이
// 언제였는지. 이 서비스가 하려는 말("담아두고 안 듣는다")이 그 숫자다.
func (m Model) nowPlayingFacts(w, rows int) []string {
	t, ok := m.nowPlaying()
	if !ok {
		return make([]string, rows)
	}
	var facts []string
	add := func(k, v string) {
		if v == "" {
			return
		}
		facts = append(facts, style.Faint.Render(k)+style.Dim.Render(v))
	}

	if t.Genre != nil && *t.Genre != "" {
		year := ""
		if t.Year != nil && *t.Year > 0 {
			year = fmt.Sprintf(" · %d", *t.Year)
		}
		add("", *t.Genre+year)
	}
	if t.AddedAt != nil {
		add("added      ", t.AddedAt.Format("Jan 2006")+ago(*t.AddedAt))
	}
	if t.LastPlayedAt == nil {
		// 이 줄이 이 서비스가 주장하는 문제 자체다. 회색으로 두지 않는다.
		facts = append(facts, style.Warn.Render("never played"))
	} else {
		add("last played", t.LastPlayedAt.Format("Jan 2 2006")+ago(*t.LastPlayedAt))
		add("play count ", fmt.Sprint(t.PlayCount))
	}

	out := make([]string, 0, rows)
	// 위쪽에 한 줄 띄운다. 진행바에 딱 붙으면 진행바의 일부로 읽힌다.
	out = append(out, "")
	for _, f := range facts {
		if len(out) >= rows {
			break
		}
		out = append(out, style.Truncate(f, w))
	}
	for len(out) < rows {
		out = append(out, "")
	}
	return out[:rows]
}

func ago(t time.Time) string {
	d := int(time.Since(t).Hours() / 24)
	switch {
	case d <= 0:
		return "  today"
	case d == 1:
		return "  yesterday"
	case d < 30:
		return fmt.Sprintf("  %d days ago", d)
	case d < 365:
		return fmt.Sprintf("  %d months ago", d/30)
	}
	return fmt.Sprintf("  %d years ago", d/365)
}
