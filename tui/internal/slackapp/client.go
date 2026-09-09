package slackapp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Slack Web API 를 부르는 얇은 층. 라이브러리를 쓰지 않는 이유는 우리가 쓰는
// 메서드가 여섯 개뿐이고, slack-go 는 그 여섯 개를 위해 RTM·소켓·블록킷까지
// 전부 끌고 오기 때문이다.
//
// 통신 방식은 앱이 자기 안에서 감춘다. 호스트는 이 파일의 존재를 모른다 —
// docs/07-호스트-계약.md 7절.

const slackAPI = "https://slack.com/api/"

// 호출 하나가 화면을 오래 잡고 있으면 안 된다. 전부 Cmd 안에서 돌지만
// 무기한 기다리는 것과 실패하는 것 중에서는 실패하는 쪽이 낫다.
const callTimeout = 15 * time.Second

type webClient struct {
	token string
	hc    *http.Client
}

func newClient(token string) webClient {
	return webClient{token: token, hc: &http.Client{Timeout: callTimeout}}
}

// unauthenticated 는 OAuth 흐름 전용이다. 토큰을 받기 전에 부르는 곳.
func unauthenticated() webClient { return newClient("") }

// apiError 는 HTTP 는 200 인데 ok:false 로 온 실패다.
// Slack 은 오류를 상태 코드가 아니라 본문으로 말한다.
type apiError struct {
	Method string
	Code   string
}

func (e apiError) Error() string {
	if hint, ok := errorHints[e.Code]; ok {
		return fmt.Sprintf("Slack %s: %s (%s)", e.Method, hint, e.Code)
	}
	return fmt.Sprintf("Slack %s: %s", e.Method, e.Code)
}

// 사용자가 고칠 수 있는 실패만 우리말로 바꾼다.
// 나머지는 코드를 그대로 보여주는 편이 검색하기 좋다.
var errorHints = map[string]string{
	"invalid_auth":           "토큰이 유효하지 않습니다",
	"token_revoked":          "토큰이 취소되었습니다. 다시 발급하세요",
	"account_inactive":       "비활성 계정의 토큰입니다",
	"missing_scope":          "권한(scope)이 모자랍니다. 앱 설정을 고치고 /login 을 다시 하세요",
	"not_allowed_token_type": "이 메서드에 맞지 않는 종류의 토큰입니다",
	"channel_not_found":      "그 대화를 찾을 수 없습니다",
	"not_in_channel":         "그 채널에 들어가 있지 않습니다",
	"ratelimited":            "호출이 너무 잦습니다",
}

func asAPIError(err error, out *apiError) bool {
	e, ok := err.(apiError)
	if ok {
		*out = e
	}
	return ok
}

// call 은 메서드 하나를 부른다.
//
// Slack 은 전부 POST + form 이고 토큰은 헤더로 보낸다(본문에 넣는 것은
// 로그에 남기 쉬워 권장되지 않는다).
func (c webClient) call(ctx context.Context, method string, form url.Values, out any) error {
	body, err := c.post(ctx, method, form)
	if err != nil {
		return err
	}

	var head struct {
		Ok    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &head); err != nil {
		return fmt.Errorf("Slack %s: 응답을 읽을 수 없습니다: %w", method, err)
	}
	if !head.Ok {
		return apiError{Method: method, Code: head.Error}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("Slack %s: 응답을 읽을 수 없습니다: %w", method, err)
	}
	return nil
}

// 429 는 예외가 아니라 정상 응답이다. Retry-After 를 지키고 한 번만 다시 건다.
// 두 번 이상 미루면 화면이 멈춘 것처럼 보인다.
const retryCap = 5 * time.Second

func (c webClient) post(ctx context.Context, method string, form url.Values) ([]byte, error) {
	if form == nil {
		form = url.Values{}
	}
	encoded := form.Encode()

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, slackAPI+method,
			strings.NewReader(encoded))
		if err != nil {
			return nil, err
		}
		// oauth.v2.access 는 토큰을 받으러 가는 길이라 보낼 토큰이 없다.
		// 빈 Bearer 를 보내면 Slack 이 invalid_auth 로 막는다.
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")

		resp, err := c.hc.Do(req)
		if err != nil {
			return nil, fmt.Errorf("Slack %s: %w", method, err)
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("Slack %s: %w", method, readErr)
		}

		if resp.StatusCode == http.StatusTooManyRequests && attempt == 0 {
			wait := retryAfter(resp.Header.Get("Retry-After"))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("Slack %s: HTTP %d", method, resp.StatusCode)
		}
		return body, nil
	}
}

func retryAfter(h string) time.Duration {
	n, err := strconv.Atoi(strings.TrimSpace(h))
	if err != nil || n <= 0 {
		return time.Second
	}
	d := time.Duration(n) * time.Second
	if d > retryCap {
		return retryCap
	}
	return d
}

// ─────────────────────────────────────────────────────────────
// 우리가 쓰는 여섯 개
// ─────────────────────────────────────────────────────────────

type authInfo struct {
	Team   string `json:"team"`
	UserID string `json:"user_id"`
}

func (c webClient) authTest(ctx context.Context) (authInfo, error) {
	var out authInfo
	err := c.call(ctx, "auth.test", nil, &out)
	return out, err
}

