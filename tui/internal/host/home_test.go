package host

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func homeHost() tea.Model {
	hm := New(
		stubApp{name: "alpha"},
		stubApp{name: "beta", ready: errors.New("sign in required")},
	)
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 28})
	return m
}

// state 는 Update 가 돌려준 모델을 들여다본다.
// Update 는 값 수신자라 New 가 준 변수는 갱신되지 않는다.
func state(t *testing.T, m tea.Model) Model {
	t.Helper()
	hm, ok := m.(Model)
	if !ok {
		t.Fatalf("Update 가 Model 을 안 돌려줬다: %T", m)
	}
	return hm
}

func key(m tea.Model, r rune) (tea.Model, tea.Cmd) {
	return m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
}

// deliverBatch 는 Cmd 가 낸 메시지를 모델에 먹인다. Batch 는 펼친다.
// 호스트가 자기 메시지로 상태를 바꾸므로, 그 한 바퀴를 돌려봐야 한다.
func deliverBatch(t *testing.T, m tea.Model, cmd tea.Cmd) tea.Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	switch msg := cmd().(type) {
	case nil:
	case tea.BatchMsg:
		for _, c := range msg {
			m = deliverBatch(t, m, c)
		}
	case switchAppMsg:
		m, _ = m.Update(msg)
	}
	return m
}

// 홈은 여기 뭐가 있는지와, 그중 무엇이 막혔는지를 말한다.
func TestHomeListsAppsAndGates(t *testing.T) {
	out := homeHost().View().Content

	for _, want := range []string{
		"/alpha", "alpha does things", // 되는 앱은 소개를 보여주고
		"/beta", "sign in required", // 막힌 앱은 사유를 보여준다
	} {
		if !strings.Contains(out, want) {
			t.Errorf("홈에 %q 가 없다", want)
		}
	}
	// 막힌 앱은 소개 대신 사유를 쓴다. 둘을 같이 보여주지 않는다.
	if strings.Contains(out, "beta does things") {
		t.Error("막힌 앱인데 소개를 그대로 보여준다")
	}
}

// 홈은 커튼이 아니다. 글자를 쳐도 걷히지 않는다.
func TestTypingDoesNotLeaveHome(t *testing.T) {
	m, _ := key(homeHost(), 'a')

	hm := state(t, m)
	if !hm.home {
		t.Fatal("글자 하나에 홈이 걷혔다")
	}
	if got := hm.input.Value(); got != "a" {
		t.Errorf("친 글자가 입력창에 없다: %q", got)
	}
	if !strings.Contains(m.View().Content, "/alpha") {
		t.Error("글자를 쳤다고 홈 목록이 사라졌다")
	}
}

// ↑↓ 로 고르고 enter 로 들어간다. 팔레트와 같은 조작이다.
func TestEnterEntersThePickedApp(t *testing.T) {
	m, _ := homeHost().Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if got := state(t, m).pick; got != 1 {
		t.Fatalf("↓ 를 눌렀는데 고른 줄이 %d 다", got)
	}

	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = deliverBatch(t, m, cmd)

	hm := state(t, m)
	if hm.home {
		t.Fatal("enter 를 눌렀는데 홈에 남아 있다")
	}
	if hm.current != 1 {
		t.Errorf("고른 앱이 아니라 %d번 앱에 들어갔다", hm.current)
	}
}

// 앱에서 esc 면 홈이다. 예전에는 여기서 곧장 종료였다.
func TestEscFromAppReturnsHome(t *testing.T) {
	m, cmd := homeHost().Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = deliverBatch(t, m, cmd)
	if state(t, m).home {
		t.Fatal("앱에 못 들어갔다")
	}

	m, quit := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if quit != nil {
		if _, isQuit := quit().(tea.QuitMsg); isQuit {
			t.Fatal("앱에서 esc 를 눌렀는데 종료됐다")
		}
	}
	if !state(t, m).home {
		t.Error("앱에서 esc 를 눌렀는데 홈으로 안 왔다")
	}
}

// 홈에서 esc 는 종료다. 더 물러날 곳이 없다.
func TestEscAtHomeQuits(t *testing.T) {
	_, cmd := homeHost().Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("홈에서 esc 를 눌렀는데 아무 일도 없다")
	}
	if _, isQuit := cmd().(tea.QuitMsg); !isQuit {
		t.Error("홈에서 esc 를 눌렀는데 종료가 아니다")
	}
}

