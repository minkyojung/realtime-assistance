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
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
)

// 라이브러리가 200곡 남짓이라 전곡을 그대로 컨텍스트에 넣고 한 번에 부른다.
// 의도 해석·선곡·근거 생성을 나누지 않는 이유는, 나누면 왕복이 세 번이 되고
// 뒤 단계가 앞 단계의 맥락을 잃기 때문이다.
//
// 라이브러리가 수천 곡으로 커지면 사전 태깅(L1)으로 후보를 먼저 좁혀야 한다.
const model = openai.ChatModelGPT5_5

// Model 은 로그 상세에 무엇으로 골랐는지 적을 때 쓴다.
func Model() string { return model }

// 지연 예산이 UX 의 전부다. docs/03 8절.
var effort = shared.ReasoningEffortLow

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

	// Elapsed 는 이 요청에 걸린 시간이다. 로그 상세에 쓴다.
	Elapsed time.Duration

	// Candidates 는 고르기 전 후보가 몇 곡이었는지다.
	Candidates int
}

// 응답 스키마. strict 모드에서 모델이 이 모양을 벗어날 수 없다.
//
// strict 는 모든 객체에 additionalProperties:false 와, properties 전부를
// required 에 넣을 것을 요구한다. 선택 필드를 두려면 타입에 "null" 을 넣어야 한다.
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
						"description": "The line number of the track in the library listing (the first column). Never invent one.",
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
- Read the signals column as evidence, not as a verdict. A skip count does not
  say why a track was skipped, and this tool's own queue edits raise it too.
  A few skips on a track with many plays means little. Skips on a track with
  no plays mean they keep turning it down — that is not a track to give another
  chance. Favorites are the one signal they set on purpose; lean on them.
