package host

import (
	"strings"

	"amcli/tui/internal/app"
	"amcli/tui/internal/style"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// 비밀을 받는 자리.
//
// `/ai <key>` 처럼 인자를 이어 치게 두면 API 키가 화면에 그대로 남는다.
// 시연이나 화면 공유에서는 그것이 지워지지 않는 기록이 된다 — docker 가
// `--password` 에 경고를 띄우는 것과 같은 이유다.
//
// 그래서 인자 없이 부르면 호스트가 따로 물어보고, 치는 동안 점으로 가린다.
// 인자를 붙여 친 사람은 그대로 간다. 아는 사람의 길을 막을 이유는 없다.

// 가림 문자. 한 칸짜리여야 한다 — 폭이 2인 문자를 쓰면 커서가 글자 수만큼
// 어긋난다. 커서는 터미널의 진짜 커서라 좌표가 곧 화면이다(cursor_test).
const secretDot = "•"

// asking 은 지금 비밀을 받는 중인가.
func (m Model) asking() bool { return m.secret != nil }

// askSecret 은 이 명령의 인자를 가린 채로 받기 시작한다.
func (m *Model) askSecret(c app.Command) {
	m.secret = &c
	m.input.Reset()
	m.notice = ""
	m.pick = 0
	m.showHelp = false
}

// handleSecretKey 는 비밀을 받는 동안의 키를 본다.
//
// 세 개만 가로챈다 — 나가기, 무르기, 넣기. 나머지는 입력창의 것이다.
// 여기서 모드 전환이나 팔레트를 열면 반쯤 친 키가 다른 뜻으로 읽힌다.
func (m Model) handleSecretKey(msg tea.KeyPressMsg) (bool, tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		return true, m, tea.Quit

	case key.Matches(msg, keys.Back):
		// 무르면 흔적을 안 남긴다. 친 것도 지운다.
		m.secret = nil
		m.input.Reset()
		return true, m, nil

	case key.Matches(msg, keys.Accept):
		run, val := m.secret.Run, strings.TrimSpace(m.input.Value())
		m.secret = nil
		m.input.Reset()
		if val == "" {
			return true, m, nil // 빈 채로 엔터면 그냥 물러난다
		}
		return true, m, run(val)
	}
	return false, m, nil
}

// maskInput 은 그릴 때만 값을 점으로 바꾼다.
//
// textarea 에는 가림 기능이 없다. 값을 진짜로 바꿔 두면 커서 이동·지우기·
// 붙여넣기를 전부 우리가 다시 만들어야 하므로, **모델은 진짜 값을 들고
// 있고 그림만 가린다.** m 은 값 복사본이라 여기서 고쳐도 남지 않는다.
func (m *Model) maskInput() {
	n := len([]rune(m.input.Value()))
	if n == 0 {
		return
	}
	m.input.SetValue(strings.Repeat(secretDot, n))
	m.input.CursorEnd()
}

// secretPlaceholder 는 무엇을 기다리는지와 무르는 법을 말한다.
func (m Model) secretPlaceholder() string {
	return m.secret.Name + " " + m.secret.Arg + "    " +
		style.Truncate("hidden while you type    esc  cancel", style.Max(m.inputWidth()-20, 10))
}
