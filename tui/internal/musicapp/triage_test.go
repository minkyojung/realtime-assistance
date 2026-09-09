package musicapp

import (
	"errors"
	"testing"

	"amcli/tui/internal/app"
	"amcli/tui/internal/data"
	"amcli/tui/internal/intent"
)

// 사람의 /remove 와 AI 의 "빼줘"는 같은 문으로 들어가야 한다.
// 그래야 고치는 길이 하나가 되고, 한 곳만 맞으면 둘 다 맞는다.

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

func queueIDs(m Model) []int64 {
	out := make([]int64, 0, len(m.queue))
	for _, it := range m.queue {
		out = append(out, it.Track.Id)
	}
	return out
}

// AI 가 지목한 곡이 빠져야 한다.
func TestTriageRemovesWhatTheModelPointedAt(t *testing.T) {
	m := withQueue(t)
	want := queueIDs(m)[2] // 3번 곡만 남기고 확인할 대상

	next, _ := m.Update(triagedMsg{seq: m.askSeq, edit: intent.Edit{
		Kind: intent.EditRemove, TrackIDs: []int64{want}, Note: "뺐습니다",
	}})

	after := next.(Model)
	if len(after.queue) != 3 {
		t.Fatalf("한 곡을 뺐는데 %d곡 남았다", len(after.queue))
	}
	for _, id := range queueIDs(after) {
		if id == want {
			t.Error("빼라고 한 곡이 남아 있다")
		}
	}
}

// **여러 곡은 뒤에서부터 뺀다.**
//
// 앞에서 빼면 그 뒤 곡들의 자리가 밀려, 두 번째 곡을 지울 때 엉뚱한 줄을
// 가리킨다. 이 테스트가 그 한 줄을 지킨다.
func TestRemovingSeveralKeepsTheRightOnes(t *testing.T) {
	m := withQueue(t)
	ids := queueIDs(m)

	// 1번과 3번을 뺀다. 2번과 4번이 남아야 한다.
	next, _ := m.Update(triagedMsg{seq: m.askSeq, edit: intent.Edit{
		Kind: intent.EditRemove, TrackIDs: []int64{ids[0], ids[2]},
	}})

	got := queueIDs(next.(Model))
	if len(got) != 2 || got[0] != ids[1] || got[1] != ids[3] {
		t.Errorf("남은 곡이 %v 다 — 원한 것 %v", got, []int64{ids[1], ids[3]})
	}
}

// 판단이 실패하면 새로 짜는 쪽으로 간다. 느릴 뿐 틀리지는 않는다.
func TestTriageFailureFallsBackToBuilding(t *testing.T) {
	m := withQueue(t)
	before := len(m.queue)

	// 진짜 경로를 밟는다 — 물어보면 기다림이 켜지고 판단이 먼저 나간다.
	asked, cmd := m.Update(app.AskMsg{Prompt: "조용한 거"})
	if cmd == nil {
		t.Fatal("물었는데 아무 일도 안 일어났다")
	}
	m = asked.(Model)
	if !m.thinking {
		t.Fatal("물었는데 기다린다는 표시가 없다")
	}

	next, cmd := m.Update(triagedMsg{
		seq: m.askSeq, prompt: "조용한 거", err: errors.New("model unavailable"),
	})
	if cmd == nil {
		t.Fatal("판단이 실패했는데 아무 일도 안 한다")
	}
	after := next.(Model)
	if !after.thinking {
		t.Error("아직 답을 기다리는 중인데 기다림을 껐다")
	}
	if len(after.queue) != before {
		t.Error("판단이 실패했는데 큐를 건드렸다")
	}
}

// 그만둔 뒤 늦게 온 판단은 큐를 건드리면 안 된다.
func TestStaleTriageIsIgnored(t *testing.T) {
	m := withQueue(t)
	ids := queueIDs(m)

	next, _ := m.Update(triagedMsg{seq: m.askSeq - 1, edit: intent.Edit{
		Kind: intent.EditRemove, TrackIDs: []int64{ids[0]},
	}})

	if len(next.(Model).queue) != len(ids) {
		t.Error("그만둔 요청의 판단이 큐를 건드렸다")
	}
}

// 새로 짜라는 판단이면 큐를 건드리지 않고 선곡으로 넘어간다.
func TestTriageNewLeavesTheQueueAlone(t *testing.T) {
	m := withQueue(t)
	before := queueIDs(m)

	next, cmd := m.Update(triagedMsg{seq: m.askSeq, prompt: "신나는 걸로", edit: intent.Edit{Kind: intent.EditNew}})
	if cmd == nil {
		t.Fatal("새로 짜라는데 선곡을 시작하지 않았다")
	}
	if got := queueIDs(next.(Model)); len(got) != len(before) {
		t.Errorf("선곡 전에 큐를 건드렸다: %v", got)
	}
}
