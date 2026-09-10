package musicapp

import (
	"strings"
	"testing"

	"amcli/tui/internal/intent"
)

// AI 도 큐를 갈아끼우지 않고 붙일 수 있어야 한다.
//
// "이거 뒤에 몇 곡 더" 는 지금 듣는 것을 지키겠다는 말인데, 갈아끼우는
// 도구밖에 없던 동안에는 그 말이 늘 처음으로 튀는 결과가 됐다.

// picks 는 큐 밖의 곡들로 선곡 결과를 흉내 낸다.
func picksOutside(t *testing.T, m Model, n int) intent.Result {
	t.Helper()
	lib := atSection(t, secSongs)
	var res intent.Result
	res.Title = "more"
	res.Note = "added a few more"
	for _, r := range lib.rows() {
		if len(res.Picks) == n {
			break
		}
		if r.track != nil && m.queueAt(r.track.Id) < 0 {
			res.Picks = append(res.Picks, intent.Pick{TrackID: r.track.Id, Reason: "because"})
		}
	}
	if len(res.Picks) < n {
		t.Skipf("큐 밖의 곡이 %d개 없다", n)
	}
	return res
}

// 붙인다. 갈아끼우지 않는다 — 있던 곡이 그대로 있어야 한다.
func TestAddTracksKeepsWhatWasThere(t *testing.T) {
	m := queued(t, 4, 0)
	before := ids(m)
	res := picksOutside(t, m, 2)

	next, cmd := m.appendQueue(res, true)
	if cmd == nil {
		t.Fatal("아무 일도 안 일어났다")
	}
	got := ids(next.(Model))
	if len(got) != len(before)+2 {
		t.Fatalf("%d곡에 2곡을 붙였는데 %d곡이다", len(before), len(got))
	}
	for i, id := range before {
		if got[i] != id {
			t.Fatalf("있던 곡이 밀렸다: %v → %v", before, got)
		}
	}
}

// 지금 나오는 곡은 그대로다. 그것이 안 끊기는 조건이다.
func TestAddTracksDoesNotDisturbTheCurrentTrack(t *testing.T) {
	m := queued(t, 4, 0)
	playing := m.nowPlayingID
	res := picksOutside(t, m, 2)

	next, _ := m.appendQueue(res, true)
	mm := next.(Model)
	if mm.queue[0].Track.Id != playing {
		t.Error("지금 나오는 곡이 첫 자리에서 밀렸다 — 소리가 끊긴다")
	}
	if mm.nowPlayingID != playing {
		t.Error("지금 나오는 곡이 바뀌었다")
	}
}

// where=next 면 지금 곡 바로 뒤에 들어간다.
func TestAddTracksCanLandRightAfterTheCurrentTrack(t *testing.T) {
	m := queued(t, 4, 0)
	res := picksOutside(t, m, 2)
	want := res.Picks[0].TrackID

	next, _ := m.appendQueue(res, false)
	if got := ids(next.(Model)); got[1] != want {
		t.Errorf("지금 곡 바로 뒤여야 하는데 %v 다", got)
	}
}

// 이미 큐에 있는 곡은 안 붙인다.
//
// 같은 곡이 두 자리를 차지하면 자리번호로 곡을 찾는 길이 전부 흔들린다.
func TestAddTracksSkipsWhatIsAlreadyQueued(t *testing.T) {
	m := queued(t, 4, 0)
	var res intent.Result
	for _, it := range m.queue {
		res.Picks = append(res.Picks, intent.Pick{TrackID: it.Track.Id, Reason: "dup"})
	}
	before := len(m.queue)

	next, _ := m.appendQueue(res, true)
	if got := len(next.(Model).queue); got != before {
		t.Errorf("이미 있는 곡을 또 붙였다: %d → %d", before, got)
	}
}

// 붙일 큐가 없으면 붙이는 것이 곧 만드는 것이다.
func TestAddTracksOnAnEmptyQueueBuildsOne(t *testing.T) {
	m := atSection(t, secSongs)
	m.bodyH = 20
	if len(m.queue) != 0 {
		t.Skip("큐가 비어 있지 않다")
	}
	res := picksOutside(t, m, 2)

	next, cmd := m.appendQueue(res, true)
	if cmd == nil {
		t.Fatal("아무 일도 안 일어났다")
	}
	if len(next.(Model).queue) != 2 {
		t.Error("빈 큐에 붙였는데 큐가 안 생겼다")
	}
}

// 도구가 모델에게 보이고, 갈아끼우는 도구가 이쪽으로 길을 낸다.
func TestAddTracksIsOfferedAndBuildQueuePointsAtIt(t *testing.T) {
	m := New()
	var add, build string
	for _, ts := range m.tools() {
		switch ts.Name {
		case "add_tracks":
			add = ts.Description
		case "build_queue":
			build = ts.Description
		}
	}
	if add == "" {
		t.Fatal("add_tracks 가 도구 목록에 없다")
	}
	if !strings.Contains(add, "without interrupting") {
		t.Error("안 끊긴다는 것이 설명에 없다 — 모델이 고를 이유를 모른다")
	}
	if !strings.Contains(build, "add_tracks") {
		t.Error("갈아끼우는 도구가 붙이는 길을 안 가리킨다 — 습관대로 갈아끼운다")
	}
}
