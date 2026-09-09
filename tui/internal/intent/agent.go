package intent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"amcli/tui/internal/api"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
)

// 한 문장이 무엇을 시키는지 손으로 갈라 두면, 갈래를 늘릴 때마다 코드를
// 고쳐야 한다. Triage 는 갈래가 둘("새로 짜기"·"빼기")뿐이었고, 셋째가
// 생기는 순간 같은 모양을 또 박게 되어 있었다.
//
// 그래서 갈래를 도구 목록으로 바꾼다. 모델에게 무엇을 부를 수 있는지 알려주고
// 고르게 하면, 갈래를 늘리는 일이 도구를 하나 더 적는 일이 된다.
// **그리고 아무 도구도 안 부르는 선택지가 생긴다** — 잡담이 거기서 나온다.
// 곡을 지어내는 것 말고 달리 답할 길이 없던 문제가 같이 풀린다.
//
// pi(badlogic/pi-mono)의 하네스가 도구 넷으로 버티는 것을 보고 정한 모양이다.
// 도구를 적게 두고 루프를 짧게 잡는다.

var errNoChoices = errors.New("the response had no choices")

// 이 층은 고르기만 한다. 무거운 일은 도구 안에서 벌어지므로 작은 모델로 족하다.
const agentModel = openai.ChatModelGPT5_4Mini

// 추론을 끈다. **끄지 않으면 도구를 못 쓴다.**
//
//	Function tools with reasoning_effort are not supported for gpt-5.4-mini
//	in /v1/chat/completions. (400, 실기 확인)
//
// 다른 층(선곡·라우터)은 구조화 출력만 쓰므로 low 로 둔다. 도구를 쥐여주는
// 것은 여기뿐이고, 여기만 none 이어야 한다. 이 상수를 low 로 되돌리면
// 모든 요청이 400 으로 죽는다.
//
// 잃는 것도 없다. 이 층이 하는 일은 "무엇을 부를까"를 고르는 분류이지
// 따져 보는 일이 아니다.
const agentEffort = shared.ReasoningEffortNone

// EditModel 은 첫 화면이 무엇으로 고치는지 적을 때 쓴다. Model 의 짝이다.
//
// 이름이 "고치는 모델"인 것은 이 층이 큐를 고치는 말을 받아내던 시절의
// 흔적이다. 지금은 도구를 고르는 층이고, 고치는 것도 그중 하나다.
func EditModel() string { return agentModel }

// Tool 은 모델이 부를 수 있는 것 하나다.
type Tool struct {
	Name        string
	Description string

	// Params 는 JSON 스키마다. 인자가 없으면 비워 둔다.
	Params map[string]any
}

// Call 은 모델이 부르겠다고 한 것이다.
//
// Args 를 해석하지 않고 원문 그대로 넘긴다. 무엇을 기대하는지 아는 것은
// 도구를 가진 쪽이지 이 층이 아니다.
type Call struct {
	ID   string
	Name string
	Args string
}

// Step 은 모델을 한 번 부른 결과다.
//
// 도구를 부르겠다고 했거나(Calls), 그냥 말했거나(Text) 둘 중 하나다.
// 둘 다 비어 있으면 할 말이 없다는 뜻이고, 호출자가 턴을 닫는다.
type Step struct {
	Calls   []Call
	Text    string
	Usage   api.Usage
	Elapsed time.Duration
}

// Chat 은 한 턴 동안의 대화다.
//
// 값이다. 복사가 곧 스냅샷이라 걸음마다 들고 다니기 좋고, 도중에 그만두면
// 그냥 버리면 된다. 턴을 넘어 살아남지 않는다 — 지난 대화를 기억하는 것은
// 이 다음 이야기다.
type Chat struct {
	msgs []openai.ChatCompletionMessageParamUnion
}

// NewChat 은 한 문장으로 대화를 연다.
//
// 라이브러리를 넣지 않는다. 이 층이 답할 물음은 "무엇을 할까"이지
// "무엇이 좋을까"가 아니고, 그것이 이 층이 싼 이유의 전부다.
// 곡을 고르는 일은 build_queue 도구 안에서 벌어진다.
func NewChat(prompt string, cur Current, tools []Tool) Chat {
	msgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(agentPrompt),
	}
	if len(cur.Items) > 0 {
		msgs = append(msgs, openai.SystemMessage(renderQueue(cur)))
	}
	return Chat{msgs: append(msgs, openai.UserMessage(prompt))}
}

