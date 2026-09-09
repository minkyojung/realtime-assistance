package slackapp

import (
	"strings"
	"testing"
	"time"

	"amcli/tui/internal/app"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// 실제 Slack 을 부르지 않는다. 계약과 화면만 본다 —
// 네트워크가 있어야 도는 검증은 검증이 아니라 의식이다.

func sample() []Conversation {
	at := time.Date(2026, 9, 9, 14, 32, 0, 0, time.UTC)
	return []Conversation{
		{ID: "D1", Name: "@minkyo", IsIM: true, Unread: 2, Last: "늦을 것 같아요", At: at},
		{ID: "C1", Name: "#general", Last: "배포 끝났습니다", At: at},
		{ID: "C2", Name: "#random"},
	}
}

func loaded(t *testing.T) Model {
	t.Helper()
	m := Model{token: "xoxp-test", appToken: "xapp-test", users: map[string]string{}, bodyH: 20}
	next, _ := m.Update(LoadedMsgFor("U0", "Acme", sample()))
	got, ok := next.(Model)
	if !ok {
		t.Fatalf("Update 가 Model 을 안 돌려줬다: %T", next)
	}
	return got
}

// 1단계 — 토큰이 없으면 관문에서 막고, 다음에 뭘 할지 화면이 말한다.
func TestGate(t *testing.T) {
	m := Model{users: map[string]string{}}
	if m.Ready() == nil {
		t.Fatal("토큰이 없는데 Ready 가 nil 이다")
	}
	v := m.View(80, 12)
	if !strings.Contains(v, "LOGIN") {
		t.Error("관문 화면에 LOGIN 배지가 없다")
	}
	if !strings.Contains(v, "api.slack.com/apps") {
		t.Error("관문 화면이 다음에 할 일을 안 알려준다")
	}
	if m.Badge() != 0 {
		t.Error("로그인 전인데 배지가 0 이 아니다")
	}
}

// 2단계 — 목록을 읽으면 배지와 상태줄에 진짜 숫자가 뜬다.
func TestReadCounts(t *testing.T) {
	m := loaded(t)
	if got := m.Badge(); got != 2 {
		t.Errorf("Badge = %d, want 2", got)
	}
	if s := m.Status(); !strings.Contains(s, "Acme") || !strings.Contains(s, "2 unread") {
		t.Errorf("상태줄이 워크스페이스와 안 읽음을 안 보여준다: %q", s)
	}
	if !strings.Contains(m.View(80, 20), "@minkyo") {
		t.Error("본문에 대화 이름이 없다")
	}
}

// 3단계 — **이 앱의 존재 이유.**
// 이벤트 루프 밖에서 밀려 들어온 메시지가 배지를 올린다.
// 사용자가 음악 화면을 보고 있어도 이 경로는 그대로 돈다.
func TestIncomingRaisesBadge(t *testing.T) {
	m := loaded(t)

	next, _ := m.Update(IncomingMsgFor("C1", "U9", "누가 나 찾았어?", time.Now()))
	m = next.(Model)
	if got := m.Badge(); got != 3 {
		t.Fatalf("밀어 넣은 뒤 Badge = %d, want 3", got)
	}

	// 내가 쓴 것은 안 읽음이 아니다.
	next, _ = m.Update(IncomingMsgFor("C1", "U0", "네 지금 갑니다", time.Now()))
	if got := next.(Model).Badge(); got != 3 {
		t.Errorf("내 메시지가 배지를 올렸다: %d", got)
	}
}

// 모르는 대화에서 와도 버리지 않는다. 이름은 뒤이어 물어본다.
func TestIncomingFromUnknownConversation(t *testing.T) {
	m := loaded(t)
	next, cmd := m.Update(IncomingMsgFor("C99", "U9", "처음 보는 채널", time.Now()))
	m = next.(Model)
	if len(m.convs) != 4 {
		t.Fatalf("대화가 %d개, want 4", len(m.convs))
	}
	if m.Badge() != 3 {
		t.Errorf("Badge = %d, want 3", m.Badge())
	}
	if cmd == nil {
		t.Error("이름을 물어보는 Cmd 가 없다")
	}
}

// 뒤늦게 온 이름이 자리에 앉는다.
func TestRename(t *testing.T) {
	m := loaded(t)
	next, _ := m.Update(nameMsg{Kind: "conversation", ID: "C2", Name: "#치킨"})
	if got := next.(Model).convs[2].Name; got != "#치킨" {
		t.Errorf("이름 = %q, want #치킨", got)
	}
}

// enter 는 고른 대화를 읽음으로 바꾸고, **그 사실을 로그에 남긴다.**
// 자기 화면에만 쓰면 다른 앱을 보는 사용자에게는 아무 일도 안 일어난 것과 같다.
func TestOpenSaysToLog(t *testing.T) {
	m := loaded(t)
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := next.(Model).Badge(); got != 0 {
		t.Errorf("읽음 처리 뒤 Badge = %d, want 0", got)
	}
	if cmd == nil {
		t.Fatal("로그에 남기는 Cmd 가 없다")
	}
	say, ok := cmd().(app.SayMsg)
	if !ok {
		t.Fatalf("SayMsg 가 아니다: %T", cmd())
	}
	if say.App != "chat" {
		t.Errorf("누가 말했는지가 %q 다", say.App)
	}
}

// 앱은 자기 키 바인딩을 만들지 않는다. 하고 싶은 것은 전부 팔레트로 간다.
func TestCommandsRegistered(t *testing.T) {
	m := loaded(t)
	want := map[string]bool{"/unread": false, "/all": false, "/dnd": false, "/undnd": false, "/read": false}
	for _, c := range m.Commands() {
		if _, ok := want[c.Name]; !ok {
			t.Errorf("모르는 명령 %q", c.Name)
			continue
		}
		want[c.Name] = true
		if c.Help == "" {
			t.Errorf("%s 에 팔레트에 뜰 설명이 없다", c.Name)
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("%s 가 등록되지 않았다", name)
		}
	}
}

// /unread 는 안 읽은 것만 남긴다. /all 이 되돌린다.
func TestUnreadFilter(t *testing.T) {
	m := loaded(t)
	next, _ := m.Update(unreadMsg{})
	if n := next.(Model).rowCount(); n != 1 {
		t.Errorf("/unread 뒤 %d줄, want 1", n)
	}
	next, _ = next.(Model).Update(allMsg{})
	if n := next.(Model).rowCount(); n != 3 {
		t.Errorf("/all 뒤 %d줄, want 3", n)
	}
}

// 검색은 즉시 반영된다. Cmd 를 돌려주지 않는다.
func TestFilter(t *testing.T) {
	m := loaded(t)
	if n := m.Filter("general").(Model).rowCount(); n != 1 {
		t.Errorf("검색 결과 %d줄, want 1", n)
	}
	if n := m.Filter("").(Model).rowCount(); n != 3 {
		t.Errorf("검색을 푼 뒤 %d줄, want 3", n)
	}
}

// 앱은 자기 크기를 기억하지 않고, 받은 크기를 넘지 않는다.
func TestViewFitsEveryWidth(t *testing.T) {
	m := loaded(t)
	for _, w := range []int{30, 40, 60, 80, 120, 200} {
		for _, h := range []int{4, 6, 12, 24} {
			v := m.View(w, h)
			lines := strings.Split(v, "\n")
			if len(lines) > h {
				t.Errorf("w=%d h=%d: %d줄이 나왔다", w, h, len(lines))
			}
			for i, l := range lines {
				if got := lipgloss.Width(l); got > w {
					t.Errorf("w=%d h=%d 줄%d: 폭 %d", w, h, i, got)
				}
			}
		}
	}
}

// 앱 토큰이 없으면 실시간 수신만 꺼진다. 읽기·쓰기는 그대로다.
func TestAppTokenIsNotAGate(t *testing.T) {
	m := loaded(t)
	m.appToken = ""
	if err := m.Ready(); err != nil {
		t.Errorf("앱 토큰이 없다고 관문에서 막혔다: %v", err)
	}
	if !strings.Contains(m.hint(), "SLACK_APP_TOKEN") {
		t.Error("실시간 수신이 꺼진 이유를 화면이 말하지 않는다")
	}
}

// 연결이 끊기면 조용히 있지 않는다. 실패도 대화의 일부다.
func TestSocketFailureSays(t *testing.T) {
	m := loaded(t)
	_, cmd := m.Update(SocketMsgFor(false, apiError{Method: "apps.connections.open", Code: "invalid_auth"}))
	if cmd == nil {
		t.Fatal("연결 실패가 로그로 안 간다")
	}
	if say, ok := cmd().(app.SayMsg); !ok || !say.Err {
		t.Errorf("실패로 안 남았다: %#v", cmd())
	}
}

// 봉투에서 셀 것과 안 셀 것을 가른다. 배지의 정확도가 여기서 정해진다.
func TestEnvelopeFilters(t *testing.T) {
	base := func(mut func(*envelope)) envelope {
		var e envelope
		e.Type = "events_api"
		e.Payload.Event.Type = "message"
		e.Payload.Event.Channel = "C1"
		e.Payload.Event.User = "U9"
		e.Payload.Event.Ts = "1757400000.000200"
		mut(&e)
		return e
	}
	cases := []struct {
		name string
		env  envelope
		want bool
	}{
		{"보통 메시지", base(func(*envelope) {}), true},
		{"수정된 메시지", base(func(e *envelope) { e.Payload.Event.Subtype = "message_changed" }), false},
		{"봇이 쓴 것", base(func(e *envelope) { e.Payload.Event.BotID = "B1" }), false},
		{"메시지가 아닌 것", base(func(e *envelope) { e.Payload.Event.Type = "reaction_added" }), false},
		{"보낸이가 없는 것", base(func(e *envelope) { e.Payload.Event.User = "" }), false},
	}
	for _, c := range cases {
		if _, ok := c.env.incoming(); ok != c.want {
			t.Errorf("%s: %v, want %v", c.name, ok, c.want)
		}
	}
}

func TestParseTS(t *testing.T) {
	got := parseTS("1757400000.000200")
	if got.Unix() != 1757400000 {
		t.Errorf("초 = %d", got.Unix())
	}
	if !parseTS("").IsZero() {
		t.Error("빈 ts 가 영시각이 아니다")
	}
}
