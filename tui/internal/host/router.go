package host

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"amcli/tui/internal/secrets"
	tea "charm.land/bubbletea/v2"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

// 라우터 — 한 문장을 받아 어느 앱이 답할지 정한다.
//
// 단어 매칭으로는 안 된다. "조용한 거"에는 음악이라는 단어가 없고
// "누가 나 찾았어?"에는 슬랙이라는 단어가 없다. 의도를 알아들어야 한다.
//
// 모델에게 주는 것은 앱 이름과 한 줄 설명뿐이다. 라이브러리도 대화 기록도
// 주지 않는다. 전부 합쳐 200토큰 남짓이다.
//
// 실측 지연 **약 1초** (gpt-5.4-mini · nano 도 같다 — 네트워크가 지배한다).
// 선곡이 7초이므로 얹어도 체감이 없다.
//
// 그리고 **대부분의 경우 부르지 않는다** — 앱이 하나거나, 사용자가 @ 로
// 지정했거나, 슬래시 명령이면 답이 이미 정해져 있다.

// Spec 은 라우터가 보는 앱의 전부다.
type Spec struct {
	Name        string
	Description string
}

// 라우팅은 판단이 아니라 분류다. 긴 추론이 필요 없다.
const routerModel = openai.ChatModelGPT5_4Mini

var routerSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required":             []string{"apps"},
	"properties": map[string]any{
		"apps": map[string]any{
			"type":        "array",
			"minItems":    0,
			"items":       map[string]any{"type": "string"},
			"description": "Names of the apps that should handle this request, in the order they should run. Empty if none fits.",
		},
	},
}

const routerPrompt = `You route one sentence to the apps that should handle it.

- Answer with app names only, from the list you are given. Never invent one.
- A sentence can need more than one app ("play something quiet and mute slack").
  List every app it needs.
- If the sentence does not clearly belong to any app, return an empty list.
  The host will fall back to whatever the person is looking at.
- Do not explain. Do not answer the request itself.`

// Route 는 문장을 앱 이름 목록으로 바꾼다.
//
// 빈 목록은 "모르겠다"는 뜻이고, 호스트가 지금 보고 있는 앱으로 넘긴다.
func Route(ctx context.Context, prompt string, apps []Spec, current string) ([]string, error) {
	if len(apps) == 0 {
		return nil, fmt.Errorf("앱이 없습니다")
	}
	// 앱이 하나면 물어볼 것이 없다.
	if len(apps) == 1 {
		return []string{apps[0].Name}, nil
	}

	var b strings.Builder
	b.WriteString("apps:\n")
	for _, a := range apps {
		fmt.Fprintf(&b, "  %s — %s\n", a.Name, a.Description)
	}
	if current != "" {
		fmt.Fprintf(&b, "\nnow looking at: %s\n", current)
	}
	fmt.Fprintf(&b, "\nrequest: %q", prompt)

	client := openai.NewClient(option.WithAPIKey(secrets.OpenAIKey()))
	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:           routerModel,
		ReasoningEffort: shared.ReasoningEffortLow,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(routerPrompt),
			openai.UserMessage(b.String()),
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
				JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:   "route",
					Strict: openai.Bool(true),
					Schema: routerSchema,
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("응답이 비어 있습니다")
	}

	var out struct {
		Apps []string `json:"apps"`
	}
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &out); err != nil {
		return nil, fmt.Errorf("응답을 읽을 수 없습니다: %w", err)
	}

	// 모델이 없는 이름을 내놓으면 버린다. 스키마가 막지 못하는 유일한 구멍이다.
	known := make(map[string]bool, len(apps))
	for _, a := range apps {
		known[a.Name] = true
	}
	var kept []string
	for _, n := range out.Apps {
		if known[n] {
			kept = append(kept, n)
		}
	}
	return kept, nil
}

// routeTimeout — 라우터가 느리면 기다리느니 지금 앱에게 넘긴다.
const routeTimeout = 8 * time.Second

// specs 는 지금 담긴 앱들을 라우터가 보는 형태로 바꾼다.
func (m Model) specs() []Spec {
	out := make([]Spec, 0, len(m.apps))
	for _, a := range m.apps {
		out = append(out, Spec{Name: a.Name(), Description: a.Description()})
	}
	return out
}

// mention 은 "@music 조용한 거" 에서 앱 이름과 나머지를 떼어낸다.
// 사용자가 직접 지정했으면 라우터를 부르지 않는다.
func (m Model) mention(prompt string) (string, string) {
	if !strings.HasPrefix(prompt, "@") {
		return "", prompt
	}
	name, rest, _ := strings.Cut(prompt[1:], " ")
	for _, a := range m.apps {
		if strings.EqualFold(a.Name(), name) {
			return a.Name(), strings.TrimSpace(rest)
		}
	}
	return "", prompt
}

// routedMsg 는 라우터의 결과다.
//
// seq 는 "몇 번째 물음의 답인가"다. 취소하고 다시 물어본 사이에 옛 답이
// 도착할 수 있어서, 호스트가 이 번호로 거른다.
type routedMsg struct {
	seq    int
	prompt string
	apps   []string
	err    error
}

// cmdRoute — 라우터를 부른다. 입력창을 막지 않도록 Cmd 로 돈다.
func cmdRoute(seq int, prompt string, apps []Spec, current string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), routeTimeout)
		defer cancel()
		names, err := Route(ctx, prompt, apps, current)
		return routedMsg{seq: seq, prompt: prompt, apps: names, err: err}
	}
}
