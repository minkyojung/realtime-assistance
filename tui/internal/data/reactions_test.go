package data

import "testing"

func play(id int64, why EndedBy, playedMs, durMs int) Play {
	return Play{TrackID: id, EndedBy: why, PlayedMs: playedMs, DurMs: durMs}
}

// 절반을 못 채우고 나갔으면 일찍 나간 것이다.
func TestEarlyIsHalfTheTrack(t *testing.T) {
	r := Reactions([]Play{
		play(1, EndedSkipped, 10_000, 180_000),  // 10초 / 3분 — 일찍
		play(1, EndedSkipped, 170_000, 180_000), // 거의 다 듣고 넘김 — 아니다
	})
	if got := r[1].Early; got != 1 {
		t.Errorf("일찍 나간 것이 1이어야 하는데 %d 다", got)
	}
}

// **우리가 만든 사건은 안 센다.**
//
// 큐를 갈아끼우느라 끊긴 곡을 취향으로 읽으면, 우리 행동을 사용자의
// 마음으로 착각한 채 쌓일수록 더 확신하면서 틀린다.
func TestOurOwnDoingIsNotEvidence(t *testing.T) {
	r := Reactions([]Play{
		play(1, EndedRequeue, 3_000, 180_000),
		play(1, EndedStopped, 3_000, 180_000),
	})
	if got, ok := r[1]; ok && (got.Early+got.Done+got.Removed) > 0 {
		t.Errorf("우리가 끊은 것을 셌다: %+v", got)
	}
}

// 끝까지 들은 것과 지목해서 뺀 것은 따로 센다. 세기가 다른 신호다.
func TestFinishedAndRemovedAreCountedApart(t *testing.T) {
	r := Reactions([]Play{
		play(1, EndedDone, 180_000, 180_000),
		play(1, EndedRemoved, 5_000, 180_000),
	})
	if r[1].Done != 1 || r[1].Removed != 1 {
		t.Errorf("따로 안 셌다: %+v", r[1])
	}
}

// 길이를 모르면 일찍인지 알 수 없다. 0으로 나누지도, 짐작하지도 않는다.
func TestUnknownLengthIsNotCountedAsEarly(t *testing.T) {
	r := Reactions([]Play{play(1, EndedSkipped, 3_000, 0)})
	if r[1].Early != 0 {
		t.Error("길이를 모르는데 일찍 나갔다고 셌다")
	}
}
