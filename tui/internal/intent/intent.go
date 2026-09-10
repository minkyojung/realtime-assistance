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
	"amcli/tui/internal/data"
	"amcli/tui/internal/secrets"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
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
//
// 둘 중 하나만 찬다. TrackID 가 차 있으면 이미 가진 곡이고, CatalogID 가
// 차 있으면 라이브러리 밖의 곡이다 — 그 곡은 담아야 틀 수 있다.
type Pick struct {
	TrackID   int64  `json:"trackId"`
	CatalogID string `json:"-"`
	Reason    string `json:"reason"`
}

// Result 는 한 번의 요청이 만들어낸 큐다.
type Result struct {
	Title string `json:"title"`
	Note  string `json:"note"`

	// Context 는 모델이 요청을 보고 고른 자리다 — 공부인가 운동인가.
	// 재생 기록과 이어 붙일 때 집계 키가 된다(data/turns.go).
	Context data.Context `json:"context"`

	Picks []Pick `json:"picks"`

	Usage api.Usage

	// Elapsed 는 이 요청에 걸린 시간이다. 로그 상세에 쓴다.
	Elapsed time.Duration

	// Candidates 는 고르기 전 후보가 몇 곡이었는지다.
	Candidates int
}

// contextEnum 은 data.Contexts 를 스키마가 읽는 모양으로 바꾼다.
//
// 값을 새로 적지 않는다는 것이 요점이다. 목록이 늘면 여기는 저절로 따라온다.
func contextEnum() []string {
	out := make([]string, len(data.Contexts))
	for i, c := range data.Contexts {
		out[i] = string(c)
	}
	return out
}

