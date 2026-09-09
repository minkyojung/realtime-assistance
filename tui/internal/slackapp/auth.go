package slackapp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

// 브라우저로 하는 로그인.
//
// # 왜 이게 되는가
//
// Slack 은 리다이렉트 URL 에 HTTPS 를 강제한다. gh·gcloud 가 쓰는
// `http://localhost:포트` 루프백은 원래 거부된다. **PKCE 를 켠 앱만
// 예외다** — 문서 원문: "Redirects to localhost are treated as desktop
// redirects if the app has opted into PKCE."
//
// PKCE 는 앱을 public client 로 만들므로 client_secret 이 필요 없다.
// 그래서 바이너리에 박을 것이 client_id 하나뿐이고, 그것은 공개값이다.
// desktop redirect 는 bot scope 를 요청할 수 없지만 우리는 user scope
// 만 쓴다.
//
// # 이것이 없애주는 것과 없애주지 못하는 것
//
// SLACK_USER_TOKEN 은 사라진다. **SLACK_APP_TOKEN 은 사라지지 않는다** —
// 앱 토큰은 OAuth 로 발급되지 않고(앱 설정 화면에서만 나온다) 설치별이
// 아니라 앱당 하나라 배포할 수도 없다. 배포판의 실시간 수신은 다른
// 문제이고, 그때까지 이 앱은 앱 토큰이 있으면 켜고 없으면 끈다.

// clientID 는 배포할 때 여기에 박는다. 공개값이라 숨길 이유가 없다.
//
// 비어 있으면 환경변수를 본다. 값을 받기 전에도 시험해 볼 수 있어야 한다.
const clientID = ""

func oauthClientID() string {
	if clientID != "" {
		return clientID
	}
	return strings.TrimSpace(os.Getenv("SLACK_CLIENT_ID"))
}

// 리다이렉트 주소는 앱 설정에 **글자 그대로** 등록된 것과 같아야 한다.
// 그래서 포트를 고르지 않고 고정한다.
const (
	callbackPort = 8765
	callbackPath = "/callback"
)

func redirectURI() string {
	return fmt.Sprintf("http://localhost:%d%s", callbackPort, callbackPath)
}

// 요청하는 권한. 관문 화면이 안내하는 것과 같은 목록이어야 한다.
var userScopes = []string{
	"channels:read", "groups:read", "im:read", "im:history",
	"chat:write", "dnd:write", "users:read",
}

// 사람이 브라우저에서 앱을 고르고 승인하는 데 걸리는 시간.
// 이보다 오래 걸리면 창을 닫았다고 보는 편이 낫다.
const loginTimeout = 3 * time.Minute

// loginMsg 는 로그인 한 번의 결과다.
type loginMsg struct {
	Creds creds
	Err   error
}

// logoutMsg 는 저장된 토큰을 지웠다는 뜻이다.
type logoutMsg struct{ Err error }

var (
	errNoClientID = errors.New(
		"SLACK_CLIENT_ID 가 없습니다. api.slack.com/apps 의 Basic Information 에서 Client ID 를 받으세요")
	errPortBusy = fmt.Errorf(
		"포트 %d 를 쓸 수 없습니다. 그 포트를 쓰는 다른 프로그램을 끄고 다시 시도하세요", callbackPort)
	errDenied    = errors.New("로그인이 취소되었습니다")
	errBadState  = errors.New("응답이 우리가 보낸 요청과 맞지 않습니다. 다시 시도하세요")
	errNoUserTok = errors.New(
		"사용자 토큰이 오지 않았습니다. 앱 설정에서 User Token Scopes 를 확인하세요")
)

