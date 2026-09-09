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
	who    string // "" 이면 사용자가 한 말
	text   string
	err    bool
	detail []string // ctrl+o 로 펼쳤을 때 보일 줄들
}

// waiting 은 답을 기다리는 앱 이름들이다. 스피너에 쓴다.
func (m Model) waiting() []string {
	if m.routing {
		return []string{"…"}
	}
	out := make([]string, 0, len(m.pending))
	for name := range m.pending {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// 대화 띠가 쓸 수 있는 줄 수의 상한. 넘으면 오래된 것부터 잘린다.
const maxLogRows = 10

// logRows 는 입력창 바로 위에 붙는 대화 띠다.
//
// **마지막 한 판을 통째로 보여준다** — 내가 한 말, 앱의 답, 곡별 근거,
// 비용까지. 한때 이것이 한 줄이었고 근거는 ctrl+o 로 열어야 보였다.
// 자리가 없어서였는데, 목록 상한을 걷어내고 배치를 정리하니 자리가 생겼다.
//
// 입력창 **바로 위**인 것이 요점이다. 답이 화면 꼭대기에 뜨면 바닥에서 치고
// 꼭대기에서 읽느라 눈이 왕복한다. agentic CLI 가 전부 입력창 위에서
// 답을 키우는 이유다.
//
// 아무것도 안 물어봤으면 **한 줄도 그리지 않는다.** 그 자리는 목록이 쓴다.
func (m Model) logRows(w int) []string {
	waiting := m.waiting()
	if m.logShut {
		// 접었으면 마지막 한 줄만. 목록을 더 보고 싶을 때다.
		if len(m.log) == 0 {
			return m.spinnerRows(waiting, w)
		}
		return append([]string{m.renderLogEntry(m.log[len(m.log)-1], w)},
			m.spinnerRows(waiting, w)...)
	}

	out := make([]string, 0, maxLogRows)
	for _, e := range m.lastExchange() {
		out = append(out, m.renderLogEntry(e, w))
		if len(e.detail) > 0 {
			out = append(out, m.renderDetail(e.detail, w)...)
		}
	}
	// 넘치면 앞을 자른다. 방금 온 답이 잘리면 안 된다.
	if len(out) > maxLogRows {
		out = out[len(out)-maxLogRows:]
	}
	return append(out, m.spinnerRows(waiting, w)...)
}

// lastExchange — 마지막으로 내가 한 말과, 그 뒤에 온 답들.
//
// 한 문장이 여러 앱에 갈 수 있으므로 답이 여럿일 수 있다. 그 앞의 판은
// 지나간 것이다 — 필요하면 ctrl+j 로 접었다 펴는 대신 위로 스크롤하는
// 것이 맞지만, 세션 기록을 화면에 쌓지 않기로 했으므로 한 판만 둔다.
func (m Model) lastExchange() []logEntry {
	for i := len(m.log) - 1; i >= 0; i-- {
		if m.log[i].who == "" {
			return m.log[i:]
		}
	}
	return m.log
}

func (m Model) spinnerRows(waiting []string, w int) []string {
	if len(waiting) == 0 {
		return nil
	}
	label := strings.Join(waiting, ", ") + " 에게 묻는 중…"
	if m.routing {
		label = "어디로 보낼지 정하는 중…"
	}
	return []string{m.spinner.View() + " " + style.Dim.Render(style.Truncate(label, w-2))}
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

// 상세는 들여쓰고 흐리게. 본문이 아니라 주석이라는 뜻이다.
//
// "제목|근거" 로 온 줄은 두 칸으로 나눠 앉힌다 — 좁아지면 근거부터 접힌다.
func (m Model) renderDetail(lines []string, w int) []string {
	const indent = "    "
	inner := style.Max(w-len(indent), 20)
	cols := style.Columns(inner, 2, []style.Col{
		{Min: 12, Weight: 2, Max: 40},
		{Min: 16, Weight: 3, Drop: 1},
	})

	out := make([]string, 0, len(lines))
	for _, l := range lines {
		left, right, split := strings.Cut(l, "|")
		if !split {
			out = append(out, indent+style.Faint.Render(style.Truncate(l, inner)))
			continue
		}
		cell := style.Truncate(left, cols[0])
		if d := cols[0] - len(cell); d > 0 {
			cell += strings.Repeat(" ", d)
		}
		row := style.Faint.Render(cell)
		if cols[1] > 0 {
			row += "  " + style.Faint.Render(style.Truncate(right, cols[1]))
		}
		out = append(out, indent+row)
	}
	return out
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

// cancelPending — 기다리던 것을 그만둔다.
//
// 배경에서 계속 돌게 두지 않는다. 취소했는데 잠시 뒤 큐가 통째로 갈리면
// 그것이 제일 나쁜 상태다. 그래서 앱에게도 알려 실제 요청을 끊게 한다.
func (m Model) cancelPending() Model {
	for name := range m.pending {
		for i, a := range m.apps {
			if a.Name() == name {
				next, _ := a.Update(app.CancelMsg{})
				m.apps[i] = next
			}
		}
		delete(m.pending, name)
	}
	// 라우터의 답도 버린다. 번호를 올려두면 늦게 와도 걸러진다(host.go).
	if m.routing {
		m.routeSeq++
		m.routing = false
	}
	m.log = append(m.log, logEntry{who: "host", text: "그만뒀습니다"})
	return m
}

// handleSay — 앱이 한 말을 받는다.
func (m Model) handleSay(msg app.SayMsg) Model {
	delete(m.pending, msg.App)
	if strings.TrimSpace(msg.Text) != "" {
		m.log = append(m.log, logEntry{
			who: msg.App, text: msg.Text, err: msg.Err, detail: msg.Detail,
		})
	}
	return m
}
