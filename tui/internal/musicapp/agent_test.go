package musicapp

import (
	"errors"
	"strings"
	"testing"

	"amcli/tui/internal/app"
	"amcli/tui/internal/data"
	"amcli/tui/internal/intent"
)

// 사람의 /remove 와 AI 의 "빼줘"는 같은 문으로 들어가야 한다. 그래야 고치는
// 길이 하나가 되고, 한 곳만 맞으면 둘 다 맞는다.

// withQueue 는 곡 넷이 담긴 큐를 가진 모델이다.
func withQueue(t *testing.T) Model {
	t.Helper()
	ts := data.Lib().Songs()
	if len(ts) < 4 {
		t.Fatal("픽스처에 곡이 모자라다")
	}
	m := New()
	m.bodyH = 20
	next, _ := m.applyQueue(intent.Result{Title: "set", Picks: []intent.Pick{
		{TrackID: ts[0].Id}, {TrackID: ts[1].Id}, {TrackID: ts[2].Id}, {TrackID: ts[3].Id},
	}})
	mm := next.(Model)
	if len(mm.queue) != 4 {
		t.Fatalf("큐에 4곡이 있어야 하는데 %d곡이다", len(mm.queue))
	}
	return mm
}

// asked 는 문장을 하나 던져 턴을 연 모델이다.
//
// 걸음 결과를 직접 흘려보내려면 턴이 열려 있어야 한다. 기다리지 않는데
// 도착한 답은 언제나 늦은 답으로 버려지기 때문이다(turn.go).
func asked(t *testing.T, m Model, prompt string) Model {
	t.Helper()
	next, cmd := m.Update(app.AskMsg{Prompt: prompt})
	if cmd == nil {
		t.Fatal("물었는데 아무 일도 안 일어났다")
	}
	mm := next.(Model)
	if !mm.ask.live {
		t.Fatal("물었는데 기다린다는 표시가 없다")
	}
	return mm
}

func queueIDs(m Model) []int64 {
	out := make([]int64, 0, len(m.queue))
	for _, it := range m.queue {
		out = append(out, it.Track.Id)
	}
	return out
}

// step 은 모델이 한 걸음 딛었다는 소식을 만든다.
func step(m Model, calls ...intent.Call) stepMsg {
	return stepMsg{seq: m.ask.seq, chat: m.chat, step: intent.Step{Calls: calls}}
}

func said(m Model, text string) stepMsg {
	return stepMsg{seq: m.ask.seq, chat: m.chat, step: intent.Step{Text: text}}
}

// 도구를 안 부르면 그냥 한 말이다. 이 길이 없어서 "안녕"에 곡을 지어냈다.
func TestPlainAnswerNeedsNoTool(t *testing.T) {
	m := asked(t, New(), "안녕")

	next, cmd := m.Update(said(m, "안녕하세요. 뭘 틀어드릴까요?"))
	if cmd == nil {
		t.Fatal("한 말을 로그에 안 남겼다")
	}
	after := next.(Model)
	if after.ask.live {
		t.Error("할 말을 다 했는데 아직 기다린다고 한다")
	}
	if len(after.queue) != 0 {
		t.Error("잡담에 큐를 만들었다")
	}
}

// 결과가 그 자체로 답인 도구는 돌리고 턴을 닫는다.
// 한마디를 더 얹자고 모델을 또 부르면 1초가 그냥 든다.
func TestAnsweringToolClosesTheTurn(t *testing.T) {
	m := asked(t, withQueue(t), "3번 빼줘")
	want := queueIDs(m)[2]

	next, cmd := m.Update(step(m, intent.Call{
		ID: "c1", Name: "remove_tracks", Args: `{"positions":[3]}`,
	}))
	if cmd == nil {
		t.Fatal("뺐다는 말을 안 했다")
	}
	after := next.(Model)
	if after.ask.live {
		t.Error("답까지 끝났는데 아직 기다린다고 한다")
	}
	for _, id := range queueIDs(after) {
		if id == want {
			t.Fatal("빼라고 한 곡이 남아 있다")
		}
	}
}