// 응답 스키마. strict 모드에서 모델이 이 모양을 벗어날 수 없다.
//
// strict 는 모든 객체에 additionalProperties:false 와, properties 전부를
// required 에 넣을 것을 요구한다. 선택 필드를 두려면 타입에 "null" 을 넣어야 한다.
var schema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required":             []string{"title", "note", "context", "picks"},
	"properties": map[string]any{
		"title": map[string]any{
			"type":        "string",
			"maxLength":   60,
			"description": "Short label for this queue, in the user's language.",
		},
		// maxLength 는 **안전망이지 가위가 아니다.**
		//
		// 한때 120 이었다. 모델이 그 길이를 목표로 삼지 않으니 딱 걸리는 일이
		// 생겼고, 걸리면 문장이 말 끝에서 잘렸다 — "…with no skip history so".
		// 잘린 문장은 틀린 문장보다 나쁘다. 무슨 말을 하려던 건지 모르는 채로
		// 화면에 남는다.
		//
		// 그래서 짧게 쓰라는 말은 설명에 두고, 상한은 문장이 끝날 자리를 남겨
		// 둔다. 화면은 어차피 접어서 그린다(style.Wrap).
		"note": map[string]any{
			"type":      "string",
			"maxLength": 200,
			"description": "One short sentence to the user about the queue as a whole. " +
				"Aim for under 100 characters and finish the sentence.",
		},
		// 목록을 여기 적지 않는다. data.Contexts 가 유일한 목록이고 이것은
		// 그것을 옮겨 담을 뿐이다. 두 곳에 적으면 한쪽만 늘어나는 날이 오고,
		// 그러면 모델이 우리가 모르는 값을 돌려준다.
		"context": map[string]any{
			"type": "string",
			"enum": contextEnum(),
			"description": "The single activity the person named in their request. " +
				"focus: work, study, reading, anything needing concentration. " +
				"workout: exercise or training. " +
				"chores: housework, cooking, errands — hands busy, mind free. " +
				"commute: travelling between places. " +
				"social: other people are present. " +
				"unwind: relaxing alone while awake. " +
				"sleep: going to sleep. " +
				"other: the request names no occasion. " +
				"If two could apply, pick the one they actually said. " +
				"Never infer the occasion from the music itself — that is what \"other\" is for.",
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
- The Apple Music block, when present, holds search results. Its owned column
  says whether they already have the track. **Prefer owned=yes** — those play
  at once, and the library listing above may be spelling the same track in a
  different language, so a track can look missing when it is not.
  Picking owned=no adds it to their library first. That is allowed, and it is
  often the point: they asked for something they do not already have. But a
  sitting made entirely of strangers is a different product. The library is
  what they chose; reach outside it to answer the request, not to replace it.
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
// react 는 우리가 본 반응이다(data.Reactions). 비어 있어도 된다 — 아직
// 아무것도 안 들었거나 기록이 없으면 그 칸이 안 나올 뿐이다.
func Build(ctx context.Context, prompt string, library []api.Track, extras []Extra, react map[int64]data.Reaction, cur Current, now time.Time) (Result, error) {
	if strings.TrimSpace(prompt) == "" {
		return Result{}, fmt.Errorf("nothing to ask for")
	}

	start := time.Now()
	client := openai.NewClient(option.WithAPIKey(secrets.OpenAIKey()))
	lst := renderLibrary(library, react, now).withCatalog(extras)

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
		if n < 1 || int(n) > len(lst.rows) {
			out.Picks[i].TrackID = 0
			continue
		}
		c := lst.rows[n-1]
		out.Picks[i].TrackID, out.Picks[i].CatalogID = c.trackID, c.catalogID
	}
	out.Usage = api.Usage{
		PromptTokens:     int(resp.Usage.PromptTokens),
		CompletionTokens: int(resp.Usage.CompletionTokens),
		CostUsd:          cost(resp.Usage),
	}
	out.Elapsed = time.Since(start)
	out.Candidates = len(lst.rows)
	return out, nil
}

// 시스템 프롬프트와 라이브러리는 요청마다 같으므로 앞에 둔다. 그래야 캐시가 걸린다.
// 현재 큐는 매번 달라지므로 뒤에 붙인다.
func messages(prompt string, lst listing, cur Current) []openai.ChatCompletionMessageParamUnion {
	out := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.SystemMessage(lst.text),
	}
	// 카탈로그 후보는 라이브러리 **뒤**에 온다. 앞에 두면 캐시가 깨진다.
	if lst.catalog != "" {
		out = append(out, openai.SystemMessage(lst.catalog))
	}
	if len(cur.Items) > 0 {
		ord := make(map[int64]int, len(lst.rows))
		for i, c := range lst.rows {
			if c.trackID != 0 {
				ord[c.trackID] = i + 1
			}
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
	// text 는 라이브러리 표다. 요청마다 같으므로 프롬프트 캐시가 걸린다.
	text string

	// catalog 는 이번 요청에만 붙는 라이브러리 밖 후보다. 없으면 빈 문자열.
	//
	// 표를 따로 두는 이유가 둘이다. 하나는 캐시 — 라이브러리 표 안에 섞으면
	// 후보가 바뀔 때마다 166줄이 통째로 새 입력이 된다. 다른 하나는 뜻 —
	// "내 것"과 "아직 내 것이 아닌 것"은 칸 하나로 붙일 값이 아니라
	// 처지가 다른 두 무리다.
	catalog string

	rows []candidate // 줄번호-1 → 그 줄이 가리키는 곡
}

// candidate 는 줄번호 하나가 가리키는 곡이다. 둘 중 하나만 찬다.
type candidate struct {
	trackID   int64
	catalogID string
}

func renderLibrary(tracks []api.Track, react map[int64]data.Reaction, now time.Time) listing {
	var b strings.Builder
	rows := make([]candidate, 0, len(tracks))
	b.WriteString("The person's library. Columns: n | title | artist | album | genre | year | length | plays | signals | last played | added\n")
	b.WriteString("Use the n column as trackId when you pick a track.\n")
	b.WriteString("\"added: not in library\" means the track sits in a playlist but was never added to the library, so no add date exists. Never claim a date for those.\n")
	b.WriteString("signals is what we have observed. Their player's own counters: skip count, and whether they marked it a favorite. " +
		"Then what happened when this app played it: \"dropped early\" means they left before half the track, \"finished\" means it ran to the end, " +
		"\"removed\" means they picked it out of the queue. \"-\" means we have seen nothing.\n")
	b.WriteString("Those observations are evidence, not a verdict. One early exit is noise; a track dropped early several times and never finished is a real dislike. " +
		"Nothing here means never play it again — people skip tracks they love because they do not fit the moment. " +
		"Weigh it against what they asked for, and say so in your reason when it changed your mind.\n\n")
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
		rows = append(rows, candidate{trackID: t.Id})
		fmt.Fprintf(&b, "%d | %s | %s | %s | %s | %s | %s | %d plays | %s | %s | %s\n",
			len(rows), t.Title, t.Artist.Name, album, genre, year,
			mmss(t.DurationMs), t.PlayCount, signals(t, react[t.Id]), last, added)
	}
	return listing{text: b.String(), rows: rows}
}

// Extra 는 애플 뮤직에서 찾은 후보 하나다.
//
// TrackID 가 붙어 있으면 **이미 가진 곡**이다. 그것을 고르면 담을 것도
// 기다릴 것도 없이 바로 튼다.
//
// 찾은 것 중 이미 가진 곡을 버리지 않는 이유는, 라이브러리 표가 그 곡을
// 다른 이름으로 적고 있을 수 있기 때문이다. Music.app 은 시스템 언어에
// 맞춰 이름을 현지화한다 — "야생화" 를 찾는 사람에게 라이브러리 표는
// "Wild Flower" 라고 적혀 있다. 버리면 가진 곡을 못 찾고, 태그 없이 그냥
// 두면 가진 곡을 밖에서 다시 사 온다. 그래서 **표시해서 준다.**
type Extra struct {
	Track api.CatalogTrack

	// TrackID 는 이 곡이 이미 라이브러리에 있을 때 그 곡의 번호다.
	// 0 이면 밖의 곡이라 고르면 담아야 한다.
	TrackID int64
}

// withCatalog 는 애플 뮤직에서 찾은 후보를 목록 뒤에 잇는다.
//
// 줄번호는 라이브러리에서 이어진다. 모델이 보는 것은 번호 하나뿐이고,
// 그 번호가 어느 세계를 가리키는지는 우리가 표로 들고 있다 — 이 층이
// 19자리 id 를 감추는 것과 같은 이치다.
func (l listing) withCatalog(extras []Extra) listing {
	if len(extras) == 0 {
		return l
	}
	var b strings.Builder
	b.WriteString("Apple Music search results. " +
		"Columns: n | title | artist | album | genre | year | owned\n")
	b.WriteString("owned=yes means it is already in their library under a different name — " +
		"the library listing above may spell it in another language. " +
		"Picking one of those plays it immediately; prefer them.\n")
	b.WriteString("owned=no means picking it adds it to their library first, which takes a while. " +
		"There are no play counts or signals for those because they have never had them.\n")
	// 길이를 못 적는다. api.CatalogTrack 에 그 칸이 없다(스펙에는 있고
	// 생성된 타입이 낡았다). 없는 값을 지어내는 대신 모른다고 적는다 —
	// 모르는 것을 아는 척하면 "한 시간짜리"가 조용히 틀린다.
	b.WriteString("Their length is unknown, so do not count them toward a running time the person asked for.\n\n")
	for _, e := range extras {
		ct := e.Track
		album, genre, year := "", "", ""
		if ct.AlbumName != nil {
			album = *ct.AlbumName
		}
		if ct.Genre != nil {
			genre = *ct.Genre
		}
		if ct.Year != nil {
			year = fmt.Sprint(*ct.Year)
		}
		owned := "no"
		c := candidate{catalogID: ct.AppleMusicId}
		if e.TrackID != 0 {
			owned, c = "yes", candidate{trackID: e.TrackID}
		}
		l.rows = append(l.rows, c)
		fmt.Fprintf(&b, "%d | %s | %s | %s | %s | %s | %s\n",
			len(l.rows), ct.Title, ct.ArtistName, album, genre, year, owned)
	}
	l.catalog = b.String()
	return l
}

// signals 는 플레이어가 기록해 둔 것을 한 칸에 적는다.
//
// 값이 없으면 "-" 다. 빈 칸으로 두면 칸이 밀려 다음 값이 이 자리로 읽힌다.
func signals(t api.Track, r data.Reaction) string {
	var out []string
	if t.SkipCount > 0 {
		out = append(out, fmt.Sprintf("%d skips", t.SkipCount))
	}
	if t.Favorited {
		out = append(out, "favorite")
	}
	// 우리가 본 것. Music.app 의 카운터는 **왜** 끝났는지를 모른다 —
	// 끝까지 들은 것과 3초 만에 나간 것이 같은 한 번으로 들어간다.
	// 그 차이가 취향이고, 그것만 여기 적는다(docs 없음, data/plays.go).
	if r.Early > 0 {
		out = append(out, fmt.Sprintf("%d dropped early", r.Early))
	}
	if r.Done > 0 {
		out = append(out, fmt.Sprintf("%d finished", r.Done))
	}
	if r.Removed > 0 {
		out = append(out, fmt.Sprintf("%d removed", r.Removed))
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
