package data

import (
	"os"
	"testing"
	"time"
)

// 덧붙이기만 하고 고치지 않는다. 그래서 확인할 것은 둘이다 —
// 적은 것이 그대로 나오는가, 그리고 무엇이 취향의 근거인가.

func TestAppendAndLoad(t *testing.T) {
	t.Setenv("AMCLI_STATE_DIR", t.TempDir())

	want := []Play{
		{At: time.Now().UTC().Truncate(time.Second), TrackID: 11, PlayedMs: 3000, DurMs: 245000, EndedBy: EndedSkipped, Context: "조용한 거"},
		{At: time.Now().UTC().Truncate(time.Second), TrackID: 22, PlayedMs: 245000, DurMs: 245000, EndedBy: EndedDone, Shuffle: true},
	}
	for _, p := range want {
		if err := AppendPlay(p); err != nil {
			t.Fatalf("적지 못했다: %v", err)
		}
	}

	got, err := LoadPlays()
	if err != nil {
		t.Fatalf("읽지 못했다: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("%d개를 적었는데 %d개가 나온다", len(want), len(got))
	}
	for i := range got {
		if got[i].TrackID != want[i].TrackID || got[i].PlayedMs != want[i].PlayedMs ||
			got[i].EndedBy != want[i].EndedBy || got[i].Shuffle != want[i].Shuffle ||
			got[i].Context != want[i].Context || !got[i].At.Equal(want[i].At) {
			t.Errorf("%d번째가 달라졌다\n적은 것 %+v\n나온 것 %+v", i, want[i], got[i])
		}
	}
}

// 아직 아무것도 안 들은 것은 오류가 아니다.
func TestNoFileIsNotAnError(t *testing.T) {
	t.Setenv("AMCLI_STATE_DIR", t.TempDir())
	got, err := LoadPlays()
	if err != nil {
		t.Errorf("파일이 없다고 실패한다: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("없는데 %d개가 나온다", len(got))
	}
}

// 쓰다 만 줄 하나 때문에 지난 기록을 통째로 잃으면 안 된다.
func TestBrokenLineDoesNotLoseTheRest(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AMCLI_STATE_DIR", dir)

	if err := AppendPlay(Play{At: time.Now(), TrackID: 11, EndedBy: EndedDone}); err != nil {
		t.Fatal(err)
	}
	// 쓰다 죽은 줄을 흉내 낸다.
	f, _ := os.OpenFile(dir+"/plays.jsonl", os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString("{\"trackId\":22,\"play\n")
	f.Close()
	if err := AppendPlay(Play{At: time.Now(), TrackID: 33, EndedBy: EndedDone}); err != nil {
		t.Fatal(err)
	}

	got, err := LoadPlays()
	if err != nil {
		t.Fatalf("깨진 줄에 통째로 실패했다: %v", err)
	}
	if len(got) != 2 || got[0].TrackID != 11 || got[1].TrackID != 33 {
		t.Errorf("멀쩡한 줄을 잃었다: %+v", got)
	}
}

// **우리가 끊은 것은 취향의 근거가 아니다.**
//
// 구별하지 못하면 우리가 만든 행동을 사용자의 취향으로 읽고, 쌓일수록
// 더 확신하면서 틀린다.
func TestOurOwnEditsAreNotSignals(t *testing.T) {
	for _, c := range []struct {
		end  EndedBy
		want bool
	}{
		{EndedDone, true},
		{EndedSkipped, true},
		{EndedRemoved, true},
		{EndedPicked, true},
		{EndedRequeue, false}, // 우리가 큐를 갈아끼웠다
		{EndedStopped, false}, // 그냥 멈춘 것은 거절이 아니다
	} {
		if got := c.end.Signal(); got != c.want {
			t.Errorf("%q 를 신호로 %v 라고 한다 — %v 여야 한다", c.end, got, c.want)
		}
	}
}

// 캐시가 아니라 상태다. 캐시는 지워져도 되지만 이것은 안 된다.
func TestPlaysLiveOutsideTheCache(t *testing.T) {
	t.Setenv("AMCLI_STATE_DIR", "")
	p, err := playsPath()
	if err != nil {
		t.Fatal(err)
	}
	if contains(p, "Caches") {
		t.Errorf("캐시에 적는다 — 지워지면 영영 못 되찾는다: %s", p)
	}
}

func contains(hay, needle string) bool {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
