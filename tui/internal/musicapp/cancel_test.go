package musicapp

import (
	"testing"

	"amcli/tui/internal/app"
	"amcli/tui/internal/intent"
	tea "charm.land/bubbletea/v2"
)

// esc 로 그만둘 수 있어야 한다. 선곡은 실측 7초라 그 사이에 잘못 물어본
// 것을 알아채는데, 지금까지는 기다리는 수밖에 없었다.
//
// ctrl+c 로 하지 않는 이유는 그것이 터미널의 탈출구이기 때문이다.
func TestCancelStopsWaiting(t *testing.T) {
	m := askedSomething(t)

	next, _ := m.Update(app.CancelMsg{})
	if next.(Model).ask.live {
		t.Error("그만뒀는데 아직 기다린다고 표시한다")
	}
}

// 그만둔 요청의 답이 뒤늦게 와도 화면을 건드리면 안 된다.
// 취소했는데 잠시 뒤 큐가 통째로 갈리는 것이 제일 나쁜 상태다.
func TestCancelledRequestCannotTouchTheScreen(t *testing.T) {
	m := askedSomething(t)
	late := queueMsg{seq: m.ask.seq, res: intent.Result{Title: "생기면 안 되는 큐"}}

	next, _ := m.Update(app.CancelMsg{})
	after, _ := next.(Model).Update(late)

	if got := after.(Model).queueTitle; got != "" {
		t.Errorf("그만둔 요청의 답이 화면에 앉았다: %q", got)
	}
}

// 그만두지 않았으면 답은 당연히 화면에 앉는다. 위 테스트가 번호 때문이
// 아니라 그냥 늘 버려서 통과하는 것은 아닌지 확인한다.
func TestAnswerLandsWhenNotCancelled(t *testing.T) {
	m := askedSomething(t)

	after, _ := m.Update(queueMsg{seq: m.ask.seq, res: intent.Result{Title: "조용한 큐"}})
	if got := after.(Model).queueTitle; got != "조용한 큐" {
		t.Errorf("멀쩡한 답이 화면에 안 앉았다: %q", got)
	}
}

// askedSomething 은 요청 하나를 띄운 모델이다.
// Cmd 를 실행하지 않으므로 실제 API 는 부르지 않는다.
func askedSomething(t *testing.T) Model {
	t.Helper()
	// 키가 없으면 요청이 아예 안 나간다(startAsk).
	t.Setenv("OPENAI_API_KEY", "sk-test")
	m := New()
	m.bodyH = 20

	next, cmd := m.Update(app.AskMsg{Prompt: "조용한 걸로"})
	if cmd == nil {
		t.Fatal("물었는데 아무 일도 안 일어났다")
	}
	mm, ok := next.(Model)
	if !ok {
		t.Fatalf("Update 가 Model 을 안 돌려줬다: %T", next)
	}
	if !mm.ask.live {
		t.Fatal("물었는데 기다린다는 표시가 없다")
	}
	return mm
}

var _ tea.Msg = app.CancelMsg{}