// **여러 곡은 뒤에서부터 뺀다.** 앞에서 빼면 그 뒤 곡들의 자리가 밀려,
// 두 번째 곡을 지울 때 엉뚱한 줄을 가리킨다.
func TestRemovingSeveralKeepsTheRightOnes(t *testing.T) {
	m := asked(t, withQueue(t), "1번이랑 3번 빼줘")
	ids := queueIDs(m)

	next, _ := m.Update(step(m, intent.Call{
		ID: "c1", Name: "remove_tracks", Args: `{"positions":[1,3]}`,
	}))

	got := queueIDs(next.(Model))
	if len(got) != 2 || got[0] != ids[1] || got[1] != ids[3] {
		t.Errorf("남은 곡이 %v 다 — 원한 것 %v", got, []int64{ids[1], ids[3]})
	}
}

// 숫자 뭉치는 사람이 읽을 답이 아니다. 모델에게 돌려주어 말이 되게 한다.
func TestFactsGoBackToTheModel(t *testing.T) {
	m := asked(t, New(), "내가 제일 많이 들은 곡이 뭐야?")

	next, cmd := m.Update(step(m, intent.Call{ID: "c1", Name: "library_facts"}))
	if cmd == nil {
		t.Fatal("한 걸음 더 묻지 않았다")
	}
	after := next.(Model)
	if !after.ask.live {
		t.Error("아직 답을 만드는 중인데 턴을 닫았다")
	}
	if after.steps != 1 {
		t.Errorf("걸음을 %d 로 셌다", after.steps)
	}
}

// 끝없이 돌지 않는다. 한 번 헛돌기 시작한 대화가 돈을 쓰면서 안 멈추면 안 된다.
func TestLoopGivesUpAfterTooManySteps(t *testing.T) {
	m := asked(t, New(), "뭐 좀 알려줘")

	var cmd interface{}
	for i := 0; i < maxSteps+1; i++ {
		next, c := m.Update(step(m, intent.Call{ID: "c", Name: "library_facts"}))
		m, cmd = next.(Model), c
		if !m.ask.live {
			break
		}
	}
	if m.ask.live {
		t.Error("상한을 넘겼는데 계속 돈다")
	}
	if m.steps > maxSteps {
		t.Errorf("모델을 %d 번 불렀다 — 상한은 %d", m.steps, maxSteps)
	}
	_ = cmd
}

// 없는 도구를 부르면 실패도 결과로 돌려준다. 침묵을 보여주면 안 된다.
func TestUnknownToolComesBackAsAResult(t *testing.T) {
	m := asked(t, New(), "뭐든 해봐")

	next, cmd := m.Update(step(m, intent.Call{ID: "c1", Name: "launch_rocket"}))
	if cmd == nil {
		t.Fatal("모르는 도구에 아무 반응이 없다")
	}
	if !next.(Model).ask.live {
		t.Error("모르는 도구 하나에 턴이 끝나 버렸다")
	}
}

// 자리번호를 곡 id 로 되돌리는 자리. 여기서 틀리면 엉뚱한 곡이 사라진다.
func TestPositionsBecomeTrackIDs(t *testing.T) {
	m := withQueue(t)
	ids := queueIDs(m)

	for _, c := range []struct {
		name string
		in   []int
		want []int64
	}{
		{"자리번호 그대로", []int{1, 3}, []int64{ids[0], ids[2]}},
		{"순서를 지킨다", []int{3, 1}, []int64{ids[2], ids[0]}},
		{"범위 밖은 버린다", []int{0, 9, 2, -1}, []int64{ids[1]}},
		{"같은 자리를 두 번 짚으면 한 번만", []int{2, 2}, []int64{ids[1]}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := m.idsAtPositions(c.in)
			if len(got) != len(c.want) {
				t.Fatalf("%v → %v, 원한 것 %v", c.in, got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("%v → %v, 원한 것 %v", c.in, got, c.want)
				}
			}
		})
	}
}

// 걸음이 실패하면 왜 실패했는지 말하고 턴을 닫는다.
func TestFailedStepSaysSo(t *testing.T) {
	m := asked(t, New(), "조용한 거")

	next, cmd := m.Update(stepMsg{seq: m.ask.seq, err: errors.New("model unavailable")})
	if cmd == nil {
		t.Fatal("실패했는데 아무 말도 안 한다")
	}
	if next.(Model).ask.live {
		t.Error("실패했는데 아직 기다린다고 한다")
	}
}

