package musicapp

import (
	"amcli/tui/internal/music"

	"charm.land/bubbles/v2/key"
)

// 이 앱이 받는 키. 설명이 같은 값에 붙어 있다.
//
// 호스트의 도움말이 이것을 그대로 그린다(app.App.Keys). 예전에는 호스트가
// 우리 키를 문자열로 베껴 적었고, 우리가 키를 바꿔도 호스트는 몰랐다.
var keys = struct {
	Section        key.Binding
	Up, Down       key.Binding
	PlayPause      key.Binding
	Next, Previous key.Binding
	Settings       key.Binding
	Accept         key.Binding
}{
	// 다음 칸으로만 간다.
	//
	// tab 과 shift+tab 은 어디서나 한 쌍이라 되돌아가는 키를 잠깐 붙였는데,
	// shift+tab 은 호스트가 모드를 왕복하는 데 쓴다. 한 자리를 두고 다투면
	// **모드 전환이 훨씬 자주 쓰인다.** 먼 섹션에는 `/` 로 곧장 간다.
	Section: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next section — / goes straight to one"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑ · ctrl+p", "move up the list"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓ · ctrl+n", "move down the list"),
	),
	// 재생 제어는 shift+화살표 한 가족이다. 수식키+화살표라 입력창도
	// 한글 조합도 건드리지 않는다 — 알파벳이나 space 를 쓸 수 없는 이유가
	// 그것이다(docs/07 2절).
	PlayPause: key.NewBinding(
		key.WithKeys("shift+down"),
		key.WithHelp("shift+↓", "play / pause"),
	),
	Previous: key.NewBinding(
		key.WithKeys("shift+left"),
		key.WithHelp("shift+←", "previous track"),
	),
	Next: key.NewBinding(
		key.WithKeys("shift+right"),
		key.WithHelp("shift+→", "next track"),
	),
	// enter 는 호스트가 먼저 보고 남는 것만 넘어온다(handleEnter). 설명은
	// 호스트가 갖는다 — 같은 키를 두 곳에서 설명하면 도움말에 두 번 나온다.
	Accept: key.NewBinding(key.WithKeys("enter")),

	// 권한이 막혔을 때만 뜻이 있다. 그때만 도움말에도 낸다(Keys).
	Settings: key.NewBinding(
		key.WithKeys("ctrl+g"),
		key.WithHelp("ctrl+g", "open System Settings › Privacy & Security › Automation"),
	),
}

// Keys 는 호스트의 도움말에 나갈 것들이다.
//
// **지금 뜻이 있는 것만 낸다.** ctrl+g 는 권한이 막혔을 때만 무언가를
// 하므로, 평소에 도움말에 적어 두면 눌러도 아무 일이 없는 키가 된다 —
// ctrl+o 가 그렇게 남았다.
//
// 위·아래는 여기서 설명한다. 호스트도 같은 키를 쓰지만 그것은 팔레트
// 안에서고, **사람이 보는 목록은 우리 것이다.**
func (m Model) Keys() []key.Binding {
	out := []key.Binding{
		keys.Up, keys.Down,
		keys.Section,
		keys.Previous, keys.PlayPause, keys.Next,
	}
	if m.playerErr == music.ErrPermissionDenied {
		out = append(out, keys.Settings)
	}
	return out
}
