package music

import (
	"context"
	"errors"
	"testing"
	"time"
)

// 통로는 하나다. 이 규칙이 무너지면 요청이 Music.app 앞에 쌓이고,
// 쌓이면 Music.app 이 굳는다 — 실측으로 40개 동시에 11배 느려졌다.

// 폴링은 통로가 차 있으면 기다리지 않고 버린다.
func TestPollIsDroppedWhileLaneIsHeld(t *testing.T) {
	release, ok := tryHold()
	if !ok {
		t.Fatal("빈 통로를 못 잡았다")
	}
	defer release()

	if _, ok := tryHold(); ok {
		t.Fatal("차 있는 통로를 또 잡았다 — 두 요청이 동시에 나간다")
	}
	// Status 는 Music.app 을 보기도 전에 물러나야 한다. pgrep 도 안 띄운다.
	start := time.Now()
	_, err := Status()
	if !errors.Is(err, ErrBusy) {
		t.Errorf("통로가 찼는데 %v — ErrBusy 여야 한다", err)
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Error("버려야 할 폴링이 기다렸다")
	}
}

// 명령은 통로가 빌 때까지 기다렸다가 간다. 버리지 않는다.
func TestCommandWaitsForTheLane(t *testing.T) {
	release, _ := tryHold()

	got := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		r, err := hold(ctx)
		if err == nil {
			r()
		}
		got <- err
	}()

	select {
	case <-got:
		t.Fatal("통로가 찼는데 명령이 기다리지 않고 지나갔다")
	case <-time.After(100 * time.Millisecond):
	}
	release()
	select {
	case err := <-got:
		if err != nil {
			t.Errorf("통로가 비었는데 못 들어갔다: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("통로가 비었는데 명령이 안 깨어났다")
	}
}

// 기다림에도 끝이 있다. 앞 요청이 영영 안 끝나면(권한 대화상자) 뒤도
// 영영 기다리면 안 된다 — 그 상한이 명령 자신의 timeout 이다.
func TestWaitingGivesUpWithTheContext(t *testing.T) {
	release, _ := tryHold()
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := hold(ctx); err == nil {
		t.Fatal("영영 안 비는 통로를 기다리다 돌아오지 않았다")
	}
}

// 놓으면 다음이 잡는다. 놓는 것을 잊은 경로가 하나라도 있으면 그 뒤로
// 모든 요청이 멈추므로, 잡는 쪽은 반드시 defer 로 놓는다.
func TestReleaseFreesTheLane(t *testing.T) {
	release, _ := tryHold()
	release()
	again, ok := tryHold()
	if !ok {
		t.Fatal("놓았는데 아직 차 있다")
	}
	again() // 이 테스트가 통로를 쥔 채 끝나면 뒤 테스트가 전부 ErrBusy 를 본다
}
