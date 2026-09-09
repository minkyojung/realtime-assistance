// Package app 은 호스트와 앱 사이의 계약이다.
//
// 호스트는 두 줄만 갖는다 — 입력창과 상태줄. 그 위는 전부 앱의 것이다.
// 그래서 앱마다 화면이 달라도 된다. 음악은 사이드바와 목록을 그리고,
// 채팅 앱은 대화를 그리면 된다.
package app

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// App 은 호스트가 담을 수 있는 하나의 앱이다.
//
// 설계 근거는 전부 실측이나 조사에서 나왔다.
//
//   - Update 가 tea.Model 이 아니라 App 을 돌려준다.
//     tea.Model 을 돌려주면 호스트가 메시지마다 타입 단언을 해야 한다.
//     Bubble Tea 쪽에서 반복적으로 지적되는 함정이다.
//
//   - View 가 크기를 인자로 받는다.
//     WindowSizeMsg 를 자식들에게 전달하고 각자 기억하게 하면 상태가 늘고
//     어긋나기 쉽다. 레이아웃은 호스트가 아니까 그릴 때 알려주면 된다.
//
//   - Init 이 send 를 받는다.
//     Music.app 은 우리가 물어보지만(폴링), 채팅은 저쪽에서 밀어 넣는다.
//     이벤트 루프 바깥에서 메시지를 넣을 통로가 없으면 그런 앱을 담을 수 없다.
type App interface {
	// Name 은 사이드바·상태줄·라우터가 쓰는 짧은 이름이다. "music" 처럼.
	Name() string

	// Description 은 라우터가 읽는 한 줄이다.
	// 어떤 문장이 이 앱의 것인지 판단하는 유일한 근거이므로 구체적으로 쓴다.
	Description() string

	// Tagline 은 사람이 읽는 한 줄이다. 첫 화면에서 이름 옆에 붙는다.
	//
	// Description 과 나누는 이유는 독자가 다르기 때문이다. 저쪽은 모델이
	// 읽으므로 빠짐없이 구체적이어야 하고, 이쪽은 사람이 읽으므로 짧아야
	// 한다. 한 문장에 둘을 담으면 라우팅이 나빠지거나 첫 화면이 길어진다.
	Tagline() string

	// Init 은 앱을 시작한다. send 는 이벤트 루프 바깥에서 메시지를 넣는 통로다.
	Init(send func(tea.Msg)) tea.Cmd

	// Update 는 호스트가 자기 몫을 처리한 뒤 남은 메시지를 넘긴다.
	Update(tea.Msg) (App, tea.Cmd)

	// View 는 본문을 그린다. 주어진 크기를 넘지 않아야 한다.
	View(width, height int) string

	// Status 는 상태줄에 들어갈 한 줄이다.
	// 배경에 있어도 호출되므로, 다른 앱을 보는 중에도 이 앱이 뭘 하는지 보인다.
	Status() string

	// Spend 는 이 앱이 쓴 것이다 — 토큰과 돈. 안 썼으면 빈 문자열.
	//
	// Status 와 나누는 이유는 내용이 달라서가 아니라 **자리가 달라서**다.
	// 상태줄은 좁아지면 왼쪽의 끝부터 잘리는데, 한 줄로 이어 붙이면 제일
	// 뒤에 있던 비용이 제일 먼저 조용히 사라진다. 쓴 돈이 안 보이는 것은
	// 안 쓴 것처럼 보이는 것과 같으므로, 호스트가 이것만 오른쪽 끝에 둔다.
	Spend() string

	// Keys 는 이 앱이 받는 키와 그 설명이다.
	//
	// **호스트의 도움말이 이것으로 그려진다.** 예전에는 호스트가 앱의 키를
	// 문자열로 베껴 적었다. 앱이 키를 바꿔도 호스트는 몰랐고, 도움말은
	// 아무도 안 누르는 키를 계속 약속했다 — ctrl+o 가 그렇게 남았다.
	//
	// key.Binding 은 키와 설명을 한 값에 묶는다. 앱은 이 값으로 키를
	// 받고(key.Matches) 호스트는 같은 값으로 도움말을 그리므로, 둘이
	// 어긋나려면 한 값이 자기 자신과 달라야 한다.
	Keys() []key.Binding

	// Hint 는 **지금 고른 것으로 무엇을 할 수 있는가**다. 없으면 빈 문자열.
	//
	// 호스트가 입력창 안내문 자리에 놓는다. 그 자리인 이유는 둘이다 —
	// 자리를 새로 안 먹고, 눈이 이미 거기 있다(치는 자리다). 그리고 글자를
	// 치면 안내문이 사라지므로, 칠 때는 저절로 비켜난다.
	//
	// enter 가 자리마다 다른 일을 하던 시절에는 누르기 전에 무엇이 일어날지
	// 알 수 없었다. 뜻을 하나로 모은 뒤에도(musicapp/play.go) 그 하나가
	// 무엇인지는 화면이 말해야 한다.
	Hint() string

	// Badge 는 사용자가 아직 보지 않은 것의 개수다. 없으면 0.
	Badge() int

	// Ready 는 앱이 쓸 수 있는 상태인지 본다.
	// 앱마다 관문이 다르다 — 음악은 macOS 자동화 권한, 채팅은 로그인.
	// nil 이 아니면 호스트가 그 사유를 대신 띄운다.
	Ready() error

	// Ask 는 자연어 요청을 받는다. 라우터가 이 앱을 지목했을 때 불린다.
	Ask(prompt string) tea.Cmd

	// Back 은 앱 안에서 한 단계 물러난다. 물러났으면 true 다.
	//
	// esc 는 호스트의 키다(docs/07 2절). 그런데 그 키가 하는 일은 "한 단계씩
	// 물러난다"이고, 앱이 자기 안에 단계를 갖고 있으면 그 사슬의 한 칸이 된다 —
	// 음악의 아티스트 파고들기가 그렇다. 앱에게 esc 를 통째로 넘기지 않는 이유는
	// 앱마다 물러나는 뜻이 달라지기 때문이다. 넘기는 것은 요청 하나뿐이다.
	//
	// 물러날 곳이 없으면 false 다. 그러면 호스트가 자기 차례를 이어서 밟는다 —
	// 홈으로, 그리고 종료로.
	Back() (App, bool)

	// Filter 는 검색어를 받는다. 즉시 반영되어야 하므로 Cmd 를 돌려주지 않는다.
	// 빈 문자열이면 필터를 푼다.
	Filter(query string) App

	// Commands 는 이 앱이 등록하는 슬래시 명령이다.
	// 호스트가 전부 모아 하나의 팔레트로 보여준다.
	Commands() []Command

	// Facts 는 첫 화면이 적는 "무엇에 붙어 있고 무엇을 갖고 있는가"다.
	//
	// 호스트가 직접 세지 않고 앱에게 묻는 이유는, 세는 방법이 앱마다
	// 다르기 때문이다 — 음악은 라이브러리 스냅샷을 세고, 채팅이라면
	// 서버와 대화 수를 셀 것이다. 호스트가 data 패키지를 직접 읽으면
	// "앱은 음악 하나"라는 가정이 껍데기 쪽에 박힌다.
	//
	// 그릴 때마다 불린다. 값이 바뀌면 다음 프레임에 저절로 따라오므로
	// 갱신을 알릴 통로가 따로 없다. 대신 여기서 파일이나 망을 읽으면 안 된다.
	Facts() []Fact
}

