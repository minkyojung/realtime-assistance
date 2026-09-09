package slackapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
)

// 4단계 — 쓰기.
//
// 라우터가 이 앱을 지목하면 문장이 여기로 온다. 라우터는 "누구 것인가"만
// 정하고, "무엇을 하라는 것인가"는 앱이 정한다 — 그 판단에 필요한 것(대화
// 목록)은 앱 안에만 있기 때문이다.
//
// 고르는 일이지 짓는 일이 아니므로 작은 모델로 충분하다.
// 라우터와 같은 모델·같은 추론량이고, 실측 지연도 1초 남짓이다.
const askModel = openai.ChatModelGPT5_4Mini

const askTimeout = 30 * time.Second

// 후보 대화가 너무 많으면 프롬프트만 커지고 정확도는 안 오른다.
// 목록 앞쪽(안 읽음 · DM)이 답일 확률이 압도적이다.
const askConvLimit = 40

type actionKind string

const (
	actSend    actionKind = "send"
	actDND     actionKind = "dnd"
	actEndDND  actionKind = "enddnd"
	actNothing actionKind = "none"
)

// action 은 모델이 고른 것 하나다.
//
// strict 스키마는 선택 필드를 허용하지 않는다(모든 property 가 required).
// 그래서 안 쓰는 칸은 빈 값으로 온다.
type action struct {
	Action       actionKind `json:"action"`
	Conversation string     `json:"conversation"`
	Text         string     `json:"text"`
	Minutes      int        `json:"minutes"`
	Say          string     `json:"say"`
}

var askSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required":             []string{"action", "conversation", "text", "minutes", "say"},
	"properties": map[string]any{
		"action": map[string]any{
			"type": "string",
			"enum": []string{"send", "dnd", "enddnd", "none"},
			"description": "send: post a message. dnd: turn on do not disturb. " +
				"enddnd: turn it off. none: the request is not something this app can do.",
		},
		"conversation": map[string]any{
			"type": "string",
			"description": "The conversation id from the listing. " +
				"Empty unless the action is send. Never invent one.",
		},
		"text": map[string]any{
			"type":      "string",
			"maxLength": 600,
			"description": "The message to post, written as the user would write it, " +
				"in the user's language. Empty unless the action is send.",
		},
		"minutes": map[string]any{
			"type":        "integer",
			"minimum":     0,
			"maximum":     1440,
			"description": "Minutes of do not disturb. 0 unless the action is dnd.",
		},
		"say": map[string]any{
			"type":      "string",
			"maxLength": 120,
			"description": "One line telling the user what you did or why you did nothing, " +
				"in the same language they used.",
		},
	},
}

const askPrompt = `You act on someone's Slack on their behalf.

Rules that matter:
- Only choose a conversation id from the listing you are given. Never invent one.
- If the request names a person or channel that is not in the listing, choose
  "none" and say which one you could not find.
- When you send a message, write it the way the person would write it — short,
  no signature, no quotes around it, same language as the request.
- "mute", "do not disturb", "quiet", "알림 꺼줘" mean dnd. Default to 60 minutes
  when no duration is given.
- A request that is not about Slack at all is "none". Say so in one line.
  Another app is probably handling it; do not guess.`

// actedMsg 는 4단계의 결과다. 성공도 실패도 로그로 간다.
type actedMsg struct {
	Say string
	Err error
}

var (
	errNoConversations = errors.New("아직 대화 목록을 읽지 못했습니다")
	errUnknownConv     = errors.New("그 대화를 찾을 수 없습니다")
)

// cmdAct 는 문장 하나를 해석하고 실행한다.
//
// 해석과 실행을 한 Cmd 안에서 하는 이유는, 나누면 중간 상태("보내려는 중")가
// 생기고 그것을 화면에 그려야 하기 때문이다. 기다린다는 표시는 호스트가 한다.
func cmdAct(token, prompt string, convs []Conversation) tea.Cmd {
	return func() tea.Msg {
		if len(convs) == 0 {
			return actedMsg{Err: errNoConversations}
		}
		ctx, cancel := context.WithTimeout(context.Background(), askTimeout)
		defer cancel()

		act, err := decideAction(ctx, prompt, convs)
		if err != nil {
			return actedMsg{Err: err}
		}
		return runAction(ctx, newClient(token), act, convs)
	}
}

func decideAction(ctx context.Context, prompt string, convs []Conversation) (action, error) {
	var b strings.Builder
	b.WriteString("conversations:\n")
	for i, c := range convs {
		if i >= askConvLimit {
			break
		}
		fmt.Fprintf(&b, "  %s — %s", c.ID, c.Name)
		if c.Unread > 0 {
			fmt.Fprintf(&b, " (%d unread)", c.Unread)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "\nrequest: %q", prompt)

	client := openai.NewClient()
	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:           askModel,
		ReasoningEffort: shared.ReasoningEffortLow,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(askPrompt),
			openai.UserMessage(b.String()),
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
				JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:   "slack_action",
					Strict: openai.Bool(true),
					Schema: askSchema,
				},
			},
		},
	})
	if err != nil {
		return action{}, err
	}
	if len(resp.Choices) == 0 {
		return action{}, errors.New("응답이 비어 있습니다")
	}

	var out action
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &out); err != nil {
		return action{}, fmt.Errorf("응답을 읽을 수 없습니다: %w", err)
	}
	return out, nil
}

// runAction 은 고른 것을 실제로 한다.
//
// 스키마가 막지 못하는 구멍은 하나뿐이다 — 목록에 없는 대화 id.
// 라우터가 그랬듯 여기서도 모르는 이름은 버린다.
func runAction(ctx context.Context, c webClient, act action, convs []Conversation) actedMsg {
	switch act.Action {
	case actSend:
		cv, ok := findConv(convs, act.Conversation)
		if !ok {
			return actedMsg{Err: errUnknownConv}
		}
		if strings.TrimSpace(act.Text) == "" {
			return actedMsg{Err: errors.New("보낼 내용이 비어 있습니다")}
		}
		if err := c.postMessage(ctx, cv.ID, act.Text); err != nil {
			return actedMsg{Err: err}
		}
		return actedMsg{Say: fmt.Sprintf("%s 에 보냈습니다 — %q", cv.Name, act.Text)}

	case actDND:
		mins := act.Minutes
		if mins <= 0 {
			mins = defaultSnoozeMinutes
		}
		if err := c.setSnooze(ctx, mins); err != nil {
			return actedMsg{Err: err}
		}
		return actedMsg{Say: fmt.Sprintf("%d분 동안 알림을 껐습니다", mins)}

	case actEndDND:
		if err := c.endSnooze(ctx); err != nil {
			return actedMsg{Err: err}
		}
		return actedMsg{Say: "알림을 다시 켰습니다"}

	default:
		say := strings.TrimSpace(act.Say)
		if say == "" {
			say = "그건 제가 할 수 있는 게 아닙니다"
		}
		return actedMsg{Say: say}
	}
}

func findConv(convs []Conversation, id string) (Conversation, bool) {
	for _, c := range convs {
		if c.ID == id {
			return c, true
		}
	}
	return Conversation{}, false
}
