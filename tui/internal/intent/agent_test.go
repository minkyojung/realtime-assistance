package intent

import (
	"strings"
	"testing"

	"amcli/tui/internal/api"
	"github.com/openai/openai-go/v3/shared"
)

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

// 모델이 보는 큐. 1부터 세고, 지금 나오는 곡에 표가 있어야 한다.
func TestRenderedQueueIsNumberedFromOne(t *testing.T) {
	cur := queueOf(11, 22, 33)
	cur.Playing = 22

	got := renderQueue(cur)
	for _, want := range []string{"  1 | track 11", "▶ 2 | track 22", "  3 | track 33"} {
		if !strings.Contains(got, want) {
			t.Errorf("큐에 %q 가 없다:\n%s", want, got)
		}
	}
}

func queueOf(ids ...int64) Current {
	items := make([]api.QueueItem, 0, len(ids))
	for _, id := range ids {
		items = append(items, api.QueueItem{Track: api.Track{
			Id:     id,
			Title:  "track " + itoa(id),
			Artist: api.Artist{Name: "someone"},
		}})
	}
	return Current{Items: items}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// 도구를 쥐여주는 호출은 추론을 꺼야 한다.
//
// 실기로 만난 400 이다:
//
//	Function tools with reasoning_effort are not supported for gpt-5.4-mini
//	in /v1/chat/completions.
//
// 다른 층은 low 를 쓰므로 여기도 그래야 할 것처럼 보이고, 그래서 되돌리기
// 쉽다. 되돌리면 모든 요청이 400 으로 죽는다.
func TestToolCallsMustNotAskForReasoning(t *testing.T) {
	if agentEffort != shared.ReasoningEffortNone {
		t.Errorf("도구를 쓰는 층의 추론이 %q 다 — none 이어야 400 이 안 난다", agentEffort)
	}
}
