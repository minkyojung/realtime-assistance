package musicapp

import (
	"testing"
	"time"
)

// 번호·손잡이·기다림은 언제나 함께 움직여야 한다. 흩어져 있을 때는 이 규칙을
// 열다섯 군데에서 각자 지켜야 했다. 이제 여기 한 곳에서 증명한다.

const testTimeout = time.Minute

// 턴을 열면 기다리는 중이고, 그 번호의 답만 받는다.
func TestTurnAcceptsOnlyItsOwnAnswer(t *testing.T) {
	tn, ctx := turn{}.start(testTimeout)
	defer tn.cancel()

	if !tn.live {
		t.Fatal("턴을 열었는데 기다리지 않는다")
	}
	if ctx == nil {
		t.Fatal("끊을 수 있는 ctx 를 안 줬다")
	}
	if !tn.fresh(tn.seq) {
		t.Error("자기 번호의 답을 안 받는다")
	}
	if tn.fresh(tn.seq - 1) {
		t.Error("앞 번호의 답을 받는다")
	}
	if tn.fresh(tn.seq + 1) {
		t.Error("있지도 않은 번호의 답을 받는다")
	}
}

// 그만두면 도는 요청이 실제로 끊겨야 한다. 번호만 올리고 손잡이를 안 부르면
// 요청은 배경에서 계속 돌며 토큰을 쓴다.
func TestStopActuallyCutsTheRequest(t *testing.T) {
	tn, ctx := turn{}.start(testTimeout)
	seq := tn.seq

	tn = tn.stop()

	if ctx.Err() == nil {
		t.Error("그만뒀는데 요청이 아직 살아 있다")
	}
	if tn.live {
		t.Error("그만뒀는데 아직 기다린다고 한다")
	}
	if tn.fresh(seq) {
		t.Error("그만둔 요청의 답을 아직 받는다")
	}
}

// 답이 도착해 끝난 턴도 더는 답을 받지 않는다.
// 번호는 올리지 않는다 — 올릴 이유가 없고, "그만뒀다"와 구별되어야 한다.
func TestDoneEndsTheTurnWithoutBumping(t *testing.T) {
	tn, _ := turn{}.start(testTimeout)
	seq := tn.seq

	tn = tn.done()

	if tn.live {
		t.Error("답이 왔는데 아직 기다린다고 한다")
	}
	if tn.fresh(seq) {
		t.Error("끝난 턴이 같은 답을 또 받는다")
	}
	if tn.seq != seq {
		t.Errorf("답이 왔을 뿐인데 번호가 %d 에서 %d 로 올랐다", seq, tn.seq)
	}
}

// 한 물음이 두 걸음으로 나뉘어도 번호는 하나다.
//
// 판단이 끝나고 선곡으로 넘어가는 길이다. 번호가 갈리면 도중에 그만뒀을 때
// 뒷걸음만 버려지고 앞걸음의 답이 화면에 앉는다.
func TestExtendKeepsOneNumberForOneQuestion(t *testing.T) {
	tn, first := turn{}.start(testTimeout)
	seq := tn.seq

	tn, second := tn.extend(testTimeout)

	if tn.seq != seq {
		t.Errorf("한 물음인데 번호가 %d 에서 %d 로 갈렸다", seq, tn.seq)
	}
	if !tn.fresh(seq) {
		t.Error("이어간 걸음이 자기 답을 못 받는다")
	}
	if first.Err() == nil {
		t.Error("앞걸음의 손잡이를 안 놓았다 — 걸음마다 쌓인다")
	}
	if second.Err() != nil {
		t.Error("이어갈 걸음이 시작부터 끊겨 있다")
	}
	// 이어간 턴을 그만두면 두 걸음이 함께 버려져야 한다.
	if tn = tn.stop(); tn.fresh(seq) {
		t.Error("그만뒀는데 앞걸음의 답을 아직 받는다")
	}
}

// 새 물음은 앞의 것을 확실히 버리고 시작한다. 한 번에 하나만 기다린다.
func TestStartDiscardsThePreviousTurn(t *testing.T) {
	old, oldCtx := turn{}.start(testTimeout)
	oldSeq := old.seq

	next, _ := old.start(testTimeout)

	if oldCtx.Err() == nil {
		t.Error("새로 물었는데 앞 요청이 아직 돈다")
	}
	if next.fresh(oldSeq) {
		t.Error("앞 물음의 답이 새 턴에 앉는다")
	}
	if !next.fresh(next.seq) {
		t.Error("새 턴이 자기 답을 못 받는다")
	}
}
