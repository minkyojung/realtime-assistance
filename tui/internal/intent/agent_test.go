package intent

import "testing"

// Chat 은 값이다. 복사가 곧 스냅샷이라 걸음마다 들고 다닐 수 있고, 도중에
// 그만두면 그냥 버리면 된다.
//
// 그 성질이 깨지면 조용히 틀어진다 — 버린 걸음이 남은 대화에 자기 결과를
// 밀어 넣고, 모델은 하지도 않은 일을 했다고 읽는다.
func TestChatCopiesDoNotShareHistory(t *testing.T) {
	base := NewChat("조용한 거", Current{}, nil)
	n := len(base.msgs)

	a := base.WithResult("call-1", "queued 8 tracks")
	b := base.WithResult("call-2", "nothing to remove")

	if len(base.msgs) != n {
		t.Errorf("원본 대화가 %d 줄에서 %d 줄로 늘었다", n, len(base.msgs))
	}
	if len(a.msgs) != n+1 || len(b.msgs) != n+1 {
		t.Fatalf("가지가 각각 한 줄씩 늘어야 하는데 %d, %d 다", len(a.msgs), len(b.msgs))
	}
	// 두 가지가 같은 바닥 배열을 나눠 쓰면 뒤에 붙인 쪽이 앞의 것을 덮는다.
	if a.msgs[n].OfTool.ToolCallID != "call-1" {
		t.Error("한쪽 가지의 결과가 다른 쪽에 덮였다")
	}
	if b.msgs[n].OfTool.ToolCallID != "call-2" {
		t.Error("한쪽 가지의 결과가 다른 쪽에 덮였다")
	}
}

// 큐가 없으면 큐 얘기를 넣지 않는다. 빈 목록을 보여주면 모델이 그것을
// "지금 큐가 비어 있다"는 사실로 읽고 엉뚱한 말을 지어낸다.
func TestChatMentionsTheQueueOnlyWhenThereIsOne(t *testing.T) {
	empty := NewChat("조용한 거", Current{}, nil)
	full := NewChat("조용한 거", queueOf(11, 22), nil)

	if len(full.msgs) != len(empty.msgs)+1 {
		t.Errorf("큐가 있을 때 한 줄이 더 붙어야 한다: %d 대 %d", len(full.msgs), len(empty.msgs))
	}
}
