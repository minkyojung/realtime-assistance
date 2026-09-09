package musicapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"amcli/tui/internal/app"
	"amcli/tui/internal/data"
	"amcli/tui/internal/intent"
	tea "charm.land/bubbletea/v2"
)

// 이 앱이 모델에게 쥐여주는 것들.
//
// 셋이다. pi 의 하네스가 read·write·edit·bash 넷으로 버티는 것을 보고 정했다.
// 도구가 늘면 모델이 고를 것도 늘고, 무엇을 잘못 골랐는지 짚기도 어려워진다.
// 늘리는 것은 쉬우니 모자랄 때 늘린다.
//
// **아무것도 안 부르는 선택지가 넷째다.** "안녕"에는 도구가 필요 없다.

// 한 물음에 모델을 부르는 횟수의 상한.
//
// 도구를 부르고 그 결과를 보고 또 부르는 것을 무한히 두면, 한 번 헛돌기
// 시작한 대화가 돈을 쓰면서 멈추지 않는다. 셋이면 "물어보고 → 하고 → 말하고"가
// 들어간다. 그보다 긴 일은 이 제품에 아직 없다.
const maxSteps = 3

var (
	errNothingToSay = errors.New("the model had nothing to say")
	errTooManySteps = errors.New("gave up after going in circles")
)

// toolSpec 은 도구 하나와, 그 결과를 어떻게 다룰지다.
type toolSpec struct {
	intent.Tool

	// answers 는 이 도구의 결과가 그 자체로 사람에게 할 답인지다.
	//
	// 큐를 짜면 왜 그렇게 골랐는지를 선곡이 이미 한 문장으로 말해 준다.
	// 그 위에 한마디를 더 얹자고 모델을 또 부르면 1초가 그냥 든다.
	//
	// 반대로 라이브러리를 물어본 결과는 숫자 뭉치라 사람이 읽을 문장이 아니다.
	// 그때는 모델에게 돌려주어 말이 되게 해야 한다.
	answers bool
}

func (m Model) tools() []toolSpec {
	return []toolSpec{
		{answers: true, Tool: intent.Tool{
			Name: "build_queue",
			Description: "Pick tracks from the person's library and start playing them. " +
				"Use this for any request about what to listen to, including adjusting the " +
				"current queue into a different one (quieter, shorter, another mood).",
			Params: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"request"},
				"properties": map[string]any{
					"request": map[string]any{
						"type": "string",
						"description": "What to play, in the person's own words. " +
							"Pass their sentence through; do not translate or summarise it.",
					},
				},
			},
		}},
		{answers: true, Tool: intent.Tool{
			Name: "remove_tracks",
			Description: "Drop tracks from the queue that is already playing. " +
				"Only for tracks already in the queue — to play something else, use build_queue.",
			Params: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"positions"},
				"properties": map[string]any{
					"positions": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "integer"},
						"description": "Queue positions to drop, from the numbered queue you were shown.",
					},
				},
			},
		}},
		{Tool: intent.Tool{
			Name: "library_facts",
			Description: "Numbers about the person's library — how many tracks, how many " +
				"they have never played, what they play most. Use this to answer questions " +
				"about their listening. Returns numbers, not a sentence.",
		}},
	}
}

// specs 는 모델에게 넘길 형태로 바꾼다.
func (m Model) toolDefs() []intent.Tool {
	ts := m.tools()
	out := make([]intent.Tool, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.Tool)
	}
	return out
}

func (m Model) specFor(name string) (toolSpec, bool) {
	for _, t := range m.tools() {
		if t.Name == name {
			return t, true
		}
	}
	return toolSpec{}, false
}

// runTool 은 도구 하나를 돌린다.
//
// 돌려주는 것이 셋이다:
//
//	cmd != nil  느린 도구다. 답이 자기 메시지로 따로 온다(queueMsg). 턴은 산다
//	result      즉시 끝났다. 대화에 붙이거나, 그대로 사람에게 말한다
//	그리고 실패도 result 다 — 예외로 터뜨리면 사람은 침묵을 본다.
func (m Model) runTool(c intent.Call) (Model, string, tea.Cmd) {
	switch c.Name {
	case "build_queue":
		var a struct {
			Request string `json:"request"`
		}
		if err := json.Unmarshal([]byte(c.Args), &a); err != nil || strings.TrimSpace(a.Request) == "" {
			return m, "could not read the request", nil
		}
		// 선곡은 라이브러리 전체를 읽는다. 여기서만 그 값을 치른다.
		ctx := m.ask.ctx
		return m, "", cmdBuildQueue(ctx, m.ask.seq, a.Request, data.Lib().Tracks, m.current())

	case "remove_tracks":
		var a struct {
			Positions []int `json:"positions"`
		}
		if err := json.Unmarshal([]byte(c.Args), &a); err != nil {
			return m, "could not read the positions", nil
		}
		ids := m.idsAtPositions(a.Positions)
		if len(ids) == 0 {
			return m, "none of those positions are in the queue", nil
		}
		titles := make([]string, 0, len(ids))
		for _, id := range ids {
			for _, it := range m.queue {
				if it.Track.Id == id {
					titles = append(titles, it.Track.Title)
				}
			}
		}
		next, cmd := m.removeTracks(ids, "")
		return next.(Model), "removed " + strings.Join(titles, ", "), cmd

	case "library_facts":
		return m, libraryFacts(), nil
	}
	return m, "no such tool: " + c.Name, nil
}

