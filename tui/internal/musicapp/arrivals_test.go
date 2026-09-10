package musicapp

import (
	"context"
	"errors"
	"testing"
)

// 기다리는 동안은 곡 수만 묻고, 전곡 열거는 수가 늘었을 때 한 번이다.
//
// Music.app 이 방금 담은 곡을 받아오느라 가장 바쁜 순간에 가장 무거운
// 질문을 열세 번 겹쳐 얹고 있었다. 그 더미가 Music.app 을 굳혔다.

// 짧게 기다리도록 상수를 줄인다. 판단 로직만 잰다.
func fastArrivals(t *testing.T) {
	t.Helper()
	old := addPlayInterval
	addPlayInterval = 0
	t.Cleanup(func() { addPlayInterval = old })
}

func TestEnumeratesOnlyOnceCountGrows(t *testing.T) {
	fastArrivals(t)
	before := map[string]bool{"a": true, "b": true}
	counts := []int{2, 2, 2, 3} // 세 바퀴는 그대로, 넷째에 늘어난다
	calls, enums := 0, 0
	count := func() (int, error) {
		n := counts[min(calls, len(counts)-1)]
		calls++
		return n, nil
	}
	ids := func() (map[string]bool, error) {
		enums++
		return map[string]bool{"a": true, "b": true, "new": true}, nil
	}

	got := waitForArrivals(context.Background(), before, 1, count, ids)

	if !got["new"] {
		t.Fatalf("나타난 곡을 못 챙겼다: %v", got)
	}
	if enums != 1 {
		t.Errorf("전곡 열거가 %d번 — 수가 늘었을 때 한 번이어야 한다", enums)
	}
	if calls != 4 {
		t.Errorf("수를 %d번 물었다 — 늘 때까지 네 번이어야 한다", calls)
	}
}

// 수가 끝내 안 늘면 열거도 안 한다. 빈 손이 정답이다.
func TestNeverEnumeratesWhenNothingArrives(t *testing.T) {
	fastArrivals(t)
	before := map[string]bool{"a": true}
	enums := 0
	count := func() (int, error) { return 1, nil }
	ids := func() (map[string]bool, error) { enums++; return before, nil }

	got := waitForArrivals(context.Background(), before, 1, count, ids)
	if len(got) != 0 {
		t.Errorf("아무것도 안 왔는데 %v 를 챙겼다", got)
	}
	if enums != 0 {
		t.Errorf("수가 안 늘었는데 %d번 열거했다", enums)
	}
}

// 둘을 시켰는데 하나만 왔으면 마지막에 그 하나는 챙긴다.
// 하나가 늦는다고 나머지까지 버릴 이유가 없다.
func TestKeepsWhatArrivedOnTheLastTry(t *testing.T) {
	fastArrivals(t)
	before := map[string]bool{"a": true}
	count := func() (int, error) { return 2, nil } // 셋이어야 하는데 둘뿐
	ids := func() (map[string]bool, error) { return map[string]bool{"a": true, "x": true}, nil }

	got := waitForArrivals(context.Background(), before, 2, count, ids)
	if !got["x"] {
		t.Errorf("온 만큼이라도 챙겨야 한다: %v", got)
	}
}

// 수를 못 물어도 죽지 않고 다음 바퀴로 간다.
func TestCountErrorsAreRetried(t *testing.T) {
	fastArrivals(t)
	before := map[string]bool{}
	calls := 0
	count := func() (int, error) {
		calls++
		if calls < 3 {
			return 0, errors.New("busy")
		}
		return 1, nil
	}
	ids := func() (map[string]bool, error) { return map[string]bool{"n": true}, nil }

	if got := waitForArrivals(context.Background(), before, 1, count, ids); !got["n"] {
		t.Errorf("오류 뒤에 회복하지 못했다: %v", got)
	}
}

// esc 면 즉시 손을 뗀다.
func TestCancelStopsWaitingForArrivals(t *testing.T) {
	fastArrivals(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	count := func() (int, error) { calls++; return 0, nil }
	waitForArrivals(ctx, map[string]bool{}, 1, count, nil)
	if calls != 0 {
		t.Errorf("그만뒀는데 %d번 더 물었다", calls)
	}
}
