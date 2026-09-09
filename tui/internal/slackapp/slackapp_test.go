package slackapp

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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
	m := Model{creds: creds{Access: "xoxp-test"}, appToken: "xapp-test", users: map[string]string{}, bodyH: 20}
	next, _ := m.Update(LoadedMsgFor("U0", "Acme", sample()))
	got, ok := next.(Model)
	if !ok {
		t.Fatalf("Update 가 Model 을 안 돌려줬다: %T", next)
	}
	return got
}

// 1단계 — 토큰이 없으면 관문에서 막고, 다음에 뭘 할지 화면이 말한다.
//
// 사용자가 할 일은 /login 하나다. 앱을 만들고 권한을 고르는 일은
// 우리가 이미 했다.
func TestGate(t *testing.T) {
	m := Model{users: map[string]string{}}
	if err := m.Ready(); err != errNoLogin {
		t.Fatalf("Ready = %v, want errNoLogin", err)
	}
	v := m.View(80, 12)
	if !strings.Contains(v, "LOGIN") {
		t.Error("관문 화면에 LOGIN 배지가 없다")
	}
	if !strings.Contains(v, "/login") {
		t.Error("관문 화면이 /login 을 안 알려준다")
	}
	if strings.Contains(v, "User Token Scopes") {
		t.Error("사용자에게 앱을 직접 만들라고 안내한다")
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
	want := map[string]bool{
		"/unread": false, "/all": false, "/dnd": false, "/undnd": false,
		"/read": false, "/login": false, "/logout": false,
	}
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

// ─────────────────────────────────────────────────────────────
// 브라우저 로그인
// ─────────────────────────────────────────────────────────────

// 로그인이 끝나면 토큰이 앉고 곧바로 목록을 읽으러 간다.
func TestLoginThenLogout(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := loaded(t)

	next, cmd := m.Update(loginMsg{Creds: creds{Access: "xoxp-new", Team: "Acme"}})
	m = next.(Model)
	if m.token() != "xoxp-new" {
		t.Errorf("토큰 = %q", m.token())
	}
	if m.Ready() != nil {
		t.Errorf("로그인했는데 관문이 안 열렸다: %v", m.Ready())
	}
	if cmd == nil {
		t.Error("로그인 뒤 목록을 읽으러 가지 않는다")
	}
	if got := loadCreds().Access; got != "xoxp-new" {
		t.Errorf("저장된 토큰 = %q", got)
	}

	// 파일을 지우는 것은 Cmd 이고, Update 는 그 결과를 받는다.
	out := cmdLogout()()
	if lm, ok := out.(logoutMsg); !ok || lm.Err != nil {
		t.Fatalf("logout = %#v", out)
	}
	next, _ = m.Update(out)
	m = next.(Model)
	if m.token() != "" || m.Ready() == nil {
		t.Error("로그아웃했는데 관문이 안 닫혔다")
	}
	if len(m.convs) != 0 {
		t.Error("로그아웃했는데 대화가 남아 있다")
	}
	if loadCreds().Access != "" {
		t.Error("저장된 토큰이 안 지워졌다")
	}
}

// 실패해도 조용히 있지 않는다. 그리고 기다리는 표시를 풀어야 한다 —
// 안 그러면 상태줄이 영원히 "승인을 기다리는 중"으로 남는다.
func TestLoginFailureSays(t *testing.T) {
	m := loaded(t)
	m.loggingIn = true

	next, cmd := m.Update(loginMsg{Err: errDenied})
	if next.(Model).loggingIn {
		t.Error("실패했는데 기다리는 중으로 남아 있다")
	}
	if say, ok := cmd().(app.SayMsg); !ok || !say.Err {
		t.Errorf("실패로 안 남았다: %#v", cmd())
	}
}

// 검증자는 우리만 알고 해시만 먼저 나간다.
// 이 관계가 client_secret 없이도 안전한 이유다.
func TestPKCE(t *testing.T) {
	verifier, challenge, err := pkce()
	if err != nil {
		t.Fatal(err)
	}
	// RFC 7636 은 43~128자를 요구한다.
	if len(verifier) < 43 || len(verifier) > 128 {
		t.Errorf("검증자 길이 %d", len(verifier))
	}
	sum := sha256.Sum256([]byte(verifier))
	if want := base64.RawURLEncoding.EncodeToString(sum[:]); challenge != want {
		t.Errorf("challenge 가 검증자의 SHA-256 이 아니다")
	}
	if strings.ContainsAny(verifier+challenge, "+/=") {
		t.Error("URL-safe base64 가 아니다")
	}

	v2, _, _ := pkce()
	if v2 == verifier {
		t.Error("검증자가 매번 같다")
	}
}

func TestAuthorizeURL(t *testing.T) {
	u, err := url.Parse(authorizeURL("123.456", "CHAL", "STATE", 8765))
	if err != nil {
		t.Fatal(err)
	}
	if u.Host != "slack.com" || u.Path != "/oauth/v2/authorize" {
		t.Errorf("주소가 %s", u)
	}
	q := u.Query()
	for k, want := range map[string]string{
		"client_id":             "123.456",
		"code_challenge":        "CHAL",
		"code_challenge_method": "S256",
		"state":                 "STATE",
		"redirect_uri":          "http://localhost:8765/callback",
	} {
		if got := q.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	// desktop redirect 는 봇 권한을 요청할 수 없다. 사용자 권한만 보낸다.
	if q.Get("scope") != "" {
		t.Error("봇 scope 를 요청하고 있다")
	}
	if !strings.Contains(q.Get("user_scope"), "chat:write") {
		t.Errorf("user_scope = %q", q.Get("user_scope"))
	}
}

// 브라우저가 돌아오는 자리. 루프백에만 열려 있어야 한다.
func TestCallbackServer(t *testing.T) {
	srv, results, port, err := listenForCallback()
	if err != nil {
		t.Skipf("포트를 쓸 수 없다: %v", err)
	}
	defer srv.Close()

	resp, err := http.Get(redirectURI(port) + "?code=CODE&state=STATE")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	select {
	case got := <-results:
		if got.err != nil {
			t.Fatalf("err = %v", got.err)
		}
		if got.code != "CODE" || got.state != "STATE" {
			t.Errorf("code=%q state=%q", got.code, got.state)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("콜백이 안 왔다")
	}
}

func TestCallbackDenied(t *testing.T) {
	srv, results, port, err := listenForCallback()
	if err != nil {
		t.Skipf("포트를 쓸 수 없다: %v", err)
	}
	defer srv.Close()

	resp, err := http.Get(redirectURI(port) + "?error=access_denied")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	select {
	case got := <-results:
		if got.err != errDenied {
			t.Errorf("err = %v, want errDenied", got.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("콜백이 안 왔다")
	}
}

// 우리가 보낸 state 와 다르면 우리 요청의 답이 아니다.
func TestStateMismatchIsRejected(t *testing.T) {
	if errBadState == nil {
		t.Fatal("state 검사가 없다")
	}
}

// 사용자 토큰은 authed_user 안에 있다. 바깥 access_token 은 봇 것이다.
func TestOAuthResponseUsesAuthedUser(t *testing.T) {
	var r oauthResponse
	body := `{"ok":true,"access_token":"xoxb-bot","team":{"name":"Acme"},
	  "authed_user":{"access_token":"xoxp-me","refresh_token":"xoxe-1","expires_in":43200}}`
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatal(err)
	}
	c, err := r.creds()
	if err != nil {
		t.Fatal(err)
	}
	if c.Access != "xoxp-me" || c.Refresh != "xoxe-1" || c.Team != "Acme" {
		t.Errorf("%+v", c)
	}
	if c.Expires.IsZero() || c.expired() {
		t.Error("만료 시각이 안 앉았다")
	}

	// 사용자 토큰이 없으면 권한 설정이 틀린 것이다. 조용히 넘어가면 안 된다.
	var empty oauthResponse
	if _, err := empty.creds(); err != errNoUserTok {
		t.Errorf("err = %v, want errNoUserTok", err)
	}
}

// 회전을 안 켠 앱은 만료가 없는 토큰 하나만 준다. 그것도 정상이다.
func TestCredsWithoutExpiry(t *testing.T) {
	if (creds{Access: "xoxp-1"}).expired() {
		t.Error("만료가 없는 토큰을 만료로 봤다")
	}
	if !(creds{Access: "x", Expires: time.Now().Add(-time.Hour)}).expired() {
		t.Error("지난 토큰을 안 지난 것으로 봤다")
	}
	// 만료 직전은 미리 지난 것으로 본다. 부르는 도중에 만료되면 안 된다.
	if !(creds{Access: "x", Expires: time.Now().Add(30 * time.Second)}).expired() {
		t.Error("만료 직전을 미리 안 잡는다")
	}
}

func TestStoreRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if loadCreds().Access != "" {
		t.Error("아직 로그인 안 했는데 토큰이 있다")
	}
	want := creds{Access: "xoxp-1", Refresh: "xoxe-1", Team: "Acme"}
	if err := saveCreds(want); err != nil {
		t.Fatal(err)
	}
	if got := loadCreds(); got.Access != want.Access || got.Refresh != want.Refresh {
		t.Errorf("%+v", got)
	}

	// 남이 못 읽어야 한다.
	path, _ := storePath()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("권한이 %o", perm)
	}

	if err := clearCreds(); err != nil {
		t.Fatal(err)
	}
	if loadCreds().Access != "" {
		t.Error("지웠는데 남아 있다")
	}
	// 두 번 지워도 실패가 아니다.
	if err := clearCreds(); err != nil {
		t.Errorf("두 번째 삭제가 실패했다: %v", err)
	}
}

// 깨진 파일은 로그인 안 한 것과 같게 다룬다. 사용자가 할 일이 어느 쪽이든 같다.
func TestCorruptStoreIsNotAnError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path, _ := storePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{ not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if loadCreds().Access != "" {
		t.Error("깨진 파일에서 토큰이 나왔다")
	}
}

// 환경변수가 이긴다. 개발 중에 손으로 넣은 토큰을 쓸 수 있어야 한다.
func TestEnvTokenWins(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := saveCreds(creds{Access: "xoxp-stored"}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SLACK_USER_TOKEN", "xoxp-env")
	if got := New().token(); got != "xoxp-env" {
		t.Errorf("토큰 = %q, want xoxp-env", got)
	}
	t.Setenv("SLACK_USER_TOKEN", "")
	if got := New().token(); got != "xoxp-stored" {
		t.Errorf("토큰 = %q, want xoxp-stored", got)
	}
}

// 포트 하나가 막혀 있어도 로그인은 된다.
// 배포판에서 이게 없으면 그 포트를 쓰는 기계에서 아예 못 쓴다.
func TestCallbackFallsBackWhenPortBusy(t *testing.T) {
	// 첫 번째 포트를 우리가 미리 차지한다.
	blocker, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", callbackPorts[0]))
	if err != nil {
		t.Skipf("첫 포트가 이미 사용 중이다: %v", err)
	}
	defer blocker.Close()

	srv, results, port, err := listenForCallback()
	if err != nil {
		t.Fatalf("남은 포트가 있는데 실패했다: %v", err)
	}
	defer srv.Close()

	if port == callbackPorts[0] {
		t.Fatalf("막힌 포트를 골랐다: %d", port)
	}

	// 고른 포트가 리다이렉트 주소에 그대로 반영되어야 한다.
	// 여기가 어긋나면 Slack 이 redirect_uri 불일치로 막는다.
	resp, err := http.Get(redirectURI(port) + "?code=CODE&state=STATE")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	select {
	case got := <-results:
		if got.code != "CODE" {
			t.Errorf("code = %q", got.code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("콜백이 안 왔다")
	}
}

// 등록해야 하는 주소와 코드가 어긋나면 안 된다.
func TestEveryPortMakesALoopbackURI(t *testing.T) {
	if len(callbackPorts) < 2 {
		t.Fatal("포트가 하나뿐이면 대체가 없다")
	}
	for _, p := range callbackPorts {
		u, err := url.Parse(redirectURI(p))
		if err != nil {
			t.Fatal(err)
		}
		if u.Scheme != "http" || u.Hostname() != "localhost" || u.Path != callbackPath {
			t.Errorf("%s 가 루프백 주소가 아니다", u)
		}
	}
}

// client_id 가 박혀 있어야 다운로드한 사람이 환경변수 없이 로그인한다.
// 이 값은 비밀이 아니다 — PKCE 앱은 client_secret 을 쓰지 않는다.
func TestClientIDIsBakedIn(t *testing.T) {
	t.Setenv("SLACK_CLIENT_ID", "")
	if clientID == "" {
		t.Fatal("client_id 가 비어 있다. 배포판에서 /login 이 안 된다")
	}
	if oauthClientID() != clientID {
		t.Error("환경변수가 비었는데 박힌 값을 안 쓴다")
	}
	// 환경변수가 있으면 그것이 이긴다.
	t.Setenv("SLACK_CLIENT_ID", "other.id")
	if oauthClientID() != "other.id" {
		t.Error("환경변수가 안 이긴다")
	}
}

// 가져올 대화 종류와 요청하는 권한은 반드시 같이 움직인다.
//
// types 에 넣었는데 권한이 없으면 Slack 은 그것만 빼주지 않고 호출 전체를
// missing_scope 로 막는다. 로그인은 되는데 목록이 안 뜨는 상태가 된다 —
// 실제로 한 번 그랬다.
func TestRequestedScopesCoverRequestedTypes(t *testing.T) {
	granted := map[string]bool{}
	for _, s := range userScopes {
		granted[s] = true
	}
	for _, typ := range conversationTypes {
		scope, ok := conversationScopes[typ]
		if !ok {
			t.Errorf("%s 에 필요한 권한이 안 적혀 있다", typ)
			continue
		}
		if !granted[scope] {
			t.Errorf("%s 를 가져오는데 %s 권한을 요청하지 않는다", typ, scope)
		}
	}
}
