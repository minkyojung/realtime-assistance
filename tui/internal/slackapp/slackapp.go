// Package slackapp 은 호스트가 담는 두 번째 앱이다.
//
// **지금은 뼈대다.** 계약(app.App)은 전부 지켜져 있고 안이 비어 있다.
// 채우는 순서는 아래 "단계"를 따른다.
//
// # 먼저 읽을 것
//
// docs/07-호스트-계약.md — 이 앱이 무엇을 해도 되고 무엇을 하면 안 되는지.
// 특히 세 가지를 지켜야 한다.
//
//  1. **자기 키 바인딩을 만들지 않는다.** 하고 싶은 것은 Commands() 로 등록한다.
//     앱마다 조작법이 달라지면 이것은 그냥 tmux 가 된다.
//  2. **결과는 app.Say() 로 로그에 남긴다.** 자기 화면에만 쓰면
//     여러 앱에 걸친 요청("조용한 거 틀고 알림 꺼줘")의 결과가 사라진다.
//  3. **Init(send) 로 밀어 넣는다.** 음악은 우리가 물어보지만(폴링)
//     채팅은 저쪽에서 온다. 이 앱의 존재 이유가 그것이고,
//     하네스가 옳은지도 여기서 증명된다.
//
// # 단계
//
//	1. 관문      토큰이 없으면 Ready() 가 알리고, 화면이 다음에 뭘 할지 안내
//	2. 읽기      대화 목록과 안 읽음 개수. Badge() 동작
//	3. 밀어넣기  Socket Mode. 다른 앱을 보는 중에도 Badge 가 오른다  ← 핵심
//	4. 쓰기      Ask() 로 메시지 보내기, 방해금지 켜기
//	5. 합류      "조용한 거 틀고 알림 꺼줘" 한 문장에 둘 다 동작
//
// # 건드리지 말 것
//
// 이 패키지 밖. internal/app 은 읽기만 하고, host·musicapp·style 은
// 다른 세션이 맡는다. main.go 는 앱 등록 한 줄만.
package slackapp

import (
	"errors"
	"os"
	"strings"

	"amcli/tui/internal/app"
	"amcli/tui/internal/style"
	tea "charm.land/bubbletea/v2"
)

// Model 은 이 앱의 상태 전부다.
//
// 크기(w, h)를 담지 않는 것에 주의. View 가 인자로 받으므로 기억할 필요가
// 없고, 스크롤 계산처럼 키 입력 시점에 높이가 필요하면 app.ResizeMsg 를
// 받아 그것만 따로 들고 있는다.
type Model struct {
	// 1단계 — 관문
	token string

	// 2단계 — 읽기
	// TODO: conversations []Conversation
	// TODO: unread int

	// 3단계 — 밀어넣기
	send func(tea.Msg) // Init 이 받아 둔다. 웹소켓이 이걸로 메시지를 넣는다

	// 호스트가 넣어주는 검색어. 빈 문자열이면 필터가 없다.
	filter string
}

// 컴파일 타임에 계약을 지키는지 확인한다.
var _ app.App = Model{}

func New() Model {
	return Model{token: os.Getenv("SLACK_USER_TOKEN")}
}

// ─────────────────────────────────────────────────────────────
// 이름과 설명
// ─────────────────────────────────────────────────────────────

func (m Model) Name() string { return "chat" }

// Description 은 라우터가 보는 **유일한** 근거다.
//
// "누가 나 찾았어?" 같은 문장에는 앱 이름이 안 나오므로, 이 한 줄이
// 무엇을 다루는 앱인지 구체적으로 말해야 한다.
func (m Model) Description() string {
	return "Reads and sends messages in the user's Slack workspace. " +
		"Handles who messaged, unread conversations, replying, " +
		"and setting status or do-not-disturb."
}

// ─────────────────────────────────────────────────────────────
// 1단계 — 관문
// ─────────────────────────────────────────────────────────────

var errNoToken = errors.New(
	"Slack 토큰이 없습니다. api.slack.com/apps 에서 앱을 만들고 .env 를 채우세요")

// Ready 는 앱이 쓸 수 있는 상태인지 본다.
//
// 앱마다 관문이 다르다 — 음악은 macOS 자동화 권한이고, 여기는 로그인이다.
// nil 이 아니면 호스트가 그 사유를 대신 띄운다.
func (m Model) Ready() error {
	if strings.TrimSpace(m.token) == "" {
		return errNoToken
	}
	return nil
}

// ─────────────────────────────────────────────────────────────
// 3단계 — 밀어넣기
// ─────────────────────────────────────────────────────────────

