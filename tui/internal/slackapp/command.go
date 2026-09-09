package slackapp

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"amcli/tui/internal/app"
	tea "charm.land/bubbletea/v2"
)

// 이 앱이 등록하는 슬래시 명령.
//
// **앱은 자기 키 바인딩을 만들지 않는다.** 하고 싶은 것이 있으면 여기 등록하고,
// 호스트가 전부 모아 하나의 팔레트로 보여준다 — docs/07-호스트-계약.md 3-1.
//
// 명령이 모델을 안 거치는 것이 체감 속도를 지탱한다. "/dnd" 는 즉시 나가고,
// "알림 좀 꺼줘" 만 모델을 거친다.

// 시간을 안 적으면 한 시간. Slack 자체의 기본값과 같다.
const defaultSnoozeMinutes = 60

func (m Model) Commands() []app.Command {
	return []app.Command{
		{Name: "/unread", Help: "only conversations with something new",
			Run: func(string) tea.Cmd { return send(unreadMsg{}) }},
		{Name: "/all", Help: "show every conversation again",
			Run: func(string) tea.Cmd { return send(allMsg{}) }},
		{Name: "/dnd", Arg: "<minutes>", Help: "turn on do not disturb",
			Run: m.dndCmd},
		{Name: "/undnd", Help: "turn do not disturb back off",
			Run: m.undndCmd},
		{Name: "/read", Help: "mark everything as read here",
			Run: func(string) tea.Cmd { return send(readAllMsg{}) }},
	}
}

// 명령의 결과는 메시지로 돌아온다. 그래야 상태가 Update 한 곳에서만 바뀐다.
type (
	unreadMsg  struct{}
	allMsg     struct{}
	readAllMsg struct{}
)

func send(msg tea.Msg) tea.Cmd { return func() tea.Msg { return msg } }

func (m Model) dndCmd(arg string) tea.Cmd {
	if err := m.Ready(); err != nil {
		return app.SayErr(m.Name(), err)
	}
	mins := defaultSnoozeMinutes
	if s := strings.TrimSpace(arg); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n <= 0 || n > 1440 {
			return app.SayErr(m.Name(), errBadMinutes)
		}
		mins = n
	}
	return cmdSnooze(m.token, mins)
}

func (m Model) undndCmd(string) tea.Cmd {
	if err := m.Ready(); err != nil {
		return app.SayErr(m.Name(), err)
	}
	return cmdEndSnooze(m.token)
}

var errBadMinutes = errors.New("/dnd 는 1에서 1440 사이의 분을 받습니다")

func cmdSnooze(token string, minutes int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
		defer cancel()
		if err := newClient(token).setSnooze(ctx, minutes); err != nil {
			return actedMsg{Err: err}
		}
		return actedMsg{Say: strconv.Itoa(minutes) + "분 동안 알림을 껐습니다"}
	}
}

func cmdEndSnooze(token string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
		defer cancel()
		if err := newClient(token).endSnooze(ctx); err != nil {
			return actedMsg{Err: err}
		}
		return actedMsg{Say: "알림을 다시 켰습니다"}
	}
}

// 테스트가 앱 안쪽 상태를 흘려보내기 위한 것들.
// 실제 Slack 을 부르지 않고 화면을 확인할 수 있게 한다.

// LoadedMsgFor 는 켤 때 오는 대화 목록을 흉내 낸다.
func LoadedMsgFor(me, team string, convs []Conversation) tea.Msg {
	return loadedMsg{Me: me, Team: team, Convs: convs, Users: map[string]string{}}
}

// IncomingMsgFor 는 Socket Mode 로 밀려 들어오는 메시지를 흉내 낸다.
func IncomingMsgFor(channel, user, text string, at time.Time) tea.Msg {
	return incomingMsg{Channel: channel, User: user, Text: text, At: at}
}

// SocketMsgFor 는 연결 상태 변화를 흉내 낸다.
func SocketMsgFor(connected bool, err error) tea.Msg {
	return socketMsg{Connected: connected, Err: err}
}
