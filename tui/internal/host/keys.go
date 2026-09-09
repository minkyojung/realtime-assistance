package host

import "charm.land/bubbles/v2/key"

// 키와 그 설명은 **한 값이다.**
//
// 예전에는 둘이 따로 살았다 — 스위치는 host.go 의 문자열이고, 도움말은
// overlay.go 의 표였다. 아무 관계가 없으니 한쪽만 고치면 조용히 어긋났고,
// 실제로 셋이 어긋나 있었다:
//
//	ctrl+o  도움말에만 있고 받는 코드가 없었다 — 눌러도 아무 일이 없다
//	ctrl+j  코드는 대화 띠를 접는데 설명은 딴소리였다
//	ctrl+f  절반만 하는 일을 shift+tab 과 나눠 갖고 있었다
//
// **누가 실수한 것이 아니라 어긋날 수 있게 만들어 둔 것이다.** 그래서
// 고쳐도 다음에 또 생긴다. key.Binding 은 키와 설명을 한 값에 묶고,
// 스위치도 도움말도 그 값 하나를 본다. 어긋나려면 값이 자기 자신과
// 달라야 한다.
//
// 앱의 키는 여기 없다. 앱이 자기 것을 내고(app.App.Keys) 도움말이 이어
// 붙인다 — 호스트가 남의 키를 베껴 적던 것이 ctrl+o 를 남긴 길이다.
type keyMap struct {
	Quit   key.Binding
	Help   key.Binding
	Fold   key.Binding
	Mode   key.Binding
	Back   key.Binding
	Pause  key.Binding
	Redraw key.Binding
	Up     key.Binding
	Down   key.Binding
	Accept key.Binding
}

var keys = keyMap{
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "this list"),
	),
	Fold: key.NewBinding(
		key.WithKeys("ctrl+j"),
		key.WithHelp("ctrl+j", "fold the conversation strip"),
	),
	// **ctrl+f 가 아니다.** 터미널에서 ctrl+f 는 "커서 오른쪽" 이고,
	// 우리가 쓰는 textarea 의 기본 키맵에도 그렇게 들어 있다. 호스트가
	// 그것을 먼저 가로채면 손버릇대로 누른 사람은 커서가 아니라 모드를
	// 바꾸게 되고, setMode 가 입력을 비우므로 **치던 문장이 통째로 날아간다.**
	//
	// 한때 ctrl+f 로 들어가고 shift+tab 으로 왕복했다. 같은 일에 키가
	// 둘이라 하나로 합쳤는데, 그때 남길 쪽을 잘못 골랐다 — "ctrl+f 는
	// 찾기" 는 GUI(Cmd+F) 의 관습이지 터미널의 것이 아니다.
	Mode: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "switch between ask and search"),
	),
	// 셸로 잠깐 나갔다 돌아온다. 터미널 프로그램의 기본 계약이다.
	//
	// 우리가 안 받으면 아무 일도 안 일어난다. 터미널을 raw 모드로 잡고
	// 있어서 ctrl+z 가 신호가 아니라 글자로 들어오기 때문이다 — 딴 일을
	// 하려면 앱을 통째로 끄는 수밖에 없었다.
	//
	// **재생은 안 멈춘다.** 우리가 자는 동안에도 Music.app 이 계속 튼다.
	Pause: key.NewBinding(
		key.WithKeys("ctrl+z"),
		key.WithHelp("ctrl+z", "step out to the shell — fg to come back"),
	),
	// 화면이 깨졌을 때 다시 그린다.
	//
	// 깨지는 것은 우리 잘못이 아니어도 생긴다 — 배경 프로세스가 터미널에
	// 무언가를 뱉거나, 연결이 끊겼다 붙거나. 지금까지는 껐다 켜는 것이
	// 유일한 복구였다.
	Redraw: key.NewBinding(
		key.WithKeys("ctrl+l"),
		key.WithHelp("ctrl+l", "redraw the screen"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back out one step"),
	),
	// **설명이 없다. 호스트의 것이 아니기 때문이다.**
	//
	// 여기서는 팔레트와 도움말 안에서 고를 때만 쓰고, 그 밖에서는 앱에게
	// 넘긴다. 사람이 보는 목록은 앱의 것이므로 위·아래도 앱이 설명한다
	// (musicapp/keys.go). 호스트가 적으면 남의 키를 베끼는 것이고, 그
	// 베끼기가 ctrl+o 를 남긴 길이다.
	//
	// ctrl+p·ctrl+n 은 readline 의 위·아래다. 화살표와 짝으로 두는 것이
	// 터미널의 관습이다.
	Up:   key.NewBinding(key.WithKeys("up", "ctrl+p")),
	Down: key.NewBinding(key.WithKeys("down", "ctrl+n")),
	Accept: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "send the request · play or open what is selected"),
	),
}

// notKeys 는 키가 아닌 안내다.
//
// 치는 것과 `/` 는 누르는 키가 아니라 **입력창에 무엇을 넣느냐**의 문제라
// 묶을 대상이 없다. 대신 사라질 수도 없다 — 안 묶여 있어서 생기는 어긋남이
// 여기에는 없다.
var notKeys = []struct{ key, what string }{
	{"type", "ask for a queue in your own words"},
	{"/", "commands"},
}

// helpOrder 는 도움말에 나올 차례다.
//
// 값의 목록이므로 여기 없는 것은 안 나오고, 여기 있는 것은 반드시 묶여
// 있다. 문자열을 적을 자리가 없다는 것이 이 구조의 값이다.
func (m keyMap) helpOrder() []key.Binding {
	return []key.Binding{m.Accept, m.Mode, m.Fold, m.Help, m.Redraw, m.Back, m.Pause, m.Quit}
}
