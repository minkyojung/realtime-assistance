package slackapp

import (
	"context"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

// 2단계 — 읽기.
//
// # 왜 이만큼만 부르는가
//
// Slack 은 "안 읽음"을 한 번에 주는 API 가 없다. conversations.info 가
// 대화 하나씩 알려주고, 그것도 **DM 에만** 붙는다. 대화가 백 개면 호출이
// 백 번이고 Tier 3(분당 50회)에 걸린다.
//
// 그래서 켤 때 한 번, DM 앞쪽 몇 개만 확인한다. 나머지는 3단계가 채운다 —
// **켠 뒤에 오는 것은 웹소켓이 알려주므로 다시 물어볼 이유가 없다.**
// 폴링을 안 하는 것이 이 앱과 음악 앱의 차이다.
const (
	convLimit    = 200 // users.conversations 한 번에 받는 최대
	imProbeLimit = 15  // 켤 때 안 읽음을 확인할 DM 수
)

// 켤 때 부르는 호출이 최대 서른 번쯤 된다. 하나가 느려도 전체가 멈추지
// 않도록 넉넉히 두되, 무기한은 아니다.
const loadTimeout = 60 * time.Second

// Conversation 은 화면에 한 줄로 그려지는 대화 하나다.
type Conversation struct {
	ID     string
	Name   string // "#general" · "@minkyo"
	IsIM   bool
	Unread int
	Last   string // 마지막 메시지 한 줄
	LastBy string // 그것을 쓴 사람의 id
	At     time.Time
}

// loadedMsg 는 켤 때 한 번 오는 대화 목록이다.
type loadedMsg struct {
	Me    string
	Team  string
	Convs []Conversation
	Users map[string]string
	Err   error
}

// nameMsg 는 뒤늦게 알아낸 이름 하나다.
// 모르는 채널·사람에게서 메시지가 오면 그때 물어본다.
type nameMsg struct {
	Kind string // "user" · "conversation"
	ID   string
	Name string
}

// cmdLoad 는 대화 목록을 가져온다. 입력창을 막지 않도록 Cmd 로 돈다.
func cmdLoad(token string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), loadTimeout)
		defer cancel()

		c := newClient(token)
		auth, err := c.authTest(ctx)
		if err != nil {
			return loadedMsg{Err: err}
		}

		raw, err := c.myConversations(ctx, convLimit)
		if err != nil {
			return loadedMsg{Me: auth.UserID, Team: auth.Team, Err: err}
		}

		convs := make([]Conversation, 0, len(raw))
		users := map[string]string{}
		probed := 0

		for _, r := range raw {
			cv := Conversation{ID: r.ID, IsIM: r.IsIM || r.IsMPIM, Name: channelLabel(r)}

			// DM 만 안 읽음을 알 수 있고, 그것도 앞쪽 몇 개만 본다.
			if r.IsIM && probed < imProbeLimit {
				probed++
				if info, err := c.conversationInfo(ctx, r.ID); err == nil {
					cv.Unread = info.Unread
					cv.Last = info.Latest.Text
					cv.LastBy = info.Latest.User
					cv.At = info.Latest.At
				}
			}
			// 상대 이름은 따로 물어야 한다. 실패해도 id 를 그대로 쓴다 —
			// users:read 가 없는 설치가 흔하고, 그것만으로 앱이 멎으면 안 된다.
			if r.IsIM && r.User != "" {
				if _, ok := users[r.User]; !ok {
					if name, err := c.userName(ctx, r.User); err == nil && name != "" {
						users[r.User] = name
					}
				}
				if name, ok := users[r.User]; ok {
					cv.Name = "@" + name
				}
			}
			convs = append(convs, cv)
		}

		sortConvs(convs)
		return loadedMsg{Me: auth.UserID, Team: auth.Team, Convs: convs, Users: users}
	}
}

// channelLabel 은 이름을 모를 때의 기본값이다.
// 뒤에 users.info 가 성공하면 DM 은 "@사람"으로 바뀐다.
func channelLabel(r rawConversation) string {
	switch {
	case r.IsIM:
		return "@" + r.User
	case r.IsMPIM, r.Name != "":
		return "#" + r.Name
	default:
		return r.ID
	}
}

// 정렬은 켤 때 한 번만 한다.
//
// 메시지가 올 때마다 다시 정렬하면 고르고 있던 줄이 발밑에서 움직인다.
// 숫자는 제자리에서 오르고, 순서는 그대로 둔다.
func sortConvs(cs []Conversation) {
	sort.SliceStable(cs, func(i, j int) bool {
		if (cs[i].Unread > 0) != (cs[j].Unread > 0) {
			return cs[i].Unread > 0
		}
		if cs[i].IsIM != cs[j].IsIM {
			return cs[i].IsIM // 사람이 먼저다. "누가 나 찾았어?"의 답은 대개 DM 이다
		}
		return strings.ToLower(cs[i].Name) < strings.ToLower(cs[j].Name)
	})
}

// cmdUserName · cmdConvName — 모르는 것이 나타났을 때 한 개만 물어본다.
// 실패는 조용히 버린다. 이름이 없으면 id 를 그대로 보여주면 된다.

func cmdUserName(token, id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
		defer cancel()
		name, err := newClient(token).userName(ctx, id)
		if err != nil || name == "" {
			return nil
		}
		return nameMsg{Kind: "user", ID: id, Name: name}
	}
}

func cmdConvName(token, id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
		defer cancel()
		info, err := newClient(token).conversationInfo(ctx, id)
		if err != nil || info.Name == "" {
			return nil
		}
		return nameMsg{Kind: "conversation", ID: id, Name: "#" + info.Name}
	}
}
