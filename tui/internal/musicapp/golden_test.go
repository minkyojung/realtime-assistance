package musicapp

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"amcli/tui/internal/app"
	"amcli/tui/internal/intent"
	"amcli/tui/internal/music"
)

// 앱 화면을 통째로 박아둔다. 리팩터링으로 한 바이트라도 달라지면 여기서 걸린다.
//
// 호스트 골든(host/golden_test.go)이 못 만드는 상태다 — 재생 상태와 큐는
// 이 패키지의 비공개 메시지로만 앉힐 수 있다. 그래서 다시 뜨는 법도 다르다:
//
//	go test ./internal/musicapp -run Golden -update
var update = flag.Bool("update", false, "골든을 지금 화면으로 다시 뜬다")

func TestGolden(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "golden-fixture") // host 골든과 같은 이유

	got := renderScreens(t)
	path := "testdata/golden.txt"
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v — 처음이면 -update 로 뜬다", err)
	}
	if got != string(want) {
		t.Errorf("화면이 달라졌다.\n--- got ---\n%s", got)
	}
}

func renderScreens(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	shot := func(name string, m Model) {
		fmt.Fprintf(&b, "=== %s ===\n%s\n", name, m.View(100, 24))
	}

	// 재생 중 — 재생 바가 위치와 길이를 그린다. 커버·가사는 안 받은 상태.
	// 위치는 폴링 값 그대로다(보간은 가사에만). 그래서 결정적이다.
	{
		m := New()
		next, _ := m.Update(statusMsg{state: music.PlayerState{
			Playing: true, PersistentID: "5D6BA5B487F37235",
			Title: "Blood Bank", Artist: "Bon Iver",
			PositionMs: 61000, DurationMs: 284132,
		}})
		shot("playing", next.(Model))
	}

	// 큐를 앉힌 직후 — 제목·한 줄 설명·곡마다의 근거.
	{
		m := New()
		next, _ := m.Update(app.AskMsg{Prompt: "something quiet"})
		mm := next.(Model)
		next, _ = mm.Update(queueMsg{seq: mm.ask.seq, res: intent.Result{
			Title: "A quiet hour",
			Note:  "Five tracks you own but have not played in months.",
			Picks: []intent.Pick{
				{TrackID: 1, Reason: "never played, added a year ago"},
				{TrackID: 3, Reason: "your most-skipped Bon Iver — worth one more try"},
			},
		}})
		shot("queue", next.(Model))
	}
	return b.String()
}