// Fact 는 첫 화면 박스의 한 줄이다.
//
// Group 이 앞줄과 다르면 호스트가 제목을 하나 세우고 한 줄 띄운다. 줄이
// 대여섯이면 제목이 설명하는 것보다 자리를 더 먹지만, 박스가 본문을 다
// 쓰는 지금은 묶음이 없으면 열 줄짜리 평평한 목록이 된다.
type Fact struct {
	Group  string // "Sources" — 앞줄과 같으면 제목을 다시 쓰지 않는다
	Name   string // "Music.app". 비어 있으면 설명이 칸 전체를 쓴다
	Detail string // "play · queue · playlists"
}

// ResizeMsg 는 호스트가 앱에게 **본문** 크기를 알려준다.
//
// View 가 크기를 받으므로 그리는 데는 필요 없다. 다만 스크롤 위치처럼
// 키 입력 시점에 높이를 알아야 하는 계산이 있어서, 그 용도로만 쓴다.
// 레이아웃의 근거는 언제나 View 의 인자다.
type ResizeMsg struct{ Width, Height int }

// AskMsg 는 호스트가 자연어 요청을 앱에게 넘길 때 쓴다.
// 라우터가 이 앱을 지목했다는 뜻이다.
type AskMsg struct{ Prompt string }