// cmdLogin 은 브라우저를 열고 돌아오는 것을 받는다.
//
// Cmd 안에서 통째로 돈다. Bubble Tea 가 Cmd 를 고루틴에서 돌리므로
// 여기서 기다려도 화면은 멈추지 않고, 결과는 보통 메시지로 Update 에
// 돌아온다. Init(send) 를 쓸 이유가 없는 것은 이것이 **우리가 시작한**
// 일이기 때문이다 — 저쪽에서 밀어 넣는 것은 socket.go 뿐이다.
func cmdLogin() tea.Cmd {
	return func() tea.Msg {
		id := oauthClientID()
		if id == "" {
			return loginMsg{Err: errNoClientID}
		}

		verifier, challenge, err := pkce()
		if err != nil {
			return loginMsg{Err: err}
		}
		state, err := randomToken()
		if err != nil {
			return loginMsg{Err: err}
		}

		srv, results, err := listenForCallback()
		if err != nil {
			return loginMsg{Err: err}
		}
		defer srv.Close()

		if err := openBrowser(authorizeURL(id, challenge, state)); err != nil {
			return loginMsg{Err: err}
		}

		select {
		case got := <-results:
			if got.err != nil {
				return loginMsg{Err: got.err}
			}
			if got.state != state {
				return loginMsg{Err: errBadState}
			}
			ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
			defer cancel()
			c, err := exchange(ctx, id, got.code, verifier)
			return loginMsg{Creds: c, Err: err}

		case <-time.After(loginTimeout):
			return loginMsg{Err: errors.New("로그인을 기다리다 시간이 지났습니다")}
		}
	}
}

func authorizeURL(id, challenge, state string) string {
	q := url.Values{
		"client_id":             {id},
		"user_scope":            {strings.Join(userScopes, ",")},
		"redirect_uri":          {redirectURI()},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	return "https://slack.com/oauth/v2/authorize?" + q.Encode()
}

// ─────────────────────────────────────────────────────────────
// PKCE
// ─────────────────────────────────────────────────────────────

// pkce 는 검증자와 그 해시를 만든다.
//
// 검증자는 우리만 알고, 해시만 먼저 Slack 에 보낸다. 나중에 코드를
// 토큰으로 바꿀 때 검증자를 함께 내면 "그 코드를 받은 게 나"임이 증명된다.
// client_secret 없이도 안전한 이유가 이것이다.
func pkce() (verifier, challenge string, err error) {
	verifier, err = randomToken()
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ─────────────────────────────────────────────────────────────
// 되돌아오는 것을 받는 자리
// ─────────────────────────────────────────────────────────────

type callback struct {
	code  string
	state string
	err   error
}

// listenForCallback 은 루프백에만 귀를 연다.
//
// 127.0.0.1 과 [::1] 둘 다 듣는다. 브라우저가 localhost 를 어느 쪽으로
// 푸는지는 /etc/hosts 에 달려 있어서, 하나만 열면 어떤 기계에서는
// 되고 어떤 기계에서는 안 된다. 0.0.0.0 으로 여는 것은 답이 아니다 —
// 같은 네트워크의 다른 기계에 인가 코드를 노출한다.
func listenForCallback() (*http.Server, <-chan callback, error) {
	results := make(chan callback, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		var got callback
		switch {
		case q.Get("error") == "access_denied":
			got.err = errDenied
		case q.Get("error") != "":
			got.err = fmt.Errorf("Slack: %s", q.Get("error"))
		case q.Get("code") == "":
			got.err = errors.New("인가 코드가 오지 않았습니다")
		default:
			got.code, got.state = q.Get("code"), q.Get("state")
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if got.err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, closedPage, "로그인하지 못했습니다", got.err.Error())
		} else {
			fmt.Fprintf(w, closedPage, "로그인되었습니다", "터미널로 돌아가세요.")
		}

		select {
		case results <- got:
		default: // 이미 하나 받았다. 새로고침으로 두 번 오는 경우다
		}
	})

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", callbackPort))
	if err != nil {
		return nil, nil, errPortBusy
	}

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go srv.Serve(ln)

	// IPv6 루프백은 있으면 좋고 없어도 그만이다.
	if ln6, err := net.Listen("tcp", fmt.Sprintf("[::1]:%d", callbackPort)); err == nil {
		go srv.Serve(ln6)
	}
	return srv, results, nil
}

