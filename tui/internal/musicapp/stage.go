package musicapp

import (
	"strings"

	"amcli/tui/internal/app"
	"amcli/tui/internal/intent"
	"amcli/tui/internal/style"
	tea "charm.land/bubbletea/v2"
)

// 무대 — 커버 오른쪽, 진행바 아래의 자리.
//
// 그 자리를 놓고 세 가지가 다툰다. 쪼개서 나눠 갖는 대신 **한 번에 하나만
// 세운다.** 셋이 여섯 줄씩 갖는 것보다 필요할 때 열 줄을 다 쓰는 편이 낫다.
//
//	가사   기본. 듣고 있을 때
//	대화   말을 걸면. 무엇을 왜 골랐는지가 그냥 보인다
//	이력   가사가 없을 때 가사 대신 (lyrics.go)
//
// 무대는 고르는 것이 아니라 **하려던 일의 결과**다. 문장을 보내면 대화가
// 서고, esc 로 물러나면 가사로 돌아온다.
//
// 그래도 손으로 넘길 길이 있어야 한다 — 저절로만 바뀌면 "그거 좀 다시 보자"를
// 할 방법이 없다. `ctrl+j` 가 한 칸씩 넘기고(app.CycleMsg), `/lyrics` `/talk`
// 가 직행한다. 키를 앱이 만들지 않고 호스트에게서 받는 이유는 docs/07 3-1.
type stage int

const (
	stageNowPlaying stage = iota // 가사 또는 이력
	stageTalk
)

type stageMsg struct{ to stage }

// 대화 무대에 세울 것 — 방금 한 요청이 무슨 일을 했나.
//
// `ctrl+o` 로 열어 보던 그 내용이다. 자리가 있으면 열어 보게 할 이유가 없다.
type talk struct {
	prompt string
	note   string
	lines  []string // queueDetail 이 만든 것
}

// viewTalk — 대화 무대. rows 줄을 정확히 채워 돌려준다.
func (m Model) viewTalk(w, rows int) []string {
	if rows < 1 {
		return nil
	}
	out := make([]string, 0, rows)
	if m.talk.prompt == "" {
		out = append(out, style.Faint.Render(style.Truncate(
			"Nothing asked yet — type a sentence and the answer lands here", w)))
		return pad(out, rows)
	}

	// 내가 한 말이 먼저다. 무엇에 대한 답인지 모르면 답이 떠 있는 것과 같다.
	out = append(out,
		style.Faint.Render("› ")+style.Dim.Render(style.Truncate(m.talk.prompt, w-2)),
		"")
	if m.talk.note != "" {
		out = append(out, style.Brand.Render("▸ ")+style.Body.Render(style.Truncate(m.talk.note, w-2)))
	}
	if m.ask.live {
		out = append(out, m.spinner.View()+" "+style.Dim.Render("thinking…"))
		return pad(out, rows)
	}

	// 곡별 근거. `제목|근거` 로 오므로 갈라서 두 칸으로 놓는다.
	for _, l := range m.talk.lines {
		if len(out) >= rows {
			break
		}
		title, reason, split := strings.Cut(l, "|")
		if !split {
			out = append(out, "  "+style.Faint.Render(style.Truncate(l, w-2)))
			continue
		}
		out = append(out, "  "+style.Row(
			style.Dim.Render(style.Truncate(title, w/2)),
			style.Faint.Render(style.Truncate(reason, w/2-4)), w-2))
	}
	return pad(out, rows)
}

func pad(out []string, rows int) []string {
	for len(out) < rows {
		out = append(out, "")
	}
	return out[:rows]
}

// remember 는 방금의 답을 대화 무대에 담고 그 무대를 세운다.
func (m Model) rememberTalk(prompt, note string, res intent.Result) Model {
	m.talk = talk{prompt: prompt, note: note, lines: queueDetail(res)}
	m.stage = stageTalk
	return m
}

// 무대를 고르는 명령. 키를 만들지 않고 팔레트에 등록한다.
func stageCommands() []app.Command {
	return []app.Command{
		{Name: "/lyrics", Help: "lyrics of what is playing",
			Run: func(string) tea.Cmd { return send(stageMsg{to: stageNowPlaying}) }},
		{Name: "/talk", Help: "what the last request did",
			Run: func(string) tea.Cmd { return send(stageMsg{to: stageTalk}) }},
	}
}

// ── 지금 어디에 있는지 ────────────────────────────────────────────────
//
// 무대가 바뀌는데 표시가 없으면, 화면이 왜 달라졌는지 알 방법이 없다.
// 켜진 쪽만 브랜드 색이고 나머지는 회색이다 — 입력창 프롬프트가 모드를
// 말하는 것과 같은 방식이다.
func (m Model) viewStageTabs(w int) string {
	left := m.stageTab("♪ Lyrics", stageNowPlaying) + "   " + m.stageTab("▸ Asked", stageTalk)
	// 넘기는 법을 오른쪽에 적는다. 무대가 있다는 것을 아는 유일한 통로다.
	return style.Row(left, style.Faint.Render("⌃J"), w)
}

func (m Model) stageTab(label string, s stage) string {
	// 가사가 없는 곡에서는 그 자리가 이력이므로 이름도 그렇게 부른다.
	if s == stageNowPlaying && m.lyrics.Empty() {
		label = "♪ Track"
	}
	if m.stage == s {
		return style.Brand.Render(label)
	}
	return style.Faint.Render(label)
}
