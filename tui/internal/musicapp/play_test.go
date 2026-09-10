package musicapp

import (
	"amcli/tui/internal/music"
	"testing"

	"amcli/tui/internal/data"
	tea "charm.land/bubbletea/v2"
)

// enter 의 뜻은 하나다 — **고른 곡부터 이 목록을 이어서 튼다.**
//
// 예전에는 자리마다 뜻이 달랐고(라이브러리 곡 / 큐의 곡 / 카탈로그 곡),
// 그래서 누르기 전에 무엇이 일어날지 알 수 없었다.

func atRow(t *testing.T, kind sectionKind, idx int) Model {
	t.Helper()
	m := atSection(t, kind)
	m.listIdx = idx
	m.clampList()
	return m
}

// 고른 곡이 큐의 첫 곡이 되고, 그 아래가 뒤따른다.
func TestEnterQueuesFromTheChosenTrack(t *testing.T) {
	m := atRow(t, secSongs, 3)
	rows := m.rows()
	want := rows[3].track.Id

	next, cmd := m.playSelected()
	if cmd == nil {
		t.Fatal("골랐는데 아무 일도 안 일어났다")
	}
	after := next.(Model)
	if len(after.queue) < 2 {
		t.Fatalf("큐가 %d곡이다 — 고른 곡 뒤가 따라와야 한다", len(after.queue))
	}
	if after.queue[0].Track.Id != want {
		t.Error("고른 곡이 첫 곡이 아니다")
	}
	if after.queue[1].Track.Id != rows[4].track.Id {
		t.Error("목록의 다음 곡이 뒤에 안 붙었다")
	}
}

// 목록 전체를 담지 않는다. 곡마다 AppleScript 가 한 번씩 돌기 때문이다.
func TestQueueFromAListIsBounded(t *testing.T) {
	m := atRow(t, secSongs, 0)
	if n := len(m.rows()); n <= maxPickQueue {
		t.Skipf("목록이 %d곡뿐이라 상한을 못 넘는다", n)
	}
	next, _ := m.playSelected()
	if got := len(next.(Model).queue); got > maxPickQueue {
		t.Errorf("%d곡을 담았다 — 상한은 %d", got, maxPickQueue)
	}
}

// **큐를 보고 있어도 같은 일이다.** 자리에 따라 뜻이 달라지면 안 된다.
func TestEnterMeansTheSameThingInsideTheQueue(t *testing.T) {
	from := atRow(t, secSongs, 0)
	next, _ := from.playSelected()
	m := next.(Model)
	if len(m.queue) < 3 {
		t.Skip("큐가 짧아 안쪽을 고를 수 없다")
	}

	m.jumpTo(secQueue, "")
	m.listIdx = 2
	m.clampList()
	want := m.rows()[2].track.Id

	next, cmd := m.playSelected()
	if cmd == nil {
		t.Fatal("큐 안에서 골랐는데 아무 일도 안 일어났다")
	}
	if got := next.(Model).queue[0].Track.Id; got != want {
		t.Error("큐 안에서 고른 곡이 첫 곡이 되지 않았다")
	}
}

// 묶음은 트는 것이 아니라 여는 것이다. 곡이 아니므로 뜻이 달라도 된다.
func TestGroupsStillOpenInsteadOfPlaying(t *testing.T) {
	m := atRow(t, secArtists, 0)
	next, _ := m.playSelected()
	after := next.(Model)
	if after.drill == nil {
		t.Error("아티스트에서 enter 가 파고들지 않았다")
	}
	if len(after.queue) != 0 {
		t.Error("묶음을 골랐는데 큐를 만들었다")
	}
}

// 튼다는 것은 언제나 **플레이리스트를 쓴다**는 뜻이다.
//
// 곡 객체 하나를 Music.app 에 주면 그것만 틀고 멈춘다. 한 곡을 트는 길이
// 남아 있으면 그 자리에서 다시 정적이 생긴다.
func TestPlayingAlwaysWritesAQueue(t *testing.T) {
	m := atRow(t, secSongs, 1)
	next, _ := m.playSelected()
	after := next.(Model)

	if len(after.queue) == 0 {
		t.Fatal("큐를 안 만들었다")
	}
	// 플레이리스트를 새로 쓰기 전이므로 번호를 믿지 않는 상태여야 한다.
	if after.queuePID != "" {
		t.Error("아직 안 썼는데 Music.app 과 같다고 표시한다")
	}
}

// 고른 곳이 어디였는지가 남는다. 상태줄과 재생 기록이 이것을 쓴다.
func TestPickRemembersWhereItCameFrom(t *testing.T) {
	m := atRow(t, secSongs, 0)
	next, _ := m.playSelected()
	if got := next.(Model).queueTitle; got == "" {
		t.Error("어디서 골랐는지를 안 남겼다")
	}
}

var _ tea.Cmd = cmdWriteQueue(music.AppleScript{}, nil, 1, 0)
var _ = data.EndedPicked
