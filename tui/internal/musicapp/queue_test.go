package musicapp

import (
	"errors"
	"testing"

	"amcli/tui/internal/data"
	"amcli/tui/internal/intent"
)

// 큐는 Music.app 안에 실제로 있어야 한다.
//
// queuePID 는 "화면의 큐와 Music.app 의 큐가 같다"는 표시다. 비어 있으면
// 번호를 믿을 수 없다는 뜻이고, 그 상태로 n번째 곡을 틀면 엉뚱한 곡이 나온다.

// twoPicks 는 픽스처에서 곡 두 개를 고른 결과다.
func twoPicks(t *testing.T) intent.Result {
	t.Helper()
	ts := data.Lib().Songs()
	if len(ts) < 2 {
		t.Fatal("픽스처에 곡이 모자라다")
	}
	return intent.Result{
		Title: "quiet set",
		Picks: []intent.Pick{
			{TrackID: ts[0].Id, Reason: "a"},
			{TrackID: ts[1].Id, Reason: "b"},
		},
	}
}

// 큐가 생기면 Music.app 에 쓰라는 명령이 나가야 한다. 그리고 그것이
// 끝나기 전까지 queuePID 는 비어 있어야 한다 — 아직 어긋난 상태다.
func TestNewQueueAsksMusicAppToBuildIt(t *testing.T) {
	m := New()
	m.bodyH = 20

	next, cmd := m.applyQueue(twoPicks(t))
	if cmd == nil {
		t.Fatal("큐가 생겼는데 Music.app 에 아무것도 시키지 않았다")
	}
	if got := next.(Model).queuePID; got != "" {
		t.Errorf("아직 안 썼는데 같다고 표시한다: %q", got)
	}
}

// 다 쓰고 나면 그때부터 번호를 믿는다.
func TestQueuePIDIsSetOnlyAfterItIsWritten(t *testing.T) {
	m := New()
	m.bodyH = 20
	next, _ := m.applyQueue(twoPicks(t))

	after, _ := next.(Model).Update(queueWrittenMsg{pid: "PL1234"})
	if got := after.(Model).queuePID; got != "PL1234" {
		t.Errorf("다 썼는데 %q 다", got)
	}
}

// 쓰다 실패하면 같다고 말하면 안 된다. 실패를 삼키면 그다음부터
// 엉뚱한 곡이 나오는데 이유를 알 길이 없다.
func TestFailedWriteLeavesTheQueueUnmatched(t *testing.T) {
	m := New()
	m.bodyH = 20

	next, cmd := m.Update(queueWrittenMsg{err: errors.New("music app said no")})
	if cmd == nil {
		t.Error("실패를 말하지 않았다")
	}
	if got := next.(Model).queuePID; got != "" {
		t.Errorf("실패했는데 같다고 표시한다: %q", got)
	}
}

// 화면에서만 큐를 건드리면 번호가 어긋난다.
//
// ensureQueued 는 Music.app 이 틀고 있는 곡을 큐 맨 앞에 끼워 넣는다.
// 플레이리스트는 그대로이므로 그 순간 n번째의 뜻이 달라진다.
func TestTouchingTheQueueOnScreenBreaksTheMatch(t *testing.T) {
	m := New()
	m.bodyH = 20
	m.queuePID = "PL1234"

	ts := data.Lib().Songs()
	m.ensureQueued(ts[3])

	if m.queuePID != "" {
		t.Error("화면에서만 끼워 넣었는데 아직 같다고 표시한다")
	}
}

// 큐를 비우면 Music.app 의 큐와는 아무 관계가 없어진다.
func TestClearingTheQueueBreaksTheMatch(t *testing.T) {
	m := New()
	m.bodyH = 20
	m.queuePID = "PL1234"

	next, _ := m.Update(clearQueueMsg{})
	if got := next.(Model).queuePID; got != "" {
		t.Errorf("큐를 비웠는데 %q 가 남아 있다", got)
	}
}
