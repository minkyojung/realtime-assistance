package ui

import (
	"fmt"
	"strings"

	"amcli/tui/internal/data"
	tea "charm.land/bubbletea/v2"
)

// 슬래시 명령.
//
// `/` 를 치면 목록 패널이 명령 목록으로 바뀐다. 검색과 같은 방식이라
// 화면이 늘어나지 않고, 무엇을 칠 수 있는지가 눈앞에 보인다.
//
// **명령은 모델을 거치지 않는다.** 이것이 체감 속도를 지탱하는 장치다.

type command struct {
	name string
	arg  string
	help string
}

var commands = []command{
	{"/queue", "", "jump to the current queue"},
	{"/unplayed", "", "tracks you added but never played"},
	{"/save", "<name>", "save the queue as an Apple Music playlist"},
	{"/clear", "", "empty the queue"},
	{"/cost", "", "what this session has spent"},
	{"/help", "", "keys and commands"},
}

// commanding — 입력이 `/` 로 시작하면 명령 모드다. 별도 상태를 두지 않는다.
func (m Model) commanding() bool {
	return m.mode == modePrompt && strings.HasPrefix(m.input.Value(), "/")
}

// 입력한 것으로 시작하는 명령만 남긴다.
func (m Model) matchedCommands() []command {
	typed := strings.ToLower(strings.Fields(m.input.Value() + " ")[0])
	out := make([]command, 0, len(commands))
	for _, c := range commands {
		if strings.HasPrefix(c.name, typed) {
			out = append(out, c)
		}
	}
	return out
}

func (m Model) renderCommand(c command, selected bool, w int) string {
	rail := "  "
	if selected {
		rail = stBrand.Render("▌ ")
	}
	left := stBody.Render(c.name)
	if c.arg != "" {
		left += stFaint.Render(" " + c.arg)
	}
	return rail + row(left, stFaint.Render(c.help), w-2)
}

// runCommand 는 명령을 실행한다. 모르는 명령은 조용히 무시하지 않고 말한다.
func (m Model) runCommand(line string) (Model, tea.Cmd) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return m, nil
	}
	name, arg := fields[0], strings.TrimSpace(strings.TrimPrefix(line, fields[0]))

	// 목록에서 골라 실행한 경우, 인자가 필요한 명령은 이름만 남겨 둔다.
	m.input.Reset()
	m.intentErr = nil

	switch name {
	case "/queue":
		m.jumpTo(secQueue)
	case "/unplayed":
		m.jumpTo(secUnplayed)
	case "/clear":
		m.queue = nil
		m.queueTitle, m.note = "", ""
		m.jumpTo(secRecent)
	case "/cost":
		m.notice = m.costLine()
	case "/help":
		m.showHelp = true
	case "/save":
		if arg == "" {
			m.input.SetValue("/save ")
			m.input.CursorEnd()
			return m, nil
		}
		if len(m.queue) == 0 {
			m.intentErr = errNoQueue
			return m, nil
		}
		return m, cmdSavePlaylist(arg, m.queueTracks())
	default:
		m.intentErr = fmt.Errorf("모르는 명령입니다: %s", name)
	}
	return m, nil
}

func (m *Model) jumpTo(kind sectionKind) {
	for i, s := range m.sections {
		if s.kind == kind {
			m.sectionIdx = i
			m.listIdx, m.listTop = 0, 0
			return
		}
	}
}

func (m Model) costLine() string {
	u := m.usage
	if u.PromptTokens == 0 && u.CompletionTokens == 0 {
		return "No requests yet this session"
	}
	return fmt.Sprintf("%d in · %d out · $%.4f this session",
		u.PromptTokens, u.CompletionTokens, u.CostUsd)
}

// 도움말은 목록 패널을 잠시 빌려 쓴다. 오버레이를 따로 만들지 않는다.
var helpRows = []struct{ key, what string }{
	{"type", "ask for a queue in your own words"},
	{"enter", "send the request · play the selected track"},
	{"/", "commands"},
	{"ctrl+f", "search your library"},
	{"↑ ↓", "move through the list"},
	{"tab", "next section"},
	{"shift+← →", "previous / next track"},
	{"esc", "back out one step"},
	{"ctrl+c", "quit"},
}

func (m Model) renderHelpRow(i int, w int) string {
	r := helpRows[i]
	return "  " + row(stBrandSoft.Render(r.key), stFaint.Render(r.what), w-2)
}

// UnplayedTracks — 담아두고 한 번도 재생하지 않은 곡.
// 이 서비스가 말하려는 것이 목록 하나로 보이는 자리다.
func unplayedTracks(l *data.Library) []listRow {
	var out []listRow
	for _, t := range l.Songs() {
		if t.PlayCount == 0 && t.LastPlayedAt == nil {
			tt := t
			out = append(out, listRow{track: &tt})
		}
	}
	return out
}
