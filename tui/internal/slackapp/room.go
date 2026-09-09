package slackapp

import (
	"context"
	"strings"

	"amcli/tui/internal/app"
	"amcli/tui/internal/style"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// 방 — 대화 하나를 열어 읽고 답한다.
//
// # 방에 들어간다는 것의 진짜 의미
//
// 목록에서는 입력창에 친 문장이 모델을 거친다 — "누구에게 무엇을 보낼지"를
// 판단해야 하기 때문이다(ask.go). **방 안에서는 받는 사람이 이미 정해져
// 있으므로 그 판단이 통째로 필요 없다.** 친 그대로 나간다.
//
// 그래서 방을 여는 것은 화면이 하나 늘어나는 일이 아니라, 같은 입력창이
// 다른 뜻을 갖는 일이다. 빠르고, 틀릴 여지가 없다.
//
// # esc 를 쓰지 못한다
//
// docs/07 은 esc 를 "한 단계 물러나기"로 정의하지만, 호스트가 esc 를
// 항상 자기가 먹고 앱에 넘기지 않는다(끝에서는 종료한다). 그래서
// 방에서 나오는 데 esc 를 쓰면 앱이 꺼진다.
//
// 좌우 화살표는 앱까지 내려오므로 그것을 쓴다. → 열고, ← 나온다.
// **이건 우회지 답이 아니다.** 호스트가 esc 를 앱에게 먼저 물어보게
// 되면 그때 옮긴다.

// 한 번에 읽어 오는 메시지 수. 화면에 보이는 것보다 넉넉하되,
// 스크롤백을 만들 생각은 없다 — 지난 대화를 뒤지는 것은 Slack 이 한다.
const historyLimit = 60

// openedMsg 는 방 하나의 내용이다.
type openedMsg struct {
	Channel string
	Msgs    []Message
	Err     error
}

// sentMsg 는 방 안에서 보낸 결과다.
type sentMsg struct {
	Channel string
	Err     error
}

func cmdHistory(token, channel string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
		defer cancel()
		msgs, err := newClient(token).history(ctx, channel, historyLimit)
		return openedMsg{Channel: channel, Msgs: msgs, Err: err}
	}
}

// cmdSendTo 는 정해진 방으로 친 그대로 보낸다. 모델을 거치지 않는다.
func cmdSendTo(token, channel, text string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
		defer cancel()
		return sentMsg{Channel: channel, Err: newClient(token).postMessage(ctx, channel, text)}
	}
}

// inRoom 은 방을 보고 있는지다.
func (m Model) inRoom() bool { return m.openID != "" }

// openConv 는 지금 열려 있는 대화다.
func (m Model) openConv() (Conversation, bool) { return findConv(m.convs, m.openID) }

// open 은 고른 대화를 연다. 여는 것이 곧 읽는 것이므로 안 읽음을 지운다.
func (m Model) open(c Conversation) (app.App, tea.Cmd) {
	m.openID = c.ID
	m.msgs, m.msgsErr, m.loadingMsgs = nil, nil, true

	convs := make([]Conversation, len(m.convs))
	copy(convs, m.convs)
	for i := range convs {
		if convs[i].ID == c.ID {
			convs[i].Unread = 0
		}
	}
	m.convs = convs
	return m, cmdHistory(m.token(), c.ID)
}

// close 는 목록으로 돌아간다. 읽은 것은 버린다 — 다시 열면 다시 받는다.
func (m Model) close() Model {
	m.openID, m.msgs, m.msgsErr, m.loadingMsgs = "", nil, nil, false
	return m
}

// receiveInRoom 은 밀려 들어온 메시지를 열려 있는 방에도 붙인다.
// 보고 있는 방에서 온 것이면 안 읽음으로 세지 않는다 — 이미 보고 있다.
func (m Model) receiveInRoom(in incomingMsg) Model {
	msgs := make([]Message, len(m.msgs), len(m.msgs)+1)
	copy(msgs, m.msgs)
	m.msgs = append(msgs, Message{User: in.User, Text: in.Text, At: in.At})
	return m
}

// unknownSenders 는 아직 이름을 모르는 사람들이다.
// 방을 열 때 한 번에 물어본다 — DM 이면 보통 한 명이다.
func (m Model) unknownSenders() []tea.Cmd {
	seen := map[string]bool{}
	var cmds []tea.Cmd
	for _, msg := range m.msgs {
		if msg.User == "" || msg.User == m.me || seen[msg.User] {
			continue
		}
		seen[msg.User] = true
		if _, known := m.users[msg.User]; !known {
			cmds = append(cmds, cmdUserName(m.token(), msg.User))
		}
	}
	return cmds
}

// ─────────────────────────────────────────────────────────────
// 화면
// ─────────────────────────────────────────────────────────────

// 메시지 한 줄의 칸. 좁아지면 시각 → 보낸 사람 순으로 사라지고,
// 본문은 절대 안 버린다 — 본문이 없으면 그 줄은 아무 뜻이 없다.
var msgCols = []style.Col{
	{Min: 5, Drop: 2},          // 시각
	{Min: 6, Max: 14, Drop: 1}, // 보낸 사람
	{Min: 16, Weight: 1},       // 본문
}

func (m Model) viewRoom(w, h int) string {
	conv, _ := m.openConv()

	var b strings.Builder
	b.WriteString(style.Row(
		style.Title.Render(style.Truncate(conv.Name, style.Max(w-12, 8))),
		style.Faint.Render("← 목록"), w))
	b.WriteString("\n")
	b.WriteString(style.RuleBrand(w))
	b.WriteString("\n")

	bodyH := style.Max(h-3, 1)
	b.WriteString(m.viewMessages(w, bodyH))
	b.WriteString("\n")
	b.WriteString(style.Rule(w))
	return fit(b.String(), h)
}

