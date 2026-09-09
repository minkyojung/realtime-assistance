package musicapp

import (
	"testing"
	"time"

	"amcli/tui/internal/data"
	"amcli/tui/internal/music"
)

// 곡이 끝나는 순간을 잡는다. 판정은 저장하지 않으므로 여기서 확인할 것은
// 둘이다 — 언제 한 줄이 나가는가, 그리고 **왜 끝났다고 적는가.**

// atTrack 은 그 곡을 듣고 있는 모델이다.
func atTrack(pid string, posMs, durMs int) Model {
	m := New()
	m.bodyH = 20
	m.live = music.PlayerState{
		Playing: true, PersistentID: pid, PositionMs: posMs, DurationMs: durMs,
	}
	return m
}

func playing(pid string, posMs, durMs int) music.PlayerState {
	return music.PlayerState{Playing: true, PersistentID: pid, PositionMs: posMs, DurationMs: durMs}
}

// 같은 곡이 흐르는 중에는 아무것도 안 적는다.
func TestNothingIsWrittenWhileATrackPlays(t *testing.T) {
	m := atTrack("A", 30_000, 200_000)
	if _, cmd := m.notePlayback(playing("A", 31_000, 200_000)); cmd != nil {
		t.Error("아직 흐르는 중인데 한 줄 적었다")
	}
}

// 처음 본 것은 끝난 곡이 없다.
func TestFirstPollWritesNothing(t *testing.T) {
	m := New()
	m.bodyH = 20
	if _, cmd := m.notePlayback(playing("A", 0, 200_000)); cmd != nil {
		t.Error("처음 봤는데 끝난 곡이 있다고 한다")
	}
}

// 곡이 바뀌면 직전 곡을 적는다.
func TestTrackChangeWritesTheOneThatEnded(t *testing.T) {
	lib := data.Lib().Songs()
	if len(lib) < 2 || lib[0].PersistentId == nil {
		t.Skip("픽스처에 persistent ID 가 없다")
	}
	m := atTrack(*lib[0].PersistentId, 3_000, 200_000)
	if _, cmd := m.notePlayback(playing("OTHER", 0, 100_000)); cmd == nil {
		t.Error("곡이 바뀌었는데 아무것도 안 적었다")
	}
}

// 무엇이 끝냈는가 — 이 기록의 존재 이유다.
func TestWhyItEnded(t *testing.T) {
	for _, c := range []struct {
		name string
		set  func(Model) Model
		prev music.PlayerState
		next music.PlayerState
		want data.EndedBy
	}{
		{"끝까지 흘렀다", nil,
			playing("A", 199_000, 200_000), playing("B", 0, 100_000), data.EndedDone},
		{"3초에 나갔다", nil,
			playing("A", 3_000, 200_000), playing("B", 0, 100_000), data.EndedSkipped},
		{"멈췄다", nil,
			playing("A", 3_000, 200_000), music.PlayerState{Stopped: true}, data.EndedStopped},

		// **우리가 끊은 것.** 표시가 없으면 위의 "3초에 나갔다"와 구별되지 않는다.
		{"우리가 큐를 갈아끼웠다",
			func(m Model) Model { return m.hintEnd(data.EndedRequeue) },
			playing("A", 3_000, 200_000), playing("B", 0, 100_000), data.EndedRequeue},
		{"사용자가 큐에서 뺐다",
			func(m Model) Model { return m.hintEnd(data.EndedRemoved) },
			playing("A", 3_000, 200_000), playing("B", 0, 100_000), data.EndedRemoved},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := New()
			m.bodyH = 20
			if c.set != nil {
				m = c.set(m)
			}
			if got := m.whyEnded(c.prev, c.next); got != c.want {
				t.Errorf("%q 라고 한다 — %q 여야 한다", got, c.want)
			}
		})
	}
}

// 오래 남은 표시는 엉뚱한 곡에 붙는다. 시효를 넘기면 버린다.
func TestStaleHintIsIgnored(t *testing.T) {
	m := New()
	m.bodyH = 20
	m.endHint, m.endHintAt = data.EndedRequeue, time.Now().Add(-endHintTTL-time.Second)

	if got := m.whyEnded(playing("A", 3_000, 200_000), playing("B", 0, 100_000)); got != data.EndedSkipped {
		t.Errorf("낡은 표시를 그대로 쓴다: %q", got)
	}
}

// 한 곡 반복에서는 위치가 뒤로 가는 것이 곡이 끝나는 유일한 모습이다.
func TestRepeatOneCountsAsAnEnding(t *testing.T) {
	lib := data.Lib().Songs()
	if len(lib) == 0 || lib[0].PersistentId == nil {
		t.Skip("픽스처에 persistent ID 가 없다")
	}
	pid := *lib[0].PersistentId
	m := atTrack(pid, 198_000, 200_000)
	if _, cmd := m.notePlayback(playing(pid, 500, 200_000)); cmd == nil {
		t.Error("같은 곡이 처음부터 다시 시작했는데 한 판이 끝난 것으로 안 봤다")
	}
}

// 폴링 → 기록까지 실제로 이어지는지 한 번 통째로 확인한다.
//
// 조각마다 맞아도 배선이 빠지면 아무것도 안 쌓인다. 실제로 파일까지 간다.
func TestPollingActuallyWritesAFile(t *testing.T) {
	t.Setenv("AMCLI_STATE_DIR", t.TempDir())

	lib := data.Lib().Songs()
	if len(lib) < 2 || lib[0].PersistentId == nil {
		t.Skip("픽스처에 persistent ID 가 없다")
	}
	first := *lib[0].PersistentId

	// 3초쯤 듣고 있는 상태를 만든다.
	m := atTrack(first, 3_000, 200_000)
	m.queueTitle = "조용한 거"
	// 커버를 넘어갈 곡의 것으로 미리 맞춰 둔다. 안 그러면 곡이 바뀔 때
	// 커버·가사 Cmd 가 함께 묶여 나오고, 테스트가 그것을 돌리면 진짜
	// Music.app 과 남의 서버를 두드린다. 여기서 볼 것은 기록뿐이다.
	m.artPID = "OTHER"

	next, cmd := m.Update(StatusMsgFor(playing("OTHER", 0, 100_000), nil))
	if cmd == nil {
		t.Fatal("곡이 바뀌었는데 아무 Cmd 도 안 나왔다")
	}
	cmd() // 파일에 쓰는 것은 Cmd 안이다

	got, err := data.LoadPlays()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("한 줄이 남아야 하는데 %d줄이다", len(got))
	}
	p := got[0]
	if p.TrackID != lib[0].Id {
		t.Errorf("엉뚱한 곡을 적었다: %d", p.TrackID)
	}
	if p.PlayedMs != 3_000 || p.DurMs != 200_000 {
		t.Errorf("들은 만큼이 안 맞는다: %dms / %dms", p.PlayedMs, p.DurMs)
	}
	if p.EndedBy != data.EndedSkipped {
		t.Errorf("왜 끝났는지가 %q 다", p.EndedBy)
	}
	if p.Context != "조용한 거" {
		t.Errorf("어느 요청이었는지를 안 적었다: %q", p.Context)
	}
	_ = next
}