- Write title, note and reasons in the same language the person used.`

// Current 는 이미 화면에 있는 큐다. 있으면 새로 만드는 대신 고칠 수 있다.
type Current struct {
	Title   string
	Items   []api.QueueItem
	Playing int64 // 지금 재생 중인 곡의 id. 0 이면 없음
}

// Build 는 자연어 한 줄을 큐로 바꾼다.
//
// cur 이 비어 있지 않으면 그것을 함께 넘긴다. "좀 더 조용한 걸로" 같은
// 요청은 새 큐가 아니라 지금 큐를 고치라는 뜻이기 때문이다.
// 무엇이 요청인지는 모델이 문장을 보고 판단한다.
func Build(ctx context.Context, prompt string, library []api.Track, cur Current, now time.Time) (Result, error) {
	if strings.TrimSpace(prompt) == "" {
		return Result{}, fmt.Errorf("nothing to ask for")
	}

	start := time.Now()
	client := openai.NewClient()
	lst := renderLibrary(library, now)

	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: model,
		// 선곡은 긴 추론이 필요한 일이 아니다. 목록을 읽고 조건에 맞는 것을
		// 고르는 작업이라 추론을 낮추면 지연이 크게 줄어든다.
		ReasoningEffort: effort,
		// 시스템 프롬프트와 라이브러리 목록을 먼저 두어야 프롬프트 캐시가 걸린다.
		Messages: messages(prompt, lst, cur),
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
				JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:   "queue",
					Strict: openai.Bool(true),
					Schema: schema,
				},
			},
		},
	})
	if err != nil {
		return Result{}, err
	}

	var out Result
	if err := decode(resp, &out); err != nil {
		return Result{}, err
	}
	// 줄번호를 진짜 id 로 되돌린다. 범위를 벗어난 번호는 0 으로 두면
	// applyQueue 가 조용히 버린다.
	for i := range out.Picks {
		n := out.Picks[i].TrackID
		if n < 1 || int(n) > len(lst.ids) {
			out.Picks[i].TrackID = 0
			continue
		}
		out.Picks[i].TrackID = lst.ids[n-1]
	}
	out.Usage = api.Usage{
		PromptTokens:     int(resp.Usage.PromptTokens),
		CompletionTokens: int(resp.Usage.CompletionTokens),
		CostUsd:          cost(resp.Usage),
	}
	out.Elapsed = time.Since(start)
	out.Candidates = len(library)
	return out, nil
}

// 시스템 프롬프트와 라이브러리는 요청마다 같으므로 앞에 둔다. 그래야 캐시가 걸린다.
// 현재 큐는 매번 달라지므로 뒤에 붙인다.
func messages(prompt string, lst listing, cur Current) []openai.ChatCompletionMessageParamUnion {
	out := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.SystemMessage(lst.text),
	}
	if len(cur.Items) > 0 {
		ord := make(map[int64]int, len(lst.ids))
		for i, id := range lst.ids {
			ord[id] = i + 1
		}
		out = append(out, openai.SystemMessage(renderCurrent(cur, ord)))
	}
	return append(out, openai.UserMessage(prompt))
}

// renderCurrent 는 지금 큐를 적는다.
func renderCurrent(cur Current, ord map[int64]int) string {
	var b strings.Builder
	b.WriteString("A queue is already on screen")
	if cur.Title != "" {
		fmt.Fprintf(&b, " — %q", cur.Title)
	}
	b.WriteString(".\nIf the request reads as an adjustment (\"quieter\", \"drop this artist\",\n" +
		"\"shorter\"), return the adjusted queue rather than an unrelated one, and keep\n" +
		"the track that is playing in place unless the request rules it out.\n" +
		"If the request reads as a fresh ask, ignore this queue.\n\n")
	for _, it := range cur.Items {
		mark := " "
		if it.Track.Id == cur.Playing {
			mark = "▶"
		}
		// 목록에 없는 곡(라이브러리 밖)은 번호가 없다. 그런 곡은 번호 없이 적는다.
		n, ok := ord[it.Track.Id]
		if !ok {
			fmt.Fprintf(&b, "%s - | %s | %s\n", mark, it.Track.Title, it.Track.Artist.Name)
			continue
		}
		fmt.Fprintf(&b, "%s %d | %s | %s\n", mark, n, it.Track.Title, it.Track.Artist.Name)
	}
	return b.String()
}

// renderLibrary 는 곡을 한 줄씩 적는다.
//
// 재생 횟수와 마지막 재생일을 반드시 넣는다. 근거를 사실로 쓰게 하려면
// 모델이 그 사실을 볼 수 있어야 한다.
//
// signals 칸은 스킵 횟수와 좋아요를 한 칸에 담는다. 둘을 따로 두지 않는
// 이유는 값이 드물기 때문이다 — 실측 202곡에서 스킵이 있는 곡은 63곡,
// 좋아요는 20곡이다. 칸을 나누면 139줄이 "0 skips" 라는 소음이 되고
// 토큰도 3.5배 든다(실측 +742 대 +342).
//
// 별점은 넣지 않는다. 실측에서 202곡 전부 0 이었다 — Music.app 이 값을
// 돌려주지 않는다. 없는 값을 칸으로 만들면 모델이 "아무도 별점을 안 줬다"는
// 사실이 아닌 것을 읽는다. docs/05 가 loved·bpm 을 뺀 것과 같은 이유다.
// listing 은 모델에게 준 목록이다. 줄번호(1..N)가 곧 모델이 돌려줄 trackId 다.
//
// 진짜 id 를 그대로 주지 않는 이유: persistent ID 를 수로 읽으므로 19자리가 된다.
// 모델이 옮겨 적다 틀리기 쉽고 토큰도 비싸다. 줄번호는 짧고, 되돌리는 표를
// 우리가 갖고 있으므로 요청 도중에 라이브러리가 갈려도 옳은 곡을 가리킨다.
type listing struct {
	text string
	ids  []int64 // 줄번호-1 → 진짜 Track.Id
}

func renderLibrary(tracks []api.Track, now time.Time) listing {
	var b strings.Builder
	ids := make([]int64, 0, len(tracks))
	b.WriteString("The person's library. Columns: n | title | artist | album | genre | year | length | plays | signals | last played | added\n")
	b.WriteString("Use the n column as trackId when you pick a track.\n")
	b.WriteString("\"added: not in library\" means the track sits in a playlist but was never added to the library, so no add date exists. Never claim a date for those.\n")
	b.WriteString("signals is what their player recorded: skip count, and whether they marked it a favorite. \"-\" means neither.\n\n")
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
		// 라이브러리에 없는 곡은 담은 날짜가 없다. 지어내지 않는다 —
		// 없는 값을 채우면 근거가 사실이 아니게 된다.
		added := "not in library"
		if t.AddedAt != nil {
			added = fmt.Sprintf("%dd ago", int(now.Sub(*t.AddedAt).Hours()/24))
		}
		ids = append(ids, t.Id)
		fmt.Fprintf(&b, "%d | %s | %s | %s | %s | %s | %s | %d plays | %s | %s | %s\n",
			len(ids), t.Title, t.Artist.Name, album, genre, year,
			mmss(t.DurationMs), t.PlayCount, signals(t), last, added)
	}
	return listing{text: b.String(), ids: ids}
}

// signals 는 플레이어가 기록해 둔 것을 한 칸에 적는다.
//
// 값이 없으면 "-" 다. 빈 칸으로 두면 칸이 밀려 다음 값이 이 자리로 읽힌다.
func signals(t api.Track) string {
	var out []string
	if t.SkipCount > 0 {
		out = append(out, fmt.Sprintf("%d skips", t.SkipCount))
	}
	if t.Favorited {
		out = append(out, "favorite")
	}
	if len(out) == 0 {
		return "-"
	}
	return strings.Join(out, ", ")
}

func mmss(ms int) string {
	s := ms / 1000
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// gpt-5.5 기준 대략치. 정확한 청구는 사용량 대시보드를 본다.
const (
	usdPerMTokIn  = 1.25
	usdPerMTokOut = 10.0
)

func cost(u openai.CompletionUsage) float64 {
	cached := u.PromptTokensDetails.CachedTokens
	fresh := u.PromptTokens - cached
	// 캐시된 입력은 크게 싸다.
	in := (float64(fresh) + float64(cached)*0.1) / 1e6 * usdPerMTokIn
	out := float64(u.CompletionTokens) / 1e6 * usdPerMTokOut
	return in + out
}