func (m Model) viewMessages(w, h int) string {
	if note := m.roomNote(); note != "" {
		return lipgloss.NewStyle().Width(w).Height(h).Render(style.Faint.Render(note))
	}

	widths := style.Columns(w, colGap, msgCols)
	gap := strings.Repeat(" ", colGap)

	// 줄을 전부 만든 뒤 **끝에서** h 줄만 남긴다.
	// 대화는 아래가 현재이므로 잘려야 하는 쪽은 위다.
	var lines []string
	prev := ""
	for _, msg := range m.msgs {
		lines = append(lines, m.renderMessage(msg, widths, gap, &prev)...)
	}
	if len(lines) > h {
		lines = lines[len(lines)-h:]
	}
	// 모자라면 **위를** 비운다. 대화는 아래가 현재이고, 입력창이 바로
	// 아래 있으므로 방금 온 말과 답하는 자리가 붙어 있어야 한다.
	for len(lines) < h {
		lines = append([]string{""}, lines...)
	}
	return strings.Join(lines, "\n")
}

// roomNote 는 메시지 대신 보여줄 한 줄이다. 없으면 빈 문자열.
func (m Model) roomNote() string {
	switch {
	case m.msgsErr != nil:
		return m.scopeNote()
	case m.loadingMsgs:
		return "읽는 중…"
	case len(m.msgs) == 0:
		return "아직 아무 말도 없습니다"
	}
	return ""
}

// renderMessage 는 한 메시지를 여러 줄로 편다.
//
// 같은 사람이 이어 말하면 이름을 다시 쓰지 않는다. 카톡에서 프로필이
// 한 번만 붙는 것과 같은 이유고, 좁은 터미널에서는 그 절약이 크다.
func (m Model) renderMessage(msg Message, widths []int, gap string, prev *string) []string {
	who := m.senderName(msg)
	body := wrap(msg.Text, style.Max(widths[2], 1))
	if len(body) == 0 {
		return nil
	}

	head := ""
	if widths[0] > 0 {
		at := ""
		if !msg.At.IsZero() {
			at = msg.At.Format("15:04")
		}
		head += style.Faint.Render(rpad(at, widths[0])) + gap
	}
	if widths[1] > 0 {
		name := who
		if name == *prev {
			name = "" // 이어지는 같은 사람은 비워 둔다
		}
		cell := pad(name, widths[1])
		if msg.User != "" && msg.User == m.me {
			head += style.BrandSoft.Render(cell) + gap
		} else {
			head += style.Dim.Render(cell) + gap
		}
	}
	*prev = who

	// lipgloss.Width 는 이스케이프를 무시하므로 색이 든 채로 재도 된다.
	indent := strings.Repeat(" ", lipgloss.Width(head))
	out := make([]string, 0, len(body))
	for i, line := range body {
		if i == 0 {
			out = append(out, head+style.Body.Render(line))
			continue
		}
		out = append(out, indent+style.Body.Render(line))
	}
	return out
}

func (m Model) senderName(msg Message) string {
	if msg.User != "" {
		if name := m.users[msg.User]; name != "" {
			return name
		}
		if msg.User == m.me {
			return "나"
		}
		return msg.User
	}
	if msg.Name != "" {
		return msg.Name
	}
	return "?"
}

// wrap 은 표시폭 기준으로 접는다. 한글 한 글자는 2칸을 먹는다.
//
// 슬랙 메시지에는 줄바꿈이 흔하므로 문단을 먼저 나누고 각각을 접는다.
func wrap(s string, width int) []string {
	if width <= 0 {
		return nil
	}
	var out []string
	for _, para := range strings.Split(s, "\n") {
		fields := strings.Fields(para)
		if len(fields) == 0 {
			continue // 빈 줄로 화면을 먹지 않는다
		}
		line := ""
		for _, word := range fields {
			cand := word
			if line != "" {
				cand = line + " " + word
			}
			if lipgloss.Width(cand) <= width {
				line = cand
				continue
			}
			if line != "" {
				out = append(out, line)
				line = ""
			}
			// 한 낱말이 폭보다 길면(URL 등) 잘라서 이어 붙인다.
			for lipgloss.Width(word) > width {
				cut := ""
				for _, r := range word {
					if lipgloss.Width(cut+string(r)) > width {
						break
					}
					cut += string(r)
				}
				if cut == "" {
					return append(out, word) // 한 글자도 안 들어간다. 더 할 것이 없다
				}
				out = append(out, cut)
				word = word[len(cut):]
			}
			line = word
		}
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// scopeNote 는 왜 못 읽는지를 말한다.
//
// **"권한이 모자랍니다"만으로는 아무것도 못 한다.** 어느 권한인지 이름을
// 대야 사용자가 앱 설정에서 그것을 찾을 수 있다.
func (m Model) scopeNote() string {
	if !missingScope(m.msgsErr) {
		return m.msgsErr.Error()
	}
	conv, ok := m.openConv()
	scope := historyScopes[conv.Kind]
	if !ok || scope == "" {
		return m.msgsErr.Error()
	}
	return scope + " 권한이 없어 못 읽습니다. 앱 설정에 넣고 /logout → /login 하세요"
}

// 방을 보고 있을 때의 안내 한 줄.
func (m Model) roomHint() string {
	if m.msgsErr != nil {
		return "DM 은 지금도 열립니다 — ← 로 목록에서 @ 로 시작하는 것을 고르세요"
	}
	return "친 그대로 보냅니다  ·  ← 목록으로"
}
