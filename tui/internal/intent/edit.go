package intent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"amcli/tui/internal/api"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
)

// 큐를 고치는 말과 새로 짜라는 말은 값이 다르다.
//
// "조용한 거 틀어줘"는 라이브러리 157곡을 다 읽고 골라야 하니 7초가 맞는
// 값이다. 그런데 "이 곡 빼줘"도 같은 값을 치르고 있었다 — Build 가 언제나
// 목록 전체를 돌려주는 한, 한 곡을 빼려고 나머지를 전부 다시 고르게 된다.
//
// 그래서 큐가 이미 있을 때만 먼저 물어본다. **무엇을 할 셈인가.**
// 이 물음에는 라이브러리가 필요 없다. 화면의 큐와 문장 한 줄이면 된다.
//
//	고치라는 말  → 그 자리에서 처리한다 (사람이 /remove 를 눌렀을 때와 같은 길)
//	새로 짜라는 말 → 지금까지처럼 Build 로 간다
//
// 실측 지연 약 1초. 새로 짜는 요청에는 그만큼이 얹히지만, 고치는 요청은
// 7초에서 1초가 된다. 이 제품에서 사람이 하는 주된 일이 "큐를 짓고 고치는
// 대화"이므로(docs/03) 그 쪽이 훨씬 자주 일어난다.

// 판단은 추론이 아니라 분류다. 라우터와 같은 이유로 작은 모델을 쓴다.
const editModel = openai.ChatModelGPT5_4Mini

// EditKind 는 문장이 무엇을 시키는지다.
type EditKind int

const (
	// EditNew — 새 큐를 짜라는 말. Build 로 간다.
	EditNew EditKind = iota
	// EditRemove — 지금 큐에서 곡을 빼라는 말.
	EditRemove
)

// Edit 는 큐를 고치라는 판단이다.
type Edit struct {
	Kind EditKind

	// TrackIDs 는 뺄 곡이다. EditRemove 일 때만 채워진다.
	TrackIDs []int64

	// Note 는 사람에게 할 한 마디다. 로그에 남는다.
	Note string

	Usage   api.Usage
	Elapsed time.Duration
}

var editSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required":             []string{"action", "positions", "note"},
	"properties": map[string]any{
		"action": map[string]any{
			"type": "string",
			"enum": []string{"new", "remove"},
			"description": "\"remove\" only when the sentence asks to drop tracks that are " +
				"already in the queue. Anything that asks for different or more music is \"new\".",
		},
		"positions": map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "integer"},
			"description": "Queue positions to drop, from the numbered list. Empty when action is \"new\".",
		},
		"note": map[string]any{
			"type":        "string",
			"maxLength":   90,
			"description": "One short line to the person, in the language they used. Empty when action is \"new\".",
		},
	},
}

// 틀렸을 때의 값이 한쪽으로 크게 기울어 있다. 그것을 프롬프트에 적는다.
const editPrompt = `A queue is already playing. Read one sentence and decide what it asks for.

- "remove" — the sentence points at tracks already in the queue and asks to drop
  them. "빼줘", "이건 별로", "drop this", "without the loud one".
  List their positions from the numbered queue.
- "new" — anything else. A different mood, more tracks, a shorter set, another
  artist, or a request that has nothing to do with the queue.

**When unsure, answer "new".** Rebuilding the queue is merely slow. Dropping a
track the person did not mean costs them something they cannot get back by
waiting.

Answer about the queue you are shown. Never invent a position.`

// Triage 는 문장이 큐를 고치라는 말인지 본다.
//
// 라이브러리를 넘기지 않는다. 그것이 이 함수가 싼 이유의 전부다.
func Triage(ctx context.Context, prompt string, cur Current) (Edit, error) {
	if strings.TrimSpace(prompt) == "" {
		return Edit{}, fmt.Errorf("nothing to ask for")
	}
	// 고칠 큐가 없으면 물어볼 것도 없다.
	if len(cur.Items) == 0 {
		return Edit{Kind: EditNew}, nil
	}

	start := time.Now()
	client := openai.NewClient()
	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:           editModel,
		ReasoningEffort: shared.ReasoningEffortLow,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(editPrompt),
			openai.SystemMessage(renderQueue(cur)),
			openai.UserMessage(prompt),
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
				JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:   "edit",
					Strict: openai.Bool(true),
					Schema: editSchema,
				},
			},
		},
	})
	if err != nil {
		return Edit{}, err
	}

	var out struct {
		Action    string `json:"action"`
		Positions []int  `json:"positions"`
		Note      string `json:"note"`
	}
	if err := decode(resp, &out); err != nil {
		return Edit{}, err
	}

	e := Edit{
		Note: out.Note,
		Usage: api.Usage{
			PromptTokens:     int(resp.Usage.PromptTokens),
			CompletionTokens: int(resp.Usage.CompletionTokens),
			CostUsd:          cost(resp.Usage),
		},
		Elapsed: time.Since(start),
	}
	if out.Action != "remove" {
		return e, nil
	}

	e.TrackIDs = idsAt(cur, out.Positions)
	// 뺄 곡을 하나도 못 짚었으면 고치라는 말이 아니었던 셈이다.
	// 빈 손으로 "뺐습니다"라고 말하느니 새로 짜는 쪽이 낫다.
	if len(e.TrackIDs) == 0 {
		return Edit{Kind: EditNew, Usage: e.Usage, Elapsed: e.Elapsed}, nil
	}
	e.Kind = EditRemove
	return e, nil
}

// idsAt 은 큐 자리번호를 진짜 곡 id 로 되돌린다.
//
// 자리번호로 주고받는 이유는 Build 와 같다 — persistent ID 는 19자리라
// 모델이 옮겨 적다 틀린다. 범위를 벗어난 번호는 버린다. 스키마가 막지
// 못하는 유일한 구멍이고, 여기서 틀리면 엉뚱한 곡이 사라진다.
func idsAt(cur Current, positions []int) []int64 {
	seen := make(map[int]bool, len(positions))
	out := make([]int64, 0, len(positions))
	for _, n := range positions {
		if n < 1 || n > len(cur.Items) || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, cur.Items[n-1].Track.Id)
	}
	return out
}

// renderQueue 는 지금 큐를 자리번호와 함께 적는다.
//
// 라이브러리 목록(renderLibrary)과 달리 재생 횟수나 날짜를 적지 않는다.
// 여기서 답할 물음은 "무엇을 뺄까"이지 "무엇이 좋을까"가 아니다.
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