const agentPrompt = `You are the voice of a tool that plays music from someone's
own Apple Music library. You are talking to the person using it.

Use a tool when the sentence asks you to do something. Answer in words when it
does not — a greeting, a thank you, a question about what you can do. Keep
answers to one or two short lines, in the language the person used.

You cannot do anything outside this library and this queue. When asked for
something else, say so plainly in one line. Do not apologise at length and do
not offer to do it anyway.

Never claim you did something you did not do. If a tool fails, say what failed.`

// Step 은 모델을 한 번 부르고, 그 답을 대화에 붙여 돌려준다.
//
// 답을 붙여 돌려주는 것이 중요하다. 도구 결과는 어느 호출에 대한 답인지
// 짝을 맞춰 보내야 하는데, 그 짝의 한쪽이 이 답이기 때문이다.
func (c Chat) Step(ctx context.Context, tools []Tool) (Chat, Step, error) {
	start := time.Now()

	defs := make([]openai.ChatCompletionToolUnionParam, 0, len(tools))
	for _, t := range tools {
		fn := shared.FunctionDefinitionParam{
			Name:        t.Name,
			Description: openai.String(t.Description),
		}
		if t.Params != nil {
			fn.Parameters = t.Params
		}
		defs = append(defs, openai.ChatCompletionFunctionTool(fn))
	}

	client := openai.NewClient()
	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:           agentModel,
		ReasoningEffort: agentEffort,
		Messages:        c.msgs,
		Tools:           defs,
	})
	if err != nil {
		return c, Step{}, err
	}
	if len(resp.Choices) == 0 {
		return c, Step{}, errNoChoices
	}
	msg := resp.Choices[0].Message

	// 답을 대화에 붙인다. 도구 결과가 뒤따라 붙을 자리다.
	c.msgs = append(append([]openai.ChatCompletionMessageParamUnion{}, c.msgs...), msg.ToParam())

	s := Step{
		Text: strings.TrimSpace(msg.Content),
		Usage: api.Usage{
			PromptTokens:     int(resp.Usage.PromptTokens),
			CompletionTokens: int(resp.Usage.CompletionTokens),
			CostUsd:          cost(resp.Usage),
		},
		Elapsed: time.Since(start),
	}
	for _, tc := range msg.ToolCalls {
		s.Calls = append(s.Calls, Call{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: tc.Function.Arguments,
		})
	}
	return c, s, nil
}

// WithResult 는 도구가 한 일을 대화에 적는다.
//
// **실패도 결과로 적는다.** 예외로 터뜨리면 턴이 거기서 끝나 사람은 무엇이
// 잘못됐는지 모른 채 침묵을 본다. 결과로 돌려주면 모델이 그것을 읽고
// 다시 고르거나, 적어도 무엇이 안 됐는지 말할 수 있다.
func (c Chat) WithResult(callID, result string) Chat {
	c.msgs = append(append([]openai.ChatCompletionMessageParamUnion{}, c.msgs...),
		openai.ToolMessage(result, callID))
	return c
}

// renderQueue 는 지금 큐를 자리번호와 함께 적는다.
//
// 라이브러리 목록(renderLibrary)과 달리 재생 횟수나 날짜를 적지 않는다.
// 이 층이 답할 물음은 "무엇을 할까"이지 "무엇이 좋을까"가 아니다.
func renderQueue(cur Current) string {
	var b strings.Builder
	b.WriteString("The queue on screen")
	if cur.Title != "" {
		fmt.Fprintf(&b, " — %q", cur.Title)
	}
	b.WriteString(" (▶ is playing now):\n")
	for i, it := range cur.Items {
		mark := " "
		if it.Track.Id == cur.Playing {
			mark = "▶"
		}
		fmt.Fprintf(&b, "%s %d | %s | %s\n", mark, i+1, it.Track.Title, it.Track.Artist.Name)
	}
	return b.String()
}
