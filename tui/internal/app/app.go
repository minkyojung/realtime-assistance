// Package app 은 호스트와 앱 사이의 계약이다.
//
// 호스트는 두 줄만 갖는다 — 입력창과 상태줄. 그 위는 전부 앱의 것이다.
// 그래서 앱마다 화면이 달라도 된다. 음악은 사이드바와 목록을 그리고,
// 채팅 앱은 대화를 그리면 된다.
package app

import tea "charm.land/bubbletea/v2"

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

	// Badge 는 사용자가 아직 보지 않은 것의 개수다. 없으면 0.
	Badge() int

	// Ready 는 앱이 쓸 수 있는 상태인지 본다.
	// 앱마다 관문이 다르다 — 음악은 macOS 자동화 권한, 채팅은 로그인.
	// nil 이 아니면 호스트가 그 사유를 대신 띄운다.
	Ready() error

	// Ask 는 자연어 요청을 받는다. 라우터가 이 앱을 지목했을 때 불린다.
	Ask(prompt string) tea.Cmd

	// Filter 는 검색어를 받는다. 즉시 반영되어야 하므로 Cmd 를 돌려주지 않는다.
	// 빈 문자열이면 필터를 푼다.
	Filter(query string) App

	// Commands 는 이 앱이 등록하는 슬래시 명령이다.
	// 호스트가 전부 모아 하나의 팔레트로 보여준다.
	Commands() []Command
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
