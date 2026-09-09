// Package slackapp 은 호스트가 담는 두 번째 앱이다.
//
// 음악 앱과 하나만 다르다. **음악은 우리가 물어보고(폴링) 채팅은 저쪽에서
// 온다.** 그 차이가 계약의 Init(send) 를 존재하게 했고, 이 앱이 그것을 쓴다.
//
// # 지키는 것 셋
//
//  1. **자기 키 바인딩을 만들지 않는다.** 하고 싶은 것은 Commands() 로 등록한다.
//     앱마다 조작법이 달라지면 이것은 그냥 tmux 가 된다.
//  2. **결과는 app.Say() 로 로그에 남긴다.** 자기 화면에만 쓰면
//     여러 앱에 걸친 요청("조용한 거 틀고 알림 꺼줘")의 결과가 사라진다.
//  3. **Init(send) 로 밀어 넣는다.** socket.go 가 그 통로를 쓴다.
//
// # 파일 배치
//
//	slackapp.go  계약 — 호스트가 보는 전부
//	client.go    Slack Web API (net/http). 통신은 앱이 자기 안에 감춘다
//	load.go      2단계 — 켤 때 한 번 읽는 대화 목록
//	socket.go    3단계 — Socket Mode. 이 앱의 존재 이유
//	ask.go       4단계 — 문장 하나를 보내기·방해금지로 바꾼다
//	command.go   슬래시 명령
//	list.go      본문 그리기
//
// # 건드리지 말 것
//
// 이 패키지 밖. internal/app 은 읽기만 하고, host·musicapp·style 은
// 다른 세션이 맡는다. main.go 는 앱 등록 한 줄만.
package slackapp

import (
	"errors"
	"os"
	"strconv"
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
	token    string // xoxp- 사용자 토큰. 없으면 앱을 못 쓴다
	appToken string // xapp- 앱 토큰. 없으면 실시간 수신만 꺼진다

	// 2단계 — 읽기
	me      string // 내 user id. 내가 쓴 것은 안 읽음으로 세지 않는다
	team    string
	convs   []Conversation
	users   map[string]string // user id → 표시 이름
	loaded  bool
	loadErr error

	// 3단계 — 밀어넣기
	live      bool  // Socket Mode 가 붙어 있는가
	socketErr error // 안 붙어 있으면 왜인지

	// 목록 상태
	sel        int
	top        int
	onlyUnread bool
	filter     string // 호스트가 넣어주는 검색어. 빈 문자열이면 필터가 없다

	// 스크롤 계산에만 쓴다. 그리는 것은 언제나 View 의 인자를 따른다.
	bodyH int
}

// 컴파일 타임에 계약을 지키는지 확인한다.
var _ app.App = Model{}

