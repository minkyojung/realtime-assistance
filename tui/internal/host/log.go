package host

import (
	"sort"
	"strings"

	"amcli/tui/internal/app"
	"amcli/tui/internal/style"
)

// 로그 — 호스트의 세 번째 자산이다. 입력의 짝.
//
// 한 문장이 여러 앱에 갈 수 있으므로("조용한 거 틀고 슬랙도 꺼줘")
// 대화는 어느 앱에도 속하지 않는다. 앱을 바꿔도 이어져야 하고,
// 입력이 전역이면 출력도 전역이어야 한다.
//
// 세션 동안만 들고 있는다. 저장하지 않는다.

type logEntry struct {
	who  string // "" 이면 사용자가 한 말
	text string
	err  bool
}

// waiting 은 답을 기다리는 앱 이름들이다. 스피너에 쓴다.
func (m Model) waiting() []string {
	out := make([]string, 0, len(m.pending))
	for name := range m.pending {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// 펼쳤을 때 보여줄 줄 수의 상한.
const maxLogRows = 8

// logRows 는 입력창 위에 붙일 줄들이다.
//
// 평소에는 마지막 한 줄만 보여준다. "화면에 상시로 자리를 주지 않는다"는
// 원칙 때문이고, ctrl+j 로 펼치면 더 보여준다.
func (m Model) logRows(w int) []string {
	waiting := m.waiting()
	keep := 1
	if m.logOpen {
		keep = maxLogRows
	}
	// 기다리는 동안에는 방금 한 말이 보여야 한다. 무엇에 대한 답인지
	// 모르면 스피너가 불안하기만 하다.
	if len(waiting) > 0 && !m.logOpen {
		keep = 1
	}

	out := make([]string, 0, keep+1)
	if len(m.log) > 0 {
		entries := m.log
		if len(entries) > keep {
			entries = entries[len(entries)-keep:]
		}
		for _, e := range entries {
			out = append(out, m.renderLogEntry(e, w))
		}
	}
	if len(waiting) > 0 {
		out = append(out, m.spinner.View()+" "+
			style.Dim.Render(style.Truncate(strings.Join(waiting, ", ")+" 에게 묻는 중…", w-2)))
	}
	return out
}

func (m Model) renderLogEntry(e logEntry, w int) string {
	if e.who == "" {
		return style.Faint.Render("› ") + style.Dim.Render(style.Truncate(e.text, w-2))
	}

	mark := style.Brand.Render("▸ ")
	if e.err {
		mark = style.Warn.Render("▸ ")
	}
	// 앱이 하나뿐이면 이름은 군더더기다.
	name := ""
	if len(m.apps) > 1 {
		name = padRight(e.who, 6) + " "
	}
	return mark + style.Faint.Render(name) +
		style.Dim.Render(style.Truncate(e.text, w-2-len(name)))
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

// handleSay — 앱이 한 말을 받는다.
func (m Model) handleSay(msg app.SayMsg) Model {
	delete(m.pending, msg.App)
	if strings.TrimSpace(msg.Text) != "" {
		m.log = append(m.log, logEntry{who: msg.App, text: msg.Text, err: msg.Err})
	}
	return m
}
