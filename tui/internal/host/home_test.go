package host

import (
	"errors"
	"strings"
	"testing"

	"amcli/tui/internal/app"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// 이름표는 픽셀 그림을 반블록으로 옮긴 것이라 원본 문자열이 그대로
// 나오지 않는다. 획 속은 위아래 픽셀이 둘 다 차서 꽉 찬 블록이 이어지므로,
// 그 조각으로 이름표가 그려졌는지만 본다.
var wordmarkMark = strings.Repeat("█", 7)

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

// 홈 박스 — 자리가 넉넉할 때만 선다.
//
// 박스는 이름표 17줄 아래에 서고, 스스로 열두 줄 안팎을 쓴다. 아래 시험이
// 그 경계의 양쪽을 본다.
const (
	tallEnough = 44 // 박스가 서는 창
	tooShort   = 28 // 지금까지의 화면 — 이름표와 관문만
)

// 박스가 섰는지는 위 테두리에 얹힌 이름으로 본다.
const boxMark = "Apple Music CLI"

func homeAt(w, h int, a app.App) string {
	hm := New(a)
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m.View().Content
}

// 박스는 넷째를 말한다 — 무엇에 붙어 있고 무엇을 쥐고 있는가.
func TestHomeBoxShowsMarkNameAndFacts(t *testing.T) {
	out := homeAt(110, tallEnough, stubApp{name: "music"})

	for _, want := range []string{
		"⣿",                            // 마크. 점자 한 조각이면 찍혔다는 뜻이다
		boxMark + " v" + Version,       // 이름과 몇 번째 판인가
		"Sources",                      // 묶음 제목
		"Music.app",                    // 붙어 있는 곳
		"12,481 tracks · 37 playlists", // 쥐고 있는 것
	} {
		if !strings.Contains(out, want) {
			t.Errorf("홈 박스에 %q 가 없다", want)
		}
	}
}

// 자리가 모자라면 박스부터 사라진다. 이름표와 관문이 본론이다.
func TestHomeBoxYieldsToWordmarkAndGate(t *testing.T) {
	out := homeAt(110, tooShort, stubApp{name: "music"})

	if strings.Contains(out, boxMark) {
		t.Error("자리가 없는데 박스가 떠 있다")
	}
	for _, want := range []string{wordmarkMark, homeGate} {
		if !strings.Contains(out, want) {
			t.Errorf("박스를 접었더니 %q 까지 사라졌다", want)
		}
	}
}

// 좁으면 잘라서 세우지 않고 접는다. 잘린 사실은 사실이 아니다.
func TestHomeBoxYieldsToNarrowWindow(t *testing.T) {
	if out := homeAt(56, tallEnough, stubApp{name: "music"}); strings.Contains(out, boxMark) {
		t.Error("폭이 모자라는데 박스가 떠 있다")
	}
}

// 박스가 서도 관문과 그 사유는 그대로 뜬다. 사유를 잃는 것이 제일 나쁘다.
func TestHomeBoxKeepsTheGateReason(t *testing.T) {
	out := homeAt(110, tallEnough, stubApp{name: "music", ready: errors.New("sign in required")})

	for _, want := range []string{boxMark, homeGate, "sign in required"} {
		if !strings.Contains(out, want) {
			t.Errorf("박스가 뜨면서 %q 가 사라졌다", want)
		}
	}
}

// 홈은 셋을 말한다 — 이것이 무엇인가, 어떻게 들어가는가, 지금 무엇이 막혔는가.
//
// 소개 문구는 여기 없다. 박스가 붙어 있는 곳을 목록으로 말하게 된 뒤로
// 같은 것을 더 흐리게 말하는 줄이 됐다(home.go).
func TestHomeShowsNameGateAndReason(t *testing.T) {
	hm := New(stubApp{name: "alpha", ready: errors.New("sign in required")})
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 28})
	out := m.View().Content

	for _, want := range []string{
		wordmarkMark,       // 이름
		homeGate,           // 들어가는 길
		"sign in required", // 막힌 사유
	} {
		if !strings.Contains(out, want) {
			t.Errorf("홈에 %q 가 없다", want)
		}
	}
	// 앱 목록은 걷어냈다. 고를 것이 하나뿐인 목록은 목록이 아니다.
	if strings.Contains(out, "/alpha") {
		t.Error("앱 목록이 아직 홈에 남아 있다")
	}
	if strings.Contains(out, "alpha does things") {
		t.Error("소개 문구가 아직 홈에 남아 있다")
	}
}

