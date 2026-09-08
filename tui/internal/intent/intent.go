// Package intent 는 이 제품의 본체다.
//
// 기존 미디어 도구들은 `play(track)` 을 노출한다 — 무엇을 틀지 호출자가
// 이미 알고 있어야 한다. 이 패키지는 그 위의 층을 만든다.
//
//	의도를 받고 → 라이브러리 안에서 고르고 → 왜 골랐는지 말한다
//
// 후보 집합은 언제나 사용자의 라이브러리다. 카탈로그로 나가지 않는다.
package intent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"amcli/tui/internal/api"
	"github.com/anthropics/anthropic-sdk-go"
)

// 라이브러리가 200곡 남짓이라 전곡을 그대로 컨텍스트에 넣고 한 번에 부른다.
// 의도 해석·선곡·근거 생성을 나누지 않는 이유는, 나누면 왕복이 세 번이 되고
// 뒤 단계가 앞 단계의 맥락을 잃기 때문이다.
//
// 라이브러리가 수천 곡으로 커지면 사전 태깅(L1)으로 후보를 먼저 좁혀야 한다.
const model = "claude-opus-5"

// Pick 은 고른 곡 하나와 그 근거다.
type Pick struct {
	TrackID int64  `json:"trackId"`
	Reason  string `json:"reason"`
}

// Result 는 한 번의 요청이 만들어낸 큐다.
type Result struct {
	Title string `json:"title"`
	Note  string `json:"note"`
	Picks []Pick `json:"picks"`

	Usage api.Usage
}

// 응답 스키마. 모델이 이 모양을 벗어날 수 없다.
var schema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required":             []string{"title", "note", "picks"},
	"properties": map[string]any{
		"title": map[string]any{
			"type":        "string",
			"maxLength":   60,
			"description": "Short label for this queue, in the user's language.",
		},
		"note": map[string]any{
			"type":        "string",
			"maxLength":   120,
			"description": "One sentence to the user about the queue as a whole.",
		},
		"picks": map[string]any{
			"type":     "array",
			"minItems": 1,
			"maxItems": 30,
			"items": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"trackId", "reason"},
				"properties": map[string]any{
					"trackId": map[string]any{
						"type":        "integer",
						"description": "id from the library listing. Never invent one.",
					},
					"reason": map[string]any{
						"type":      "string",
						"maxLength": 90,
						"description": "Why this track, in one short line. " +
							"Prefer concrete facts from the listing over adjectives. " +
							"Write in the same language the user used.",
					},
				},
			},
		},
	},
}

const systemPrompt = `You pick music from someone's own Apple Music library.

Rules that matter:
- Only choose tracks from the listing you are given. Never invent a track id.
- Respect the shape of the request. If a duration is implied ("an hour",
  "30 minutes"), make the total run time land near it.
- Order matters. The queue should hold together as a sitting, not a shuffle.
- Every pick needs a reason the person can verify. Prefer facts you can see in
  the listing — play counts, when it was added, when it was last played, the
  album it belongs to — over vague mood words.
- Reach for tracks the person owns but rarely plays when the request allows it.
  A library where the same handful of tracks take most of the plays is the
  problem this tool exists to solve. Do not force it when the request is
  specific about something else.
- Write title, note and reasons in the same language the person used.`

// Build 는 자연어 한 줄을 큐로 바꾼다.
func Build(ctx context.Context, prompt string, library []api.Track, now time.Time) (Result, error) {
	if strings.TrimSpace(prompt) == "" {
		return Result{}, fmt.Errorf("빈 요청")
	}

	client := anthropic.NewClient()
	adaptive := anthropic.ThinkingConfigAdaptiveParam{}

	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: 8000,
		Thinking:  anthropic.ThinkingConfigParamUnion{OfAdaptive: &adaptive},
		OutputConfig: anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffortMedium,
			Format: anthropic.JSONOutputFormatParam{Schema: schema},
		},
		System: []anthropic.TextBlockParam{{
			Text: systemPrompt,
			// 시스템 프롬프트와 라이브러리 목록은 요청마다 같으므로 캐시한다.
			CacheControl: anthropic.NewCacheControlEphemeralParam(),
		}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewTextBlock(renderLibrary(library, now)),
				anthropic.NewTextBlock("Request: "+prompt),
			),
		},
	})
	if err != nil {
		return Result{}, err
	}

	var out Result
	if err := decode(resp, &out); err != nil {
		return Result{}, err
	}
	out.Usage = api.Usage{
		PromptTokens:     int(resp.Usage.InputTokens),
		CompletionTokens: int(resp.Usage.OutputTokens),
		CostUsd:          cost(resp.Usage),
	}
	return out, nil
}

// renderLibrary 는 곡을 한 줄씩 적는다.
//
// 재생 횟수와 마지막 재생일을 반드시 넣는다. 근거를 사실로 쓰게 하려면
// 모델이 그 사실을 볼 수 있어야 한다.
func renderLibrary(tracks []api.Track, now time.Time) string {
	var b strings.Builder
	b.WriteString("The person's library. Columns: id | title | artist | album | genre | year | length | plays | last played | added\n\n")
	for _, t := range tracks {
		if t.Excluded {
			continue
		}
		album, genre := "", ""
		if t.Album != nil {
			album = t.Album.Title
		}
		if t.Genre != nil {
			genre = *t.Genre
		}
		year := ""
		if t.Year != nil {
			year = fmt.Sprint(*t.Year)
		}
		last := "never"
		if t.LastPlayedAt != nil {
			last = fmt.Sprintf("%dd ago", int(now.Sub(*t.LastPlayedAt).Hours()/24))
		}
		fmt.Fprintf(&b, "%d | %s | %s | %s | %s | %s | %s | %d plays | %s | %dd ago\n",
			t.Id, t.Title, t.Artist.Name, album, genre, year,
			mmss(t.DurationMs), t.PlayCount, last,
			int(now.Sub(t.AddedAt).Hours()/24))
	}
	return b.String()
}

func mmss(ms int) string {
	s := ms / 1000
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// Claude Opus 5 — 입력 $5 / 출력 $25 per MTok.
func cost(u anthropic.Usage) float64 {
	in := float64(u.InputTokens+u.CacheReadInputTokens/10) / 1e6 * 5
	out := float64(u.OutputTokens) / 1e6 * 25
	return in + out
}