func New() Model {
	return Model{
		token:    strings.TrimSpace(os.Getenv("SLACK_USER_TOKEN")),
		appToken: strings.TrimSpace(os.Getenv("SLACK_APP_TOKEN")),
		users:    map[string]string{},
	}
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
//
// **앱 토큰(xapp-)은 관문이 아니다.** 그것이 없으면 실시간 수신만 꺼지고,
// 읽기·쓰기는 그대로 된다. 못 하는 것 하나 때문에 되는 것까지 막지 않는다.
func (m Model) Ready() error {
	if m.token == "" {
		return errNoToken
	}
	if m.loadErr != nil {
		return m.loadErr
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
// 받아 send 로 밀어 넣는다.
//
// send 를 Model 에 담아두지 않는 것에 주의. 이 메서드는 값 수신자라
// 여기서 넣은 필드는 호출자에게 돌아가지 않는다. 필요한 곳은 고루틴뿐이고,
// 고루틴은 클로저로 잡으면 된다.
func (m Model) Init(send func(tea.Msg)) tea.Cmd {
	if m.token == "" {
		return nil // 관문에서 막힌다. 아무것도 부르지 않는다
	}
	if m.appToken != "" {
		startSocket(m.appToken, send)
	}
	return cmdLoad(m.token)
}

// ─────────────────────────────────────────────────────────────
// 화면
// ─────────────────────────────────────────────────────────────

// View 는 본문을 그린다. 주어진 크기를 넘지 않아야 한다.
//
// 머리1 + 룰1 + 목록 + 룰1 + 안내(있을 때만). 음악의 뼈대와 같은 모양이고,
// 재생 바 자리에 워크스페이스가 들어간다.
func (m Model) View(w, h int) string {
	if err := m.Ready(); err != nil {
		return m.viewGate(w, h, err)
	}

	var b strings.Builder
	b.WriteString(m.viewHead(w))
	b.WriteString("\n")
	b.WriteString(style.RuleBrand(w))
	b.WriteString("\n")
	b.WriteString(m.viewList(w, m.listHeight(h)))
	b.WriteString("\n")
	b.WriteString(style.Rule(w))
	if hint := m.hint(); hint != "" {
		b.WriteString("\n")
		b.WriteString(style.Faint.Render(style.Truncate(hint, w)))
	}
	return fit(b.String(), h)
}

// fit 은 본문을 h 줄로 맞춘다.
//
// 목록에는 최소 높이가 있어서(3줄) 아주 낮은 창에서는 예산을 넘을 수 있다.
// 그때 넘치는 쪽은 아래다 — 머리와 목록 윗줄이 먼저 살아남아야 한다.
func fit(s string, h int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:style.Max(h, 0)]
	}
	return strings.Join(lines, "\n")
}

func (m Model) viewHead(w int) string {
	left := style.Title.Render(m.team)
	if m.team == "" {
		left = style.Faint.Render("연결하는 중…")
	}
	right := style.Faint.Render(m.liveLabel())
	return style.Row(left, right, w)
}

func (m Model) liveLabel() string {
	switch {
	case m.appToken == "":
		return "실시간 꺼짐"
	case m.live:
		return "실시간 ●"
	default:
		return "실시간 ○"
	}
}

// hint 는 지금 사용자가 알아야 할 한 줄이다. 없으면 줄 자체를 만들지 않는다.
//
// 빈 줄을 남기지 않는 것이 이 화면들의 규칙이다.
func (m Model) hint() string {
	if m.appToken == "" {
		return "SLACK_APP_TOKEN 이 없어 새 메시지가 저절로 오지 않습니다 — docs/08 5절"
	}
	if !m.live && m.socketErr != nil {
		return socketHint(m.socketErr).Error()
	}
	return ""
}

// 관문 화면. 다음에 뭘 해야 하는지가 전부다.
//
// **여기서 대신 해 줄 수 있는 것이 없다.** 앱을 만드는 것도 권한을 고르는
// 것도 사용자만 할 수 있으므로, 화면이 할 일은 순서를 틀리지 않게 하는 것뿐이다.
func (m Model) viewGate(w, h int, err error) string {
	lines := []string{
		style.ErrorBadge.Render("LOGIN") + " " +
			style.Body.Render(style.Truncate(err.Error(), style.Max(w-8, 10))),
		"",
	}
	for _, s := range gateSteps {
		lines = append(lines, style.Faint.Render(style.Truncate(s, w)))
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines[:style.Min(len(lines), h)], "\n")
}

var gateSteps = []string{
	"1. api.slack.com/apps 에서 앱을 만듭니다",
	"2. Socket Mode 를 켜고 app-level token (xapp-) 을 받습니다",
	"3. User Token Scopes: channels:read groups:read im:read im:history",
	"   chat:write dnd:write users:read",
	"4. 워크스페이스에 설치하고 user token (xoxp-) 을 받습니다",
	"5. 저장소 루트 .env 에 SLACK_APP_TOKEN · SLACK_USER_TOKEN 을 넣고",
	"   set -a && . ./.env && set +a 로 다시 켭니다",
}

// Status 는 상태줄에 들어갈 한 줄이다. 배경에 있어도 호출된다.
// 다른 앱을 보는 중에도 이 앱이 뭘 하는지 여기로 보인다.
func (m Model) Status() string {
	if m.token == "" {
		return style.Faint.Render("chat · not signed in")
	}
	if m.loadErr != nil {
		return style.Warn.Render("chat · " + style.Truncate(m.loadErr.Error(), 48))
	}
	if !m.loaded {
		return style.Faint.Render("chat · 연결하는 중…")
	}

	parts := []string{"chat"}
	if m.team != "" {
		parts = append(parts, m.team)
	}
	parts = append(parts, plural(len(m.convs), "conversation"))
	if n := m.Badge(); n > 0 {
		parts = append(parts, strconv.Itoa(n)+" unread")
	}
	return style.Dim.Render(strings.Join(parts, " · "))
}

// "1 conversation" 과 "3 conversations". 상태줄은 좁으므로 단어를 아낀다.
func plural(n int, word string) string {
	s := strconv.Itoa(n) + " " + word
	if n != 1 {
		s += "s"
	}
	return s
}

// Badge 는 사용자가 아직 보지 않은 것의 개수다.
//
// 지금은 숫자만 내놓는다. "어디까지 방해해도 되는가"는 답이 정해지지 않은
// 문제라 미뤄두었다 — docs/07 6절.
func (m Model) Badge() int {
	n := 0
	for _, c := range m.convs {
		n += c.Unread
	}
	return n
}

// ─────────────────────────────────────────────────────────────
// 입력
// ─────────────────────────────────────────────────────────────

// Ask 는 자연어 요청을 받는다. 라우터가 이 앱을 지목했을 때 불린다.
//
// 결과는 **반드시 app.Say() 로 로그에 남긴다.** 자기 화면에만 쓰면
// 다른 앱을 보고 있는 사용자에게는 아무 일도 안 일어난 것처럼 보인다.
func (m Model) Ask(prompt string) tea.Cmd {
	if err := m.Ready(); err != nil {
		return app.SayErr(m.Name(), err)
	}
	return cmdAct(m.token, prompt, m.convs)
}

// Filter 는 검색어를 받는다. 즉시 반영되어야 하므로 Cmd 를 돌려주지 않는다.
func (m Model) Filter(q string) app.App {
	if m.filter != q {
		m.sel, m.top = 0, 0
	}
	m.filter = q
	return m
}

// Update 는 호스트가 자기 몫을 처리한 뒤 남은 메시지를 넘긴다.
//
// App 을 돌려주므로 호스트에 타입 단언이 없다.
func (m Model) Update(msg tea.Msg) (app.App, tea.Cmd) {
	switch msg := msg.(type) {
	case app.AskMsg:
		return m, m.Ask(msg.Prompt)

	case loadedMsg:
		m.loaded = true
		m.me, m.loadErr = msg.Me, msg.Err
		if msg.Team != "" {
			m.team = msg.Team
		}
		if msg.Err != nil {
			return m, app.SayErr(m.Name(), msg.Err)
		}
		m.convs, m.users = msg.Convs, msg.Users
		m.clampList()
		return m, nil

	case incomingMsg:
		return m.receive(msg)

	case nameMsg:
		return m.rename(msg), nil

	case socketMsg:
		m.live, m.socketErr = msg.Connected, msg.Err
		// 실패는 로그로도 간다. 조용히 끊긴 채로 있는 것이 제일 나쁘다.
		if msg.Err != nil {
			return m, app.SayErr(m.Name(), socketHint(msg.Err))
		}
		return m, nil

	case actedMsg:
		if msg.Err != nil {
			return m, app.SayErr(m.Name(), msg.Err)
		}
		return m, app.Say(m.Name(), msg.Say)

	case unreadMsg:
		m.onlyUnread = true
		m.sel, m.top = 0, 0
		return m, nil

	case allMsg:
		m.onlyUnread = false
		m.sel, m.top = 0, 0
		return m, nil

	case readAllMsg:
		for i := range m.convs {
			m.convs[i].Unread = 0
		}
		return m, nil

	case app.ResizeMsg:
		m.bodyH = msg.Height
		m.clampList()
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// receive 는 웹소켓이 밀어 넣은 메시지 하나를 앉힌다.
//
// **여기가 3단계의 증거다.** 음악 화면을 보고 있어도 이 경로는 돌고,
// 상태줄의 배지가 저절로 오른다.
func (m Model) receive(in incomingMsg) (app.App, tea.Cmd) {
	// 내가 쓴 것은 안 읽음이 아니다. 내 id 를 아는 곳은 여기뿐이라
	// 소켓이 아니라 이 자리에서 거른다.
	if in.User == m.me {
		return m, nil
	}

	// 슬라이스는 Model 복사본끼리 공유된다. 값을 고칠 때는 새로 만든다.
	convs := make([]Conversation, len(m.convs))
	copy(convs, m.convs)

	var cmds []tea.Cmd
	found := false
	for i := range convs {
		if convs[i].ID != in.Channel {
			continue
		}
		found = true
		convs[i].Unread++
		convs[i].Last = in.Text
		convs[i].LastBy = in.User
		convs[i].At = in.At
	}
	// 켤 때 안 보이던 대화에서 온 것이다. 이름은 뒤이어 물어본다.
	if !found {
		convs = append(convs, Conversation{
			ID: in.Channel, Name: in.Channel, Unread: 1,
			Last: in.Text, LastBy: in.User, At: in.At,
		})
		cmds = append(cmds, cmdConvName(m.token, in.Channel))
	}
	m.convs = convs

	if _, known := m.users[in.User]; !known {
		cmds = append(cmds, cmdUserName(m.token, in.User))
	}
	m.clampList()
	return m, tea.Batch(cmds...)
}

// rename 은 뒤늦게 알아낸 이름을 앉힌다.
func (m Model) rename(msg nameMsg) app.App {
	if msg.Kind == "user" {
		users := make(map[string]string, len(m.users)+1)
		for k, v := range m.users {
			users[k] = v
		}
		users[msg.ID] = msg.Name
		m.users = users
		return m
	}

	convs := make([]Conversation, len(m.convs))
	copy(convs, m.convs)
	for i := range convs {
		if convs[i].ID == msg.ID {
			convs[i].Name = msg.Name
		}
	}
	m.convs = convs
	return m
}

// 방향키와 tab 은 호스트가 안 쓰는 키라 여기로 내려온다.
// 새로 만드는 것이 아니라 이미 있는 조작법을 이 목록에 붙이는 것이다.
func (m Model) handleKey(msg tea.KeyPressMsg) (app.App, tea.Cmd) {
	n := m.rowCount()
	switch msg.String() {
	case "up", "ctrl+p":
		m.sel = style.Clamp(m.sel-1, 0, style.Max(n-1, 0))
		m.clampList()
	case "down", "ctrl+n":
		m.sel = style.Clamp(m.sel+1, 0, style.Max(n-1, 0))
		m.clampList()
	case "tab":
		m.sel = m.nextUnread(+1)
		m.clampList()
	case "shift+tab":
		m.sel = m.nextUnread(-1)
		m.clampList()
	case "enter":
		return m.openSelected()
	}
	return m, nil
}

// tab 은 "앱 안에서 다음"이다. 이 목록에서 다음이란 다음 안 읽은 대화다 —
// 한 줄씩 내려가는 것은 방향키가 이미 한다.
func (m Model) nextUnread(step int) int {
	rows := m.rows()
	if len(rows) == 0 {
		return 0
	}
	for i := 1; i <= len(rows); i++ {
		j := ((m.sel+step*i)%len(rows) + len(rows)) % len(rows)
		if rows[j].Unread > 0 {
			return j
		}
	}
	return m.sel
}

// enter 는 고른 대화를 본 것으로 친다.
//
// 우리 쪽 숫자만 내린다. Slack 서버의 읽음 위치를 옮기는 것은 다른 일이고
// (conversations.mark), 진짜 앱에서 안 읽음이 사라지는 것은 지금 범위 밖이다.
func (m Model) openSelected() (app.App, tea.Cmd) {
	rows := m.rows()
	if m.sel < 0 || m.sel >= len(rows) || rows[m.sel].Unread == 0 {
		return m, nil
	}
	target := rows[m.sel]

	convs := make([]Conversation, len(m.convs))
	copy(convs, m.convs)
	for i := range convs {
		if convs[i].ID == target.ID {
			convs[i].Unread = 0
		}
	}
	m.convs = convs
	return m, app.Say(m.Name(), target.Name+" 을 읽음으로 표시했습니다")
}

// clampList — 고른 줄이 늘 보이도록 스크롤 위치를 맞춘다.
func (m *Model) clampList() {
	h := m.listHeight(m.bodyH)
	n := m.rowCount()
	m.sel = style.Clamp(m.sel, 0, style.Max(n-1, 0))
	if m.sel < m.top {
		m.top = m.sel
	}
	if m.sel >= m.top+h {
		m.top = m.sel - h + 1
	}
	m.top = style.Clamp(m.top, 0, style.Max(n-h, 0))
}
