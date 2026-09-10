package musicapp

import (
	"context"
	"time"
)

// turn 은 지금 도는 요청 하나다 — 번호와 취소 손잡이와 기다리는 중인지.
//
// 셋이 필드로 흩어져 있으면 한 곳만 어긋나도 조용히 틀어진다. 손잡이를
// 부르면서 번호를 안 올리면 이미 날아간 요청의 답이 화면에 앉고, 번호만
// 올리고 손잡이를 안 부르면 요청은 배경에서 계속 돌며 토큰을 쓴다.
// 둘 다 증상이 늦게, 다른 자리에서 나타난다.
//
// 그래서 셋을 한 덩어리로 묶고 바꾸는 길을 네 개로만 낸다 — start·extend·
// done·stop. 어느 길로 가도 셋이 함께 움직인다.
//
// pi 의 턴 스냅샷과 같은 장치다. 턴이 열릴 때 번호를 뜨고, 도는 동안 밖에서
// 무슨 일이 일어나도 그 턴은 자기 번호로만 판단한다.
type turn struct {
	// seq 는 몇 번째 물음인가. 늦게 온 답을 거르는 유일한 근거다.
	seq int

	// live 는 답을 기다리는 중인가.
	live bool

	// cancel 은 도는 요청을 끊는 손잡이다. live 가 아니면 nil 이다.
	cancel context.CancelFunc

	// ctx 는 이 턴 안에서 나가는 모든 요청이 매달릴 자리다.
	//
	// 손잡이와 함께 둔다. 한 턴이 여러 걸음으로 나뉘고 걸음마다 도구가
	// 요청을 더 낼 수 있으므로, 그때 쓸 ctx 를 찾아 헤매지 않아야 한다.
	ctx context.Context
}

// start 는 새 턴을 연다. 앞의 턴은 버린다 — 한 번에 하나만 기다린다.
//
// 버리는 것과 여는 것을 이어 붙인 것뿐이다. 번호는 버릴 때 한 번만 오른다.
func (t turn) start(timeout time.Duration) (turn, context.Context) {
	return t.stop().extend(timeout)
}

// extend 는 같은 턴을 이어간다. 판단이 끝나고 선곡으로 넘어갈 때처럼,
// 한 번의 물음이 두 걸음으로 나뉠 때 쓴다.
//
// 번호를 그대로 두므로 도중에 그만두면 두 걸음이 함께 버려진다. 앞 걸음의
// 손잡이는 여기서 놓는다 — 걸음마다 손잡이가 쌓이면 놓지 못한 것이 남는다.
func (t turn) extend(timeout time.Duration) (turn, context.Context) {
	if t.cancel != nil {
		t.cancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.live = true
	t.cancel = cancel
	t.ctx = ctx
	return t, ctx
}

// done 은 답이 도착해 턴이 끝났다는 뜻이다.
//
// 번호를 올리지 않는다. 올릴 이유가 없기 때문이다 — 답은 이미 왔고,
// 같은 번호로 또 올 것이 없다. "그만뒀다"와 구별되어야 읽을 때 헷갈리지 않는다.
func (t turn) done() turn {
	if t.cancel != nil {
		t.cancel()
	}
	t.live = false
	t.cancel = nil
	t.ctx = nil
	return t
}

// stop 은 도는 턴을 버린다. 번호를 올려 늦게 오는 답까지 못 앉게 한다.
func (t turn) stop() turn {
	t = t.done()
	t.seq++
	return t
}

// fresh 는 이 답이 지금 기다리는 턴의 것인지 본다.
//
// live 까지 보는 이유는, 그만둔 직후에 도착한 답이 번호만으로는 통과할 수
// 있기 때문이다. 기다리지 않는데 온 답은 언제나 늦은 답이다.
func (t turn) fresh(seq int) bool { return t.live && seq == t.seq }
