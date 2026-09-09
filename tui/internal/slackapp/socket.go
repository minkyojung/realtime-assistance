package slackapp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/coder/websocket"
)

// 3단계 — 밀어넣기.
//
// **이 앱의 존재 이유다.** 음악은 우리가 1초마다 물어보지만(폴링) 채팅은
// 저쪽에서 온다. Init 이 받은 send 는 이벤트 루프 **밖에서** 메시지를 넣는
// 유일한 통로이고, 이 파일의 고루틴이 그것을 쓴다.
//
// 하네스가 옳게 그어졌는지도 여기서 증명된다 — 음악만 보고 만들었으면
// 이 통로가 없었을 것이고, 그러면 이 앱을 담을 수 없다.
//
// # 왜 Socket Mode 인가
//
// Events API 는 원래 공개 HTTPS 엔드포인트로 웹훅을 받는다. 터미널 앱에는
// 그런 주소가 없다. Socket Mode 는 같은 이벤트를 우리가 건 웹소켓으로
// 되돌려주므로 서버 없이 받을 수 있다.
//
// RTM 은 후보가 아니다 — 세분화 권한 앱은 쓸 수 없고, 그것을 쓰던 클래식
// 앱은 2026년 11월에 끝난다.

// 조용한 연결이 죽었는지 알 방법은 우리가 물어보는 것뿐이다.
// Slack 이 먼저 끊어도 TCP 는 한참 살아 있는 것처럼 보인다.
const pingInterval = 45 * time.Second

// 재연결 간격. 곧바로 다시 걸면 장애 중인 Slack 을 때리게 된다.
const (
	backoffMin = time.Second
	backoffMax = 30 * time.Second
)

// incomingMsg 는 저쪽에서 온 메시지 하나다. Update 가 받는다.
type incomingMsg struct {
	Channel string
	User    string
	Text    string
	At      time.Time
}

// socketMsg 는 연결 상태가 바뀌었다는 알림이다.
// 실패도 화면에 남는다 — 조용히 끊긴 채로 있는 것이 제일 나쁘다.
type socketMsg struct {
	Connected bool
	Err       error
}

// startSocket 은 끊기면 다시 거는 고루틴을 띄운다.
//
// 앱은 프로그램이 끝날 때까지 살아 있으므로(호스트는 화면만 갈아끼운다)
// 멈추는 통로를 두지 않는다. 프로세스가 끝나면 같이 끝난다.
func startSocket(appToken string, send func(tea.Msg)) {
	go func() {
		backoff := backoffMin
		for {
			connected, err := runSocket(appToken, send)
			send(socketMsg{Connected: false, Err: err})

			// 한 번이라도 붙었었다면 일시적인 끊김이다. 곧바로 다시 건다.
			if connected {
				backoff = backoffMin
			}
			time.Sleep(backoff)
			if backoff *= 2; backoff > backoffMax {
				backoff = backoffMax
			}
		}
	}()
}

// runSocket 은 연결 하나의 수명을 산다.
// 첫 값은 "한 번이라도 붙었는가"다. 재연결 간격을 정하는 데 쓴다.
func runSocket(appToken string, send func(tea.Msg)) (bool, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 웹소켓 주소는 매번 새로 받는다. 한 번 쓰고 버리는 값이다.
	var open struct {
		URL string `json:"url"`
	}
	if err := newClient(appToken).call(ctx, "apps.connections.open", nil, &open); err != nil {
		return false, err
	}

	conn, _, err := websocket.Dial(ctx, open.URL, nil)
	if err != nil {
		return false, err
	}
	defer conn.CloseNow()
	// 기본 32KB 는 첨부가 붙은 이벤트에 모자란다.
	conn.SetReadLimit(4 << 20)

	send(socketMsg{Connected: true})
	go keepAlive(ctx, conn)

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return true, err
		}

		var env envelope
		if json.Unmarshal(data, &env) != nil {
			continue // 모르는 모양은 버린다. 연결은 유지한다
		}
		// 확인을 안 보내면 Slack 이 같은 것을 세 번 다시 보낸다.
		// 처리보다 먼저 보낸다 — 우리 쪽 판단이 재전송의 이유가 되면 안 된다.
		if env.EnvelopeID != "" {
			if err := ack(ctx, conn, env.EnvelopeID); err != nil {
				return true, err
			}
		}

		switch env.Type {
		case "disconnect":
			// 정상적인 교대 신호다. 실패가 아니므로 err 없이 나간다.
			return true, nil
		case "events_api":
			if in, ok := env.incoming(); ok {
				send(in)
			}
		}
	}
}

func ack(ctx context.Context, conn *websocket.Conn, id string) error {
	payload, err := json.Marshal(map[string]string{"envelope_id": id})
	if err != nil {
		return err
	}
	wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return conn.Write(wctx, websocket.MessageText, payload)
}

func keepAlive(ctx context.Context, conn *websocket.Conn) {
	t := time.NewTicker(pingInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Ping(pctx)
			cancel()
			if err != nil {
				// 읽기 루프를 깨운다. 재연결은 바깥이 한다.
				conn.CloseNow()
				return
			}
		}
	}
}

// envelope 은 Socket Mode 가 감싸서 보내는 봉투다.
// 우리가 보는 것은 메시지 이벤트 하나뿐이라 나머지는 읽지 않는다.
type envelope struct {
	Type       string `json:"type"` // hello · events_api · disconnect · …
	EnvelopeID string `json:"envelope_id"`
	Payload    struct {
		Event struct {
			Type    string `json:"type"`
			Subtype string `json:"subtype"`
			Channel string `json:"channel"`
			User    string `json:"user"`
			BotID   string `json:"bot_id"`
			Text    string `json:"text"`
			Ts      string `json:"ts"`
		} `json:"event"`
	} `json:"payload"`
}

// incoming 은 봉투에서 우리가 셀 메시지만 꺼낸다.
//
// 무엇을 버리는지가 배지의 정확도를 정한다. 수정·삭제·입장 같은
// subtype 은 새 메시지가 아니고, 봇이 쓴 것은 사람이 나를 찾은 것이 아니다.
// 내가 쓴 것은 여기서 못 거른다 — 내 id 를 아는 곳은 Update 뿐이다.
func (e envelope) incoming() (incomingMsg, bool) {
	ev := e.Payload.Event
	if ev.Type != "message" || ev.Subtype != "" || ev.BotID != "" {
		return incomingMsg{}, false
	}
	if ev.Channel == "" || ev.User == "" {
		return incomingMsg{}, false
	}
	return incomingMsg{
		Channel: ev.Channel,
		User:    ev.User,
		Text:    strings.TrimSpace(ev.Text),
		At:      parseTS(ev.Ts),
	}, true
}

// 연결 실패를 화면 한 줄로 줄인다. 스택이 아니라 다음에 뭘 할지를 보여준다.
var errSocketScope = errors.New(
	"실시간 수신 실패 — 앱 토큰(xapp-)과 Socket Mode 설정을 확인하세요")

func socketHint(err error) error {
	var e apiError
	if asAPIError(err, &e) && (e.Code == "invalid_auth" || e.Code == "not_allowed_token_type") {
		return errSocketScope
	}
	return err
}