// rawConversation 은 users.conversations 가 돌려주는 모양 그대로다.
type rawConversation struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	IsIM   bool   `json:"is_im"`
	IsMPIM bool   `json:"is_mpim"`
	User   string `json:"user"` // IM 일 때 상대방
}

// 어떤 종류의 대화를 가져올지, 그리고 그 종류마다 필요한 권한.
//
// **둘은 반드시 같이 움직인다.** types 에 넣었는데 권한이 없으면 Slack 은
// 그것만 빼주는 게 아니라 호출 전체를 missing_scope 로 막는다. 한쪽만
// 고치면 로그인은 되는데 목록이 안 뜨는 상태가 된다.
var conversationScopes = map[string]string{
	"public_channel":  "channels:read",
	"private_channel": "groups:read",
	"im":              "im:read",
	"mpim":            "mpim:read",
}

// 요청 순서를 고정한다. map 순회는 순서가 없다.
var conversationTypes = []string{"public_channel", "private_channel", "im", "mpim"}

// myConversations 는 사용자가 속한 대화를 가져온다.
//
// conversations.list 가 아니라 users.conversations 인 이유: 워크스페이스의
// 모든 채널이 아니라 **이 사람이 들어가 있는 것**만 필요하기 때문이다.
func (c webClient) myConversations(ctx context.Context, limit int) ([]rawConversation, error) {
	var out struct {
		Channels []rawConversation `json:"channels"`
	}
	err := c.call(ctx, "users.conversations", url.Values{
		"types":            {strings.Join(conversationTypes, ",")},
		"exclude_archived": {"true"},
		"limit":            {strconv.Itoa(limit)},
	}, &out)
	return out.Channels, err
}

type convInfo struct {
	Name   string
	Unread int
	Latest struct {
		Text string
		User string
		At   time.Time
	}
}

// conversationInfo 는 대화 하나의 세부를 본다.
//
// **안 읽음 개수는 여기서만 나온다.** 그것도 DM 에만 붙는다 — 채널의
// 안 읽음은 Slack 이 어떤 API 로도 주지 않는다. 그래서 이 앱의 배지는
// "DM 의 안 읽음 + 이 앱이 켜진 뒤 도착한 것"이다.
func (c webClient) conversationInfo(ctx context.Context, id string) (convInfo, error) {
	var out struct {
		Channel struct {
			Name               string          `json:"name"`
			User               string          `json:"user"`
			UnreadCountDisplay int             `json:"unread_count_display"`
			Latest             json.RawMessage `json:"latest"`
		} `json:"channel"`
	}
	if err := c.call(ctx, "conversations.info", url.Values{"channel": {id}}, &out); err != nil {
		return convInfo{}, err
	}

	info := convInfo{Name: out.Channel.Name, Unread: out.Channel.UnreadCountDisplay}
	// latest 는 없을 수도, false 일 수도 있다. 모양이 다르면 그냥 비워 둔다.
	var latest struct {
		Text string `json:"text"`
		User string `json:"user"`
		Ts   string `json:"ts"`
	}
	if json.Unmarshal(out.Channel.Latest, &latest) == nil {
		info.Latest.Text = latest.Text
		info.Latest.User = latest.User
		info.Latest.At = parseTS(latest.Ts)
	}
	return info, nil
}

// userName 은 표시할 이름 하나를 가져온다.
// display_name 이 비어 있는 계정이 흔해서 세 가지를 순서대로 본다.
func (c webClient) userName(ctx context.Context, id string) (string, error) {
	var out struct {
		User struct {
			Name    string `json:"name"`
			Profile struct {
				DisplayName string `json:"display_name"`
				RealName    string `json:"real_name"`
			} `json:"profile"`
		} `json:"user"`
	}
	if err := c.call(ctx, "users.info", url.Values{"user": {id}}, &out); err != nil {
		return "", err
	}
	for _, s := range []string{out.User.Profile.DisplayName, out.User.Profile.RealName, out.User.Name} {
		if s = strings.TrimSpace(s); s != "" {
			return s, nil
		}
	}
	return "", nil
}

func (c webClient) postMessage(ctx context.Context, channel, text string) error {
	return c.call(ctx, "chat.postMessage", url.Values{
		"channel": {channel},
		"text":    {text},
	}, nil)
}

func (c webClient) setSnooze(ctx context.Context, minutes int) error {
	return c.call(ctx, "dnd.setSnooze", url.Values{
		"num_minutes": {strconv.Itoa(minutes)},
	}, nil)
}

func (c webClient) endSnooze(ctx context.Context) error {
	return c.call(ctx, "dnd.endSnooze", nil, nil)
}

// parseTS 는 Slack 의 "1757400000.000200" 을 시각으로 바꾼다.
// 읽을 수 없으면 영시각을 돌려주고, 화면은 그 칸을 비운다.
func parseTS(ts string) time.Time {
	sec, frac, _ := strings.Cut(ts, ".")
	s, err := strconv.ParseInt(sec, 10, 64)
	if err != nil {
		return time.Time{}
	}
	var micro int64
	if n, err := strconv.ParseInt(frac, 10, 64); err == nil {
		micro = n
	}
	return time.Unix(s, micro*1000)
}