// Init 은 앱을 시작한다.
//
// send 는 이벤트 루프 **밖에서** 메시지를 넣는 통로다. Socket Mode 는
// 웹소켓이라 우리가 물어보는 게 아니라 저쪽에서 오므로, 고루틴에서
// 받아 send 로 밀어 넣으면 된다.
//
//	go func() {
//	    for ev := range socket.Events() {
//	        send(incomingMsg{ev})
//	    }
//	}()
func (m Model) Init(send func(tea.Msg)) tea.Cmd {
	m.send = send
	// TODO 3단계: Socket Mode 연결. 토큰이 없으면 아무것도 하지 않는다.
	return nil
}

// ─────────────────────────────────────────────────────────────
// 화면
// ─────────────────────────────────────────────────────────────

// View 는 본문을 그린다. 주어진 크기를 넘지 않아야 한다.
//
// 모양은 이 앱 마음대로다. 음악은 목록을 그리지만 여기는 대화를 그려도
// 되고 채널 사이드바를 둬도 된다. 호스트가 갖는 것은 입력창과 상태줄
// 두 줄뿐이다.
//
// 좁아질 때 어느 칸을 버릴지는 style.Columns 에 선언하면 된다 —
// 채널명·보낸이·시각·본문 중 무엇을 먼저 버릴지는 이 앱이 정한다.
func (m Model) View(w, h int) string {
	lines := make([]string, 0, h)
	if err := m.Ready(); err != nil {
		lines = append(lines,
			style.ErrorBadge.Render("LOGIN")+" "+
				style.Body.Render(style.Truncate(err.Error(), w-8)))
	} else {
		// TODO 2단계: 대화 목록
		lines = append(lines, style.Faint.Render("TODO: conversations"))
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines[:h], "\n")
}

// Status 는 상태줄에 들어갈 한 줄이다. 배경에 있어도 호출된다.
// 다른 앱을 보는 중에도 이 앱이 뭘 하는지 여기로 보인다.
func (m Model) Status() string {
	if err := m.Ready(); err != nil {
		return style.Faint.Render("chat · not signed in")
	}
	// TODO 2단계: "#general · 12 people" 처럼
	return style.Dim.Render("chat")
}

// Badge 는 사용자가 아직 보지 않은 것의 개수다.
//
// 지금은 숫자만 내놓는다. "어디까지 방해해도 되는가"는 답이 정해지지 않은
// 문제라 미뤄두었다 — docs/07 6절.
func (m Model) Badge() int {
	// TODO 2단계: 안 읽은 대화 수
	return 0
}

// ─────────────────────────────────────────────────────────────
// 입력
// ─────────────────────────────────────────────────────────────

// Ask 는 자연어 요청을 받는다. 라우터가 이 앱을 지목했을 때 불린다.
//
// 결과는 **반드시 app.Say() 로 로그에 남긴다.** 자기 화면에만 쓰면
// 다른 앱을 보고 있는 사용자에게는 아무 일도 안 일어난 것처럼 보인다.
func (m Model) Ask(prompt string) tea.Cmd {
	// TODO 4단계: 의도를 해석해 메시지를 보내거나 방해금지를 켠다.
	return app.Say(m.Name(), "TODO: "+prompt)
}

// Filter 는 검색어를 받는다. 즉시 반영되어야 하므로 Cmd 를 돌려주지 않는다.
func (m Model) Filter(q string) app.App {
	m.filter = q
	return m
}

// Commands 는 이 앱이 등록하는 슬래시 명령이다.
//
// **앱은 자기 키 바인딩을 만들지 않는다.** 하고 싶은 것이 있으면 여기 등록하고,
// 호스트가 전부 모아 하나의 팔레트로 보여준다.
func (m Model) Commands() []app.Command {
	// TODO 4단계: /dnd (방해금지), /unread (안 읽은 것만)
	return nil
}

// Update 는 호스트가 자기 몫을 처리한 뒤 남은 메시지를 넘긴다.
//
// App 을 돌려주므로 호스트에 타입 단언이 없다.
func (m Model) Update(msg tea.Msg) (app.App, tea.Cmd) {
	switch msg := msg.(type) {
	case app.AskMsg:
		return m, m.Ask(msg.Prompt)

	case app.ResizeMsg:
		// TODO: 스크롤 계산에 높이가 필요하면 여기서 받아 둔다.
		// 그리는 것은 언제나 View 의 인자를 따른다.
		return m, nil
	}
	// TODO 2~4단계: 방향키로 대화 고르기, enter 로 열기 등
	return m, nil
}
