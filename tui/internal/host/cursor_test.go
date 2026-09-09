package host

import (
	"strings"
	"testing"

	"amcli/tui/internal/musicapp"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// 커서는 터미널의 실제 커서다. 한글 조합(IME)은 앱이 아니라 터미널이
// 그 자리에 그리므로, 좌표가 한 칸이라도 어긋나면 조합 중인 글자가
// 입력창 밖에 뜬다. 화면을 실제로 세어 확인한다.
func TestCursorSitsAtInput(t *testing.T) {
	for _, c := range []struct {
		name string
		home bool
		h    int
		text string
	}{
		{name: "app", h: 32, text: "something quiet"},
		{name: "home", home: true, h: 32, text: "something quiet"},
		// 본문이 받은 높이를 다 쓰지 않는 경우 (musicapp 의 maxListRows).
		{name: "tall", h: 60, text: "something quiet"},
		{name: "short", h: 20, text: "something quiet"},
		// 한글은 표시폭이 2다. 조합이 일어나는 바로 그 입력이다.
		{name: "hangul", h: 32, text: "조용한 노래"},
		{name: "empty", h: 32, text: ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			hm := New(musicapp.New())
			if !c.home {
				hm.LeaveHome()
			}
			var m tea.Model = &hm
			m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: c.h})
			for _, r := range c.text {
				m, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
			}

			v := m.View()
			if v.Cursor == nil {
				t.Fatal("커서를 보고하지 않았다. 터미널이 조합을 그릴 자리를 모른다")
			}

			lines := strings.Split(v.Content, "\n")
			if v.Cursor.Position.Y >= len(lines) {
				t.Fatalf("커서 줄 %d 이 화면(%d줄) 밖이다", v.Cursor.Position.Y, len(lines))
			}
			line := ansi.ReplaceAllString(lines[v.Cursor.Position.Y], "")

			// 커서가 가리키는 줄은 입력창이어야 한다.
			want := strings.Repeat(" ", framePad) + "› " + c.text
			if !strings.HasPrefix(line, want) {
				t.Errorf("커서가 입력창이 아닌 줄(%d)을 가리킨다\n줄: %q", v.Cursor.Position.Y, line)
			}
			// 커서 칸은 마지막 글자 바로 다음이어야 한다.
			if got := v.Cursor.Position.X; got != lipgloss.Width(want) {
				t.Errorf("커서 칸 %d, 기대 %d\n줄: %q", got, lipgloss.Width(want), line)
			}
		})
	}
}