const closedPage = `<!doctype html><meta charset="utf-8">
<title>%[1]s</title>
<body style="font:16px -apple-system,sans-serif;display:grid;place-items:center;height:90vh;margin:0">
<div style="text-align:center"><h1 style="font-size:20px;margin:0 0 8px">%[1]s</h1>
<p style="color:#666;margin:0">%[2]s</p></div>`

// openBrowser — macOS 전용 앱이다(음악 앱이 AppleScript 를 쓴다).
// 다른 곳으로 갈 일이 생기면 그때 갈래를 만든다.
func openBrowser(u string) error {
	if err := exec.Command("open", u).Start(); err != nil {
		return fmt.Errorf("브라우저를 열지 못했습니다: %w", err)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────
// 코드를 토큰으로
// ─────────────────────────────────────────────────────────────

// oauthResponse 는 oauth.v2.access 가 돌려주는 것 중 우리가 보는 부분이다.
// **사용자 토큰은 authed_user 안에 있다.** 바깥의 access_token 은 봇 것이고,
// desktop redirect 는 봇 권한을 요청할 수 없으므로 우리에게는 오지 않는다.
type oauthResponse struct {
	Team struct {
		Name string `json:"name"`
	} `json:"team"`
	AuthedUser struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	} `json:"authed_user"`
}

func (r oauthResponse) creds() (creds, error) {
	if r.AuthedUser.AccessToken == "" {
		return creds{}, errNoUserTok
	}
	c := creds{
		Access:  r.AuthedUser.AccessToken,
		Refresh: r.AuthedUser.RefreshToken,
		Team:    r.Team.Name,
	}
	// expires_in 이 없으면 만료가 없는 토큰이다. 회전을 켜지 않은 앱의 정상이다.
	if n := r.AuthedUser.ExpiresIn; n > 0 {
		c.Expires = time.Now().Add(time.Duration(n) * time.Second)
	}
	return c, nil
}

// exchange 는 인가 코드를 토큰으로 바꾼다.
//
// client_secret 을 보내지 않는다. PKCE 앱은 public client 이고,
// 그 자리를 code_verifier 가 대신한다.
func exchange(ctx context.Context, id, code, verifier string) (creds, error) {
	var out oauthResponse
	err := unauthenticated().call(ctx, "oauth.v2.access", url.Values{
		"client_id":     {id},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {redirectURI()},
	}, &out)
	if err != nil {
		return creds{}, err
	}
	return out.creds()
}

// refresh 는 만료된 토큰을 갱신한다.
//
// 회전을 켠 앱에서만 일어난다. 리프레시 토큰 자체도 새것으로 바뀌므로
// 받은 것을 반드시 다시 저장해야 한다 — 안 하면 다음 갱신에서 로그아웃된다.
func refresh(ctx context.Context, id, refreshToken string) (creds, error) {
	var out oauthResponse
	err := unauthenticated().call(ctx, "oauth.v2.access", url.Values{
		"client_id":     {id},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}, &out)
	if err != nil {
		return creds{}, err
	}
	c, err := out.creds()
	if err != nil {
		return creds{}, err
	}
	// 갱신 응답에 리프레시 토큰이 없으면 쓰던 것을 계속 쓴다.
	if c.Refresh == "" {
		c.Refresh = refreshToken
	}
	return c, nil
}

// cmdLogout 은 저장된 토큰을 지운다.
func cmdLogout() tea.Cmd {
	return func() tea.Msg { return logoutMsg{Err: clearCreds()} }
}

// cmdEnsureToken 은 필요하면 갱신한 뒤 대화 목록을 읽는다.
//
// 켤 때 한 번 부른다. 만료가 없는 토큰이면 아무것도 안 하고 지나간다.
func cmdEnsureToken(c creds) tea.Cmd {
	if !c.expired() || c.Refresh == "" {
		return cmdLoad(c.Access)
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
		defer cancel()
		next, err := refresh(ctx, oauthClientID(), c.Refresh)
		if err != nil {
			return loginMsg{Err: err}
		}
		if err := saveCreds(next); err != nil {
			return loginMsg{Err: err}
		}
		return loginMsg{Creds: next}
	}
}