// idsAtPositions 는 큐 자리번호를 곡 id 로 되돌린다.
//
// 자리번호로 주고받는 이유는 선곡과 같다 — persistent ID 는 19자리라 모델이
// 옮겨 적다 틀린다. 범위 밖과 중복은 여기서 버린다. 스키마가 막지 못하는
// 유일한 구멍이고, 여기서 틀리면 엉뚱한 곡이 사라진다.
func (m Model) idsAtPositions(positions []int) []int64 {
	seen := make(map[int]bool, len(positions))
	out := make([]int64, 0, len(positions))
	for _, n := range positions {
		if n < 1 || n > len(m.queue) || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, m.queue[n-1].Track.Id)
	}
	return out
}

// libraryFacts 는 라이브러리를 숫자로 적는다.
//
// 문장으로 만들지 않는다. 그것은 모델의 몫이고, 여기서 문장을 지으면 같은
// 말을 두 곳에서 만들게 된다. 사람의 말로 옮기는 일은 한 곳에서만 한다.
func libraryFacts() string {
	songs := data.Lib().Songs()
	if len(songs) == 0 {
		return "the library has not been read yet"
	}

	never, recent := 0, 0
	year := time.Now().AddDate(-1, 0, 0)
	top := make([]int, 0, len(songs))
	for i, t := range songs {
		if t.PlayCount == 0 && t.LastPlayedAt == nil {
			never++
		}
		if t.LastPlayedAt != nil && t.LastPlayedAt.After(year) {
			recent++
		}
		top = append(top, i)
	}
	sort.Slice(top, func(a, b int) bool {
		return songs[top[a]].PlayCount > songs[top[b]].PlayCount
	})

	var b strings.Builder
	fmt.Fprintf(&b, "tracks: %d\n", len(songs))
	fmt.Fprintf(&b, "never played: %d (%d%%)\n", never, never*100/len(songs))
	fmt.Fprintf(&b, "played in the last year: %d\n", recent)
	b.WriteString("most played:\n")
	for i, n := range top {
		if i == 3 {
			break
		}
		t := songs[n]
		fmt.Fprintf(&b, "  %s — %s (%d plays)\n", t.Title, t.Artist.Name, t.PlayCount)
	}
	return b.String()
}

// applyStep 은 모델이 한 걸음을 딛은 결과를 받는다.
//
// 세 갈래다.
//
//	도구를 안 불렀다  → 그냥 한 말이다. 턴을 닫는다 ("안녕" 이 여기로 온다)
//	answers 인 도구   → 결과가 곧 답이다. 돌리고 턴을 닫는다
//	그 밖의 도구      → 결과를 대화에 붙이고 한 걸음 더 묻는다
func (m Model) applyStep(msg stepMsg) (app.App, tea.Cmd) {
	m.usage.PromptTokens += msg.step.Usage.PromptTokens
	m.usage.CompletionTokens += msg.step.Usage.CompletionTokens
	m.usage.CostUsd += msg.step.Usage.CostUsd

	if msg.err != nil {
		// 그만둔 것은 실패가 아니다. 호스트가 이미 로그에 남겼다.
		if errors.Is(msg.err, context.Canceled) {
			return m, nil
		}
		m.ask = m.ask.done()
		return m, app.SayErr(m.Name(), msg.err)
	}
	m.chat = msg.chat
	m.steps++

	// 도구를 안 불렀으면 할 말을 한 것이다.
	if len(msg.step.Calls) == 0 {
		m.ask = m.ask.done()
		if msg.step.Text == "" {
			return m, app.SayErr(m.Name(), errNothingToSay)
		}
		return m, app.Say(m.Name(), msg.step.Text)
	}

	// 한 걸음에 하나만 돌린다. 여럿을 한꺼번에 돌리면 서로의 결과를 못 보고
	// 움직여, 큐를 짜면서 동시에 빼는 식의 앞뒤가 안 맞는 일이 생긴다.
	call := msg.step.Calls[0]
	spec, known := m.specFor(call.Name)
	next, result, cmd := m.runTool(call)
	m = next

	// 느린 도구다. 답이 자기 메시지로 따로 오므로 턴은 살려 둔다.
	if cmd != nil {
		return m, cmd
	}
	if known && spec.answers {
		m.ask = m.ask.done()
		return m, app.Say(m.Name(), result)
	}

	// 결과를 보고 한 걸음 더. 끝없이 돌지 않도록 상한을 둔다.
	m.chat = m.chat.WithResult(call.ID, result)
	if m.steps >= maxSteps {
		m.ask = m.ask.done()
		return m, app.SayErr(m.Name(), errTooManySteps)
	}
	var ctx context.Context
	m.ask, ctx = m.ask.extend(askTimeout)
	return m, cmdStep(ctx, m.ask.seq, m.chat, m.toolDefs())
}