// 문장을 쳐서 라우터가 앱을 지목하면 그 앱으로 들어간다.
func TestRoutedRequestEntersTheApp(t *testing.T) {
	m, cmd := homeHost().Update(routedMsg{prompt: "뭐 좀 해줘", apps: []string{"beta"}})
	m = deliverBatch(t, m, cmd)

	hm := state(t, m)
	if hm.home {
		t.Fatal("라우터가 앱을 지목했는데 홈에 남아 있다")
	}
	if hm.current != 1 {
		t.Errorf("지목된 앱이 아니라 %d번 앱에 들어갔다", hm.current)
	}
}

// 라우터가 모르면 홈에 머문다. 고르지도 않은 화면이 튀어나오면 안 된다.
func TestUnroutableRequestStaysHome(t *testing.T) {
	m, cmd := homeHost().Update(routedMsg{prompt: "안녕", apps: nil})
	m = deliverBatch(t, m, cmd)

	hm := state(t, m)
	if !hm.home {
		t.Fatal("어디로 갈지 모르는데 앱이 열렸다")
	}
	if len(hm.log) == 0 || !hm.log[len(hm.log)-1].err {
		t.Error("모르겠다는 말을 로그에 안 남겼다")
	}
}

// 홈에서 들어갈 때는 나가는 앱이 없다. blur 가 새면 안 된다.
func TestEnteringFromHomeBlursNothing(t *testing.T) {
	var aF, aB, bF, bB int
	hm := New(
		stubApp{name: "alpha", focus: &aF, blur: &aB},
		stubApp{name: "beta", focus: &bF, blur: &bB},
	)
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 28})
	m, _ = m.Update(switchAppMsg{index: 1})

	if aB != 0 || bB != 0 {
		t.Errorf("홈에서 들어가는데 blur 가 나갔다: alpha %d, beta %d", aB, bB)
	}
	if bF != 1 || aF != 0 {
		t.Errorf("포커스가 잘못 갔다: alpha %d, beta %d — 원한 것 0, 1", aF, bF)
	}
}

// 홈에서는 앱 명령을 내놓지 않는다. 들어가지도 않은 앱을 조작하면 안 된다.
func TestHomePaletteOffersAppsNotAppCommands(t *testing.T) {
	m, _ := key(homeHost(), '/')

	var names []string
	for _, c := range state(t, m).allCommands() {
		names = append(names, c.Name)
	}
	joined := strings.Join(names, " ")
	for _, want := range []string{"/alpha", "/beta", "/help"} {
		if !strings.Contains(joined, want) {
			t.Errorf("홈 팔레트에 %q 가 없다: %v", want, names)
		}
	}
}

// 홈에는 거를 목록이 없다. 조용히 무시하면 고장으로 보인다.
func TestSearchAtHomeSaysSo(t *testing.T) {
	m, _ := homeHost().Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})

	hm := state(t, m)
	if hm.mode == modeSearch {
		t.Error("홈인데 검색 모드로 들어갔다")
	}
	if hm.notice == "" {
		t.Error("왜 안 되는지 말하지 않았다")
	}
}

// 홈과 앱의 본문 높이가 같아야 한다. 아니면 들어갈 때 화면이 튄다.
func TestHomeKeepsBodyHeight(t *testing.T) {
	m := homeHost()
	before := strings.Count(m.View().Content, "\n")

	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = deliverBatch(t, m, cmd)
	if after := strings.Count(m.View().Content, "\n"); after != before {
		t.Errorf("앱에 들어가며 화면이 %d줄에서 %d줄로 튀었다", before+1, after+1)
	}
}

// 세로가 모자라면 안경부터 접는다. 앱 목록이 안경보다 먼저다.
func TestHomeFoldsGlassesWhenShort(t *testing.T) {
	hm := New(stubApp{name: "alpha"}, stubApp{name: "beta"})
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 12})

	out := m.View().Content
	if strings.Contains(out, sunglasses[1]) {
		t.Error("자리가 없는데 안경을 그렸다")
	}
	if !strings.Contains(out, "/alpha") || !strings.Contains(out, "/beta") {
		t.Error("안경을 접었으면 앱 목록은 남아야 한다")
	}
}

// 홈도 어느 폭에서든 넘치지 않아야 한다.
func TestHomeFitsWidth(t *testing.T) {
	for _, w := range []int{70, 80, 96, 120, 200} {
		hm := New(
			stubApp{name: "alpha"},
			stubApp{name: "beta", ready: errors.New("sign in required")},
		)
		var m tea.Model = &hm
		m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: 32})

		for i, line := range strings.Split(m.View().Content, "\n") {
			if got := lipgloss.Width(line); got > w {
				t.Errorf("터미널 폭 %d: %d번째 줄이 %d칸으로 넘침\n%q", w, i+1, got, line)
			}
		}
	}
}