// SayMsg 는 앱이 로그에 한 줄 남길 때 쓴다.
//
// 문장을 만드는 것은 앱이지만 보관하고 보여주는 것은 호스트다.
// 프로세스가 stdout 에 쓰고 터미널이 스크롤백을 갖는 관계와 같다.
//
// 한 문장이 여러 앱에 갈 수 있으므로(“조용한 거 틀고 슬랙도 꺼줘”)
// 로그는 어느 앱에도 속하지 않는다. 입력이 전역이면 출력도 전역이어야 한다.
type SayMsg struct {
	App  string // 누가 말했는지
	Text string
	Err  bool // 실패를 알리는 말인지

	// Detail 은 ctrl+o 로 펼쳤을 때 보일 줄들이다. 없으면 비워 둔다.
	//
	// "무슨 일이 있었나"를 담는다 — 후보가 몇 개였고 왜 그것을 골랐고
	// 얼마나 걸렸는지. "AI 가 무엇을 생각했나"는 담을 수 없다.
	// 모델은 추론 과정을 돌려주지 않는다.
	Detail []string
}

// Say 는 SayMsg 를 만드는 Cmd 를 돌려준다. 앱이 Update 에서 쓴다.
func Say(name, text string) tea.Cmd {
	return func() tea.Msg { return SayMsg{App: name, Text: text} }
}

// SayWith 는 펼쳐 볼 상세까지 함께 남긴다.
func SayWith(name, text string, detail []string) tea.Cmd {
	return func() tea.Msg { return SayMsg{App: name, Text: text, Detail: detail} }
}

// SayErr 는 실패를 로그에 남긴다. 실패도 대화의 일부다.
func SayErr(name string, err error) tea.Cmd {
	return func() tea.Msg { return SayMsg{App: name, Text: err.Error(), Err: true} }
}

// Command 는 슬래시 명령 하나다.
type Command struct {
	Name string // "/queue" — 앱 이름을 접두어로 붙이지 않는다
	Arg  string // "<name>" 처럼 인자 모양. 없으면 빈 문자열
	Help string // 팔레트에 보이는 한 줄
	Run  func(arg string) tea.Cmd
}

// FocusMsg 는 이 앱이 화면 앞으로 나왔다는 뜻이다.
// BlurMsg 는 화면이 다른 앱으로 넘어갔다는 뜻이다.
//
// 앱은 시작할 때 전부 켜져서 끝까지 살아 있다(docs/07). 그것으로 충분했다 —
// 재생과 메시지 수신은 배경에서 계속되어야 하니까. 카메라는 다르다.
// 안 보는 동안에도 잡고 있으면 초록불이 세션 내내 켜져 있다.
//
// 그래서 "살아 있다"와 "장치를 잡고 있다"를 나눈다. 계약의 메서드가 아니라
// 메시지인 이유는, 신경 쓸 앱만 받으면 되기 때문이다 — 나머지는 무시한다.
type (
	FocusMsg struct{}
	BlurMsg  struct{}
)

// CancelMsg 는 "방금 시킨 일을 그만둬"다.
//
// esc 사슬의 첫 칸이다(host.go). 선곡은 실측 7초라 그 사이에 잘못 물어본
// 것을 알아채는데, 지금까지는 물러날 방법이 없었다.
//
// ctrl+c 로 하지 않는 이유는 그것이 터미널의 탈출구이기 때문이다. 두 번
// 눌러야 죽게 만들면 그 문에 걸쇠를 다는 것이 된다. esc 는 이미 "한 단계씩
// 물러난다"이므로 새로 배울 것이 없다.
//
// 메서드가 아니라 메시지인 이유는 FocusMsg 와 같다 — 오래 걸리는 일을
// 하는 앱만 받으면 되고, 나머지는 무시한다.
type CancelMsg struct{}
