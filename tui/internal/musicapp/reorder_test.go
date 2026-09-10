package musicapp

import (
	"testing"

	"amcli/tui/internal/api"
)

// 큐를 만든 뒤에 손대는 길. 애플뮤직의 Play Next · Play Later · 순서 바꾸기.
//
// 지키려는 것은 하나다 — **듣던 곡이 안 끊긴다.** 그 조건은 고치는 자리가
// 지금 나오는 곡보다 뒤인 것이고, 그것을 여기서 지킨다.

// queued 는 큐가 채워진 모델을 만든다. 커서는 Queue 섹션의 at 번째에 둔다.
func queued(t *testing.T, n, at int) Model {
	t.Helper()
	src := atSection(t, secSongs)
	rows := src.rows()
	if len(rows) < n {
		t.Skipf("픽스처에 곡이 %d개 없다", n)
	}
	m, _ := src.playFrom(rows[:n], 0, "test")
	m = atQueueOf(t, m)
	m.queuePID = "PID" // 화면과 Music.app 이 맞은 상태
	m.bodyH = 40
	m.listIdx = at
	return m
}

func ids(m Model) []int64 {
	out := make([]int64, 0, len(m.queue))
	for _, it := range m.queue {
		out = append(out, it.Track.Id)
	}
	return out
}

// /next 는 고른 곡을 지금 나오는 곡 바로 뒤로 보낸다.
func TestPlayNextLandsRightAfterTheCurrentTrack(t *testing.T) {
	m := queued(t, 5, 3)
	want := m.queue[3].Track.Id

	m, cmd := m.reorder(putNext)
	if cmd == nil {
		t.Fatal("아무 일도 안 일어났다")
	}
	if got := ids(m); got[1] != want {
		t.Errorf("지금 곡 바로 뒤여야 하는데 %v 다", got)
	}
	if len(m.queue) != 5 {
		t.Errorf("옮긴 것인데 곡 수가 %d 로 달라졌다", len(m.queue))
	}
}

// /later 는 맨 뒤로 보낸다.
func TestPlayLaterLandsAtTheEnd(t *testing.T) {
	m := queued(t, 5, 2)
	want := m.queue[2].Track.Id

	m, cmd := m.reorder(putLater)
	if cmd == nil {
		t.Fatal("아무 일도 안 일어났다")
	}
	if got := ids(m); got[len(got)-1] != want {
		t.Errorf("맨 뒤여야 하는데 %v 다", got)
	}
	if len(m.queue) != 5 {
		t.Errorf("옮긴 것인데 곡 수가 %d 로 달라졌다", len(m.queue))
	}
}

// 큐에 없던 곡은 끼워 넣는다. 애플뮤직에서 아무 곡이나 골라 눌렀을 때와 같다.
func TestPlayNextInsertsATrackFromOutsideTheQueue(t *testing.T) {
	m := queued(t, 3, 0)
	before := len(m.queue)

	// 라이브러리에서 큐에 없는 곡을 하나 고른다.
	lib := atSection(t, secSongs)
	var outsider api.Track
	for _, r := range lib.rows() {
		if r.track != nil && m.queueAt(r.track.Id) < 0 {
			outsider = *r.track
			break
		}
	}
	if outsider.PersistentId == nil {
		t.Skip("큐 밖의 곡이 픽스처에 없다")
	}

	m, cmd := m.place(outsider, m.playingAt()+1, "")
	if cmd == nil {
		t.Fatal("아무 일도 안 일어났다")
	}
	if len(m.queue) != before+1 {
		t.Errorf("한 곡 늘어야 하는데 %d → %d", before, len(m.queue))
	}
	if m.queue[1].Track.Id != outsider.Id {
		t.Error("끼운 곡이 지금 곡 바로 뒤에 없다")
	}
}

// **지금 나오는 곡은 못 옮긴다.** 지웠다 다시 붙이면 소리가 끊긴다.
// 애플뮤직도 재생 중인 곡은 옮기지 못한다.
func TestTheCurrentTrackCannotBeMoved(t *testing.T) {
	m := queued(t, 5, 0) // 0번이 지금 나오는 곡
	before := ids(m)

	next, _ := m.reorder(moveDown)
	if got := ids(next); !sameIDs(got, before) {
		t.Errorf("지금 나오는 곡을 옮겼다: %v → %v", before, got)
	}
}

// 지금 곡 **앞으로는** 못 간다. 그 자리를 고치려면 지금 곡을 지워야 한다.
func TestNothingMovesAheadOfTheCurrentTrack(t *testing.T) {
	m := queued(t, 5, 1) // 지금 곡(0) 바로 뒤
	before := ids(m)

	next, _ := m.reorder(moveUp)
	if got := ids(next); !sameIDs(got, before) {
		t.Errorf("지금 나오는 곡을 넘어갔다: %v → %v", before, got)
	}
}

// /up /down 은 한 칸씩 옮기고, 커서가 곡을 따라간다.
func TestShiftMovesOneSlotAndTheCursorFollows(t *testing.T) {
	m := queued(t, 5, 2)
	want := m.queue[2].Track.Id

	m, cmd := m.reorder(moveDown)
	if cmd == nil {
		t.Fatal("아무 일도 안 일어났다")
	}
	if m.queue[3].Track.Id != want {
		t.Errorf("한 칸 아래로 안 갔다: %v", ids(m))
	}
	if m.listIdx != 3 {
		t.Errorf("커서가 곡을 안 따라갔다: %d", m.listIdx)
	}
}

// 끝에서는 더 못 간다.
func TestShiftStopsAtTheEnd(t *testing.T) {
	m := queued(t, 4, 3)
	before := ids(m)

	next, _ := m.reorder(moveDown)
	if got := ids(next); !sameIDs(got, before) {
		t.Errorf("맨 끝인데 더 내려갔다: %v", got)
	}
}

// 자리번호를 다시 매긴다. 화면이 그것으로 줄을 센다.
func TestPositionsAreRenumberedAfterAMove(t *testing.T) {
	m := queued(t, 5, 2)
	m, _ = m.reorder(moveDown)
	for i, it := range m.queue {
		if it.Position != i+1 {
			t.Errorf("%d번 줄의 자리번호가 %d 다", i, it.Position)
		}
	}
}

// 큐가 비어 있으면 "다음에 틀어줘" 는 "지금 틀어줘" 와 같은 말이다.
func TestPlayNextOnAnEmptyQueueJustPlays(t *testing.T) {
	m := atSection(t, secSongs)
	m.bodyH = 20
	if len(m.queue) != 0 {
		t.Skip("큐가 비어 있지 않다")
	}
	next, cmd := m.reorder(putNext)
	if cmd == nil {
		t.Fatal("아무 일도 안 일어났다")
	}
	if len(next.queue) == 0 {
		t.Error("빈 큐에서 눌렀는데 아무것도 안 담겼다")
	}
}

// 화면과 Music.app 이 어긋나 있으면 자리번호를 믿을 수 없다. 손대지 않는다.
func TestNoSurgeryWhileTheQueueIsBeingWritten(t *testing.T) {
	m := queued(t, 5, 3)
	m.queuePID = "" // 아직 안 써진 상태
	before := ids(m)

	next, _ := m.reorder(putNext)
	if got := ids(next); !sameIDs(got, before) {
		t.Errorf("아직 안 써진 큐를 고쳤다: %v", got)
	}
}

func sameIDs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
