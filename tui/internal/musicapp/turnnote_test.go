package musicapp

import (
	"testing"

	"amcli/tui/internal/data"
	"amcli/tui/internal/intent"
)

// 한 번 청한 것은 한 줄이다.
//
// 한 문장이 build_queue 를 부르고 이어서 add_tracks 를 부를 수 있다. 그래도
// 사람은 한 번 물어봤다. 두 줄로 나뉘면 그 뒤의 재생 기록이 두 요청에 갈라
// 붙어, 맥락별 집계가 조용히 반씩 센다.
func TestOneSentenceWritesOneTurn(t *testing.T) {
	t.Setenv("AMCLI_STATE_DIR", t.TempDir())

	m := New()
	m.asked = "공부할 때 들을 거"
	m.ask.seq = 3

	m = m.noteTurn(intent.Result{Title: "Calm Hour", Context: "focus"})
	first := m.turnID
	if first == 0 {
		t.Fatal("요청을 아예 안 적었다")
	}
	// 같은 물음이 도구를 한 번 더 부른다.
	m = m.noteTurn(intent.Result{Title: "Calm Hour +2", Context: "focus"})
	if m.turnID != first {
		t.Errorf("같은 물음에 번호가 새로 났다: %d → %d", first, m.turnID)
	}

	got, err := data.LoadTurns()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("한 줄이어야 하는데 %d줄이다", len(got))
	}
	if got[0].Prompt != "공부할 때 들을 거" {
		t.Errorf("사람이 친 문장이 아니라 %q 를 적었다", got[0].Prompt)
	}
	if got[0].Context != "focus" {
		t.Errorf("자리를 %q 로 적었다", got[0].Context)
	}
}

// 다음 물음은 새 요청이다.
func TestNextSentenceWritesANewTurn(t *testing.T) {
	t.Setenv("AMCLI_STATE_DIR", t.TempDir())

	m := New()
	m.asked, m.ask.seq = "조용한 거", 1
	m = m.noteTurn(intent.Result{Context: "focus"})
	first := m.turnID

	m.asked, m.ask.seq = "이번엔 신나는 거", 2
	m = m.noteTurn(intent.Result{Context: "workout"})
	if m.turnID == first {
		t.Error("새 물음인데 앞의 번호를 그대로 쓴다")
	}

	got, _ := data.LoadTurns()
	if len(got) != 2 {
		t.Fatalf("두 줄이어야 하는데 %d줄이다", len(got))
	}
	if got[1].Context != "workout" {
		t.Errorf("두 번째 자리가 %q 다", got[1].Context)
	}
}

// 사람이 목록에서 직접 고른 재생에는 요청이 없다. 0 은 빈 값이 아니라 뜻이다.
func TestManualPlayDetachesTheTurn(t *testing.T) {
	t.Setenv("AMCLI_STATE_DIR", t.TempDir())

	m := New()
	m.asked, m.ask.seq = "조용한 거", 1
	m = m.noteTurn(intent.Result{Context: "focus"})
	if m.turnID == 0 {
		t.Fatal("요청을 안 적었다")
	}

	songs := data.Lib().Songs()
	if len(songs) == 0 {
		t.Skip("픽스처가 비었다")
	}
	m.bodyH = 20
	next, _ := m.playFrom(trackRows(songs), 0, "Songs")
	if next.turnID != 0 {
		t.Errorf("직접 고른 재생에 요청 번호가 남았다: %d", next.turnID)
	}
}