// 그만둔 뒤 늦게 온 걸음은 아무것도 건드리면 안 된다.
func TestStaleStepIsIgnored(t *testing.T) {
	m := asked(t, withQueue(t), "빼줘")
	ids := queueIDs(m)

	next, _ := m.Update(stepMsg{seq: m.ask.seq - 1, step: intent.Step{
		Calls: []intent.Call{{ID: "c", Name: "remove_tracks", Args: `{"positions":[1]}`}},
	}})

	if len(queueIDs(next.(Model))) != len(ids) {
		t.Error("그만둔 요청의 걸음이 큐를 건드렸다")
	}
}

// 라이브러리 숫자는 모델이 읽을 것이므로 사실만 적는다.
func TestLibraryFactsAreNumbersNotProse(t *testing.T) {
	got := libraryFacts()
	for _, want := range []string{"tracks:", "never played:", "most played:"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q 가 없다:\n%s", want, got)
		}
	}
}

// 기억 — "아까 그거"가 무엇을 가리키는지 아는 데 필요한 최소한.

// 주고받은 말이 남아야 다음 턴이 그것을 가리킬 수 있다.
func TestFinishedTurnIsRemembered(t *testing.T) {
	m := asked(t, New(), "안녕")

	next, _ := m.Update(said(m, "안녕하세요"))
	after := next.(Model)

	if len(after.memory) != 1 {
		t.Fatalf("기억이 %d 개다 — 하나여야 한다", len(after.memory))
	}
	if after.memory[0].Ask != "안녕" || after.memory[0].Said != "안녕하세요" {
		t.Errorf("주고받은 말이 짝이 안 맞는다: %+v", after.memory[0])
	}
}

// **그만둔 턴은 기억에 안 남는다.**
//
// 안 한 일을 했다고 적어 두면 다음 턴에 모델이 그것을 사실로 읽는다.
// esc 로 물러난 요청은 일어나지 않은 일이다.
func TestCancelledTurnLeavesNoTrace(t *testing.T) {
	m := asked(t, New(), "조용한 거")

	next, _ := m.Update(app.CancelMsg{})
	if got := len(next.(Model).memory); got != 0 {
		t.Errorf("그만둔 턴이 기억에 %d 개 남았다", got)
	}
}

// 실패한 턴도 안 남는다. 하지 못한 일을 했다고 적을 이유가 없다.
func TestFailedTurnLeavesNoTrace(t *testing.T) {
	m := asked(t, New(), "조용한 거")

	next, _ := m.Update(stepMsg{seq: m.ask.seq, err: errors.New("model unavailable")})
	if got := len(next.(Model).memory); got != 0 {
		t.Errorf("실패한 턴이 기억에 %d 개 남았다", got)
	}
}

// 오래된 것부터 빠진다. 요약을 접는 장치를 두지 않는 대신 개수로 막는다.
func TestMemoryKeepsOnlyTheRecentTurns(t *testing.T) {
	m := New()
	for i := 0; i < maxMemory+3; i++ {
		m = asked(t, m, "물음 "+string(rune('a'+i)))
		next, _ := m.Update(said(m, "답 "+string(rune('a'+i))))
		m = next.(Model)
	}

	if len(m.memory) != maxMemory {
		t.Fatalf("기억이 %d 개다 — 상한은 %d", len(m.memory), maxMemory)
	}
	// 마지막에 한 말이 마지막에 남아 있어야 한다.
	if last := m.memory[len(m.memory)-1]; last.Ask != "물음 i" {
		t.Errorf("가장 최근 물음이 %q 다", last.Ask)
	}
	// 맨 처음 것은 빠졌어야 한다.
	for _, ex := range m.memory {
		if ex.Ask == "물음 a" {
			t.Error("상한을 넘겼는데 제일 오래된 것이 남아 있다")
		}
	}
}

// 기억이 실제로 다음 물음에 실려야 한다. 쌓아두고 안 쓰면 아무 소용이 없다.
func TestMemoryIsCarriedIntoTheNextQuestion(t *testing.T) {
	m := asked(t, New(), "조용한 거")
	next, _ := m.Update(said(m, "8곡 담았습니다"))
	m = next.(Model)

	bare := New()
	bare.bodyH = 20
	bare = asked(t, bare, "아까 그거 말고")
	with := asked(t, m, "아까 그거 말고")

	if with.chat.Len() <= bare.chat.Len() {
		t.Errorf("기억이 있는데 물음에 안 실렸다: %d 줄, 기억 없을 때 %d 줄",
			with.chat.Len(), bare.chat.Len())
	}
}