// 홈은 스플래시다. 글자는 삼켜진다 — 보이지 않는 입력창에 쌓이면
// 앱에 들어간 순간 친 적 없는 문장이 거기 들어 있다.
func TestTypingIsSwallowedAtHome(t *testing.T) {
	m, _ := key(homeHost(), 'a')

	hm := state(t, m)
	if !hm.home {
		t.Fatal("글자 하나에 홈이 걷혔다")
	}
	if got := hm.input.Value(); got != "" {
		t.Errorf("삼켰어야 할 글자가 입력창에 들어갔다: %q", got)
	}
	if !strings.Contains(m.View().Content, wordmarkMark) {
		t.Error("글자를 쳤다고 홈이 걷혔다")
	}
}

// 홈에서 enter 면 앱으로 들어간다. 고를 목록이 없으므로 갈 곳은 하나다.
func TestEnterOpensTheApp(t *testing.T) {
	m, cmd := homeHost().Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = deliverBatch(t, m, cmd)

	if hm := state(t, m); hm.home {
		t.Fatal("enter 를 눌렀는데 홈에 남아 있다")
	}
}

// 그 하나는 "마지막으로 본 앱"이다. esc 로 나온 자리로 돌아가야 한다.
func TestEnterReturnsToWhereYouLeft(t *testing.T) {
	m, _ := homeHost().Update(switchAppMsg{index: 1})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if !state(t, m).home {
		t.Fatal("esc 를 눌렀는데 홈이 아니다")
	}

	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = deliverBatch(t, m, cmd)

	if got := state(t, m).current; got != 1 {
		t.Errorf("나온 자리가 아니라 %d번 앱으로 들어갔다", got)
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
	var names []string
	for _, c := range state(t, homeHost()).allCommands() {
		names = append(names, c.Name)
	}
	joined := strings.Join(names, " ")
	for _, want := range []string{"/alpha", "/beta", "/help"} {
		if !strings.Contains(joined, want) {
			t.Errorf("홈 팔레트에 %q 가 없다: %v", want, names)
		}
	}
}

// 홈에는 거를 목록이 없다. 모드는 앱 안에서만 뜻이 있으므로 키를 삼킨다 —
// 말해 줄 상태줄도 홈에는 없다(host.go View).
func TestSearchAtHomeDoesNothing(t *testing.T) {
	before := homeHost().View().Content
	m, cmd := homeHost().Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})

	if hm := state(t, m); hm.mode == modeSearch {
		t.Error("홈인데 검색 모드로 들어갔다")
	}
	if cmd != nil {
		t.Error("홈에서 ctrl+f 가 무언가를 시켰다")
	}
	if m.View().Content != before {
		t.Error("홈에서 ctrl+f 에 화면이 달라졌다")
	}
}

// 홈이 받는 키는 관문과 탈출구뿐이다.
func TestHomeHasNoInputNorStatus(t *testing.T) {
	v := homeHost().View()

	if strings.Contains(v.Content, "╭") {
		t.Error("홈에 입력창 테두리가 아직 있다")
	}
	if strings.Contains(v.Content, "alpha") {
		t.Error("홈에 상태줄이 아직 있다")
	}
	if v.Cursor != nil {
		t.Error("그릴 입력창이 없는데 커서 자리를 보고했다")
	}
	if !strings.Contains(v.Content, homeGate) {
		t.Errorf("홈에 관문 %q 가 없다", homeGate)
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

// 세로가 모자라면 이름부터 접는다. 조작법이 이름보다 먼저다 —
// 무엇인지는 몰라도 되지만 어떻게 쓰는지는 알아야 한다.
func TestHomeFoldsWordmarkWhenShort(t *testing.T) {
	hm := New(stubApp{name: "alpha"})
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 12})

	out := m.View().Content
	if strings.Contains(out, wordmarkMark) {
		t.Error("자리가 없는데 이름을 크게 그렸다")
	}
	if !strings.Contains(out, "APPLE MUSIC") {
		t.Error("접었으면 이름은 글자로라도 남아야 한다")
	}
	if !strings.Contains(out, homeGate) {
		t.Error("접었으면 관문은 남아야 한다")
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
