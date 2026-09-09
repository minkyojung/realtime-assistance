package host

import (
	"strings"
	"testing"

	"amcli/tui/internal/app"
	tea "charm.land/bubbletea/v2"
)

// 앱 두 개짜리 경로 — 라우터가 지목한 앱들에게 동시에 묻고,
// 결과가 도착하는 대로 로그에 쌓이는지 본다.
//
// 라우터 자체는 router_test.go 가 실제 모델로 검증한다. 여기서는 그
// 결과(routedMsg)를 직접 흘려보내 API 없이 나머지 경로를 확인한다.

type stubApp struct {
	name  string
	reply string
	badge int
	ready error

	// 앱이 자기 안에 물러날 단계를 갖고 있는 척한다. esc 사슬을 보려면 필요하다.
	back bool

	// 그만두라는 말을 몇 번 들었는지 센다.
	cancelled *int

	// 포커스 신호를 센다. 화면 앞에 있는 앱만 장치를 잡게 하는 통로라
	// 새거나 빠지면 안 보는 동안에도 카메라 불이 켜져 있게 된다.
	focus, blur *int
}

func (s stubApp) Name() string               { return s.name }
func (s stubApp) Description() string        { return s.name + " does things" }
func (s stubApp) Tagline() string            { return s.name + " does things" }
func (s stubApp) Init(func(tea.Msg)) tea.Cmd { return nil }
func (s stubApp) Ready() error               { return s.ready }
func (s stubApp) Badge() int                 { return s.badge }
func (s stubApp) Status() string             { return s.name }
func (s stubApp) Filter(string) app.App      { return s }
func (s stubApp) Back() (app.App, bool)      { return s, s.back }
func (s stubApp) Commands() []app.Command    { return nil }
func (s stubApp) Facts() []app.Fact {
	return []app.Fact{
		{Group: "Sources", Name: "Music.app", Detail: "play · queue · playlists"},
		{Group: "Sources", Name: "Apple Music", Detail: "catalog search · signed in"},
		{Group: "Sources", Name: "LRCLIB", Detail: "synced lyrics"},
		{Group: "Sources", Name: "OpenAI", Detail: "gpt-5.5 · gpt-5.4-mini"},
		{Group: "Library", Detail: "12,481 tracks · 37 playlists"},
		{Group: "Commands", Detail: "/songs /artists /albums /queue /save"},
		{Group: "Commands", Detail: "press / for all 18"},
	}
}
func (s stubApp) View(w, h int) string { return strings.Repeat("\n", h-1) }
func (s stubApp) Ask(string) tea.Cmd {
	return func() tea.Msg { return app.SayMsg{App: s.name, Text: s.reply} }
}
func (s stubApp) Update(msg tea.Msg) (app.App, tea.Cmd) {
	switch m := msg.(type) {
	case app.AskMsg:
		return s, s.Ask(m.Prompt)
	case app.FocusMsg:
		if s.focus != nil {
			*s.focus++
		}
	case app.BlurMsg:
		if s.blur != nil {
			*s.blur++
		}
	case app.CancelMsg:
		if s.cancelled != nil {
			*s.cancelled++
		}
	}
	return s, nil
}

func twoAppHost() (tea.Model, *Model) {
	hm := New(
		stubApp{name: "alpha", reply: "알파가 했습니다"},
		stubApp{name: "beta", reply: "베타가 했습니다", badge: 3},
	)
	hm.LeaveHome()
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 28})
	return m, &hm
}

func TestDeliverAsksEveryNamedApp(t *testing.T) {
	m, _ := twoAppHost()

	m, cmd := m.Update(routedMsg{prompt: "둘 다 해줘", apps: []string{"alpha", "beta"}})
	if cmd == nil {
		t.Fatal("지목된 앱들에게 아무것도 안 보냈다")
	}
	// 두 앱의 답을 모두 흘려보낸다. 순서는 보장하지 않으므로 임의로 준다.
	m, _ = m.Update(app.SayMsg{App: "beta", Text: "베타가 했습니다"})
	m, _ = m.Update(app.SayMsg{App: "alpha", Text: "알파가 했습니다"})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl})

	out := plain(m.View().Content)
	for _, want := range []string{"알파가 했습니다", "베타가 했습니다", "alpha", "beta"} {
		if !strings.Contains(out, want) {
			t.Errorf("로그에 %q 가 없다", want)
		}
	}
}

// 라우터가 못 고르면 지금 보고 있는 앱에게 준다. 틀려도 망하지 않는다.
func TestDeliverFallsBackToCurrentApp(t *testing.T) {
	m, _ := twoAppHost()

	m, _ = m.Update(routedMsg{prompt: "모르겠는 말", apps: nil})
	m, _ = m.Update(app.SayMsg{App: "alpha", Text: "알파가 했습니다"})

	if out := plain(m.View().Content); !strings.Contains(out, "알파가 했습니다") {
		t.Error("아무도 못 골랐는데 지금 앱으로 안 갔다")
	}
}

// 배경 앱은 상태줄에 이름과 배지를 내놓는다. 맥 메뉴바와 같다.
func TestBackgroundAppShowsBadge(t *testing.T) {
	m, _ := twoAppHost()
	if out := plain(m.View().Content); !strings.Contains(out, "beta 3") {
		t.Error("배경 앱의 배지가 상태줄에 없다")
	}
}

// @ 로 지정하면 라우터를 부르지 않는다.
func TestMentionSkipsRouter(t *testing.T) {
	_, hm := twoAppHost()
	name, rest := hm.mention("@beta 이거 해줘")
	if name != "beta" || rest != "이거 해줘" {
		t.Errorf("멘션을 못 떼어냈다: %q / %q", name, rest)
	}
	if name, _ := hm.mention("조용한 거"); name != "" {
		t.Errorf("멘션이 아닌데 %q 를 골랐다", name)
	}
}

// 앱 전환은 팔레트에 있다. 전용 키를 두지 않는 이유는 터미널이
// ctrl+tab 을 tab 과 구별하지 못하기 때문이다 — Terminal.app 은
// kitty 키보드 프로토콜을 지원하지 않는다.
func TestSwitchAppFromPalette(t *testing.T) {
	m, _ := twoAppHost()

	// `/` 를 치면 다른 앱으로 가는 길이 먼저 보인다.
	m, _ = m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	out := plain(m.View().Content)
	if !strings.Contains(out, "/beta") {
		t.Fatal("팔레트에 다른 앱이 없다")
	}
	if !strings.Contains(out, "3 unread") {
		t.Error("배지가 팔레트에 안 보인다")
	}
	if strings.Contains(out, "/alpha") {
		t.Error("지금 보고 있는 앱으로 가는 길이 있다")
	}

	// 골라서 전환한다.
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("전환 Cmd 가 없다")
	}
	m, _ = m.Update(cmd())

	if got := plain(m.View().Content); !strings.Contains(got, "beta") {
		t.Error("전환했는데 상태줄이 안 바뀌었다")
	}
	// 이제 돌아가는 길이 보여야 한다.
	m, _ = m.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	if got := plain(m.View().Content); !strings.Contains(got, "/alpha") {
		t.Error("전환 뒤에 돌아가는 길이 없다")
	}
}

// 전환은 cmd+tab 에 가깝다 — 배경 앱은 계속 살아 있다.
func TestSwitchKeepsOtherAppAlive(t *testing.T) {
	m, _ := twoAppHost()

	// beta 로 옮겨도 alpha 는 여전히 상태줄에 이름을 내놓는다.
	m, _ = m.Update(switchAppMsg{index: 1})
	if got := plain(m.View().Content); !strings.Contains(got, "alpha") {
		t.Error("전환했더니 배경 앱이 사라졌다")
	}
}

// ctrl+o — 가장 최근 응답이 무슨 일을 했는지 펼친다.
//
// 곡별 근거가 사는 유일한 자리다. 본문에 두면 여러 개 중 하나만 보이고
// 나머지는 어차피 안 보인다.
func TestDetailToggle(t *testing.T) {
	m, _ := twoAppHost()

	m, _ = m.Update(app.SayMsg{
		App:  "alpha",
		Text: "두 곡을 골랐어요",
		Detail: []string{
			"candidates 195 → 2 tracks · 8 min",
			"Perth|40일 전에 담고 한 번도 재생 안 함",
			"gpt-5.5 · 5.0s · $0.0106",
		},
	})

	if out := plain(m.View().Content); strings.Contains(out, "candidates 195") {
		t.Error("접혀 있어야 하는데 상세가 보인다")
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})
	out := plain(m.View().Content)
	for _, want := range []string{
		"candidates 195",
		"Perth",
		"40일 전에 담고 한 번도 재생 안 함",
		"$0.0106",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("펼쳤는데 %q 가 없다", want)
		}
	}

	// esc 는 한 단계씩 물러난다.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if out := plain(m.View().Content); strings.Contains(out, "candidates 195") {
		t.Error("esc 를 눌렀는데 상세가 남아 있다")
	}
}

// esc 는 호스트의 키지만 물러나는 순서에는 앱도 끼어 있다.
// 앱이 자기 안에 단계를 갖고 있으면 그것부터다 — 파고든 목록에서
// esc 를 눌렀는데 앱 밖으로 튕겨 나가면 안 된다.
func TestEscAsksTheAppFirst(t *testing.T) {
	hm := New(stubApp{name: "alpha", back: true})
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 28})
	m, _ = m.Update(switchAppMsg{index: 0})

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if hm, _ := m.(Model); hm.home {
		t.Fatal("앱이 물러날 단계를 갖고 있는데 홈으로 나가 버렸다")
	}
}

// 앱이 물러날 곳이 없다고 하면 호스트가 자기 차례를 이어서 밟는다.
func TestEscFallsThroughWhenAppHasNoStep(t *testing.T) {
	hm := New(stubApp{name: "alpha"})
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 28})
	m, _ = m.Update(switchAppMsg{index: 0})

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if hm, _ := m.(Model); !hm.home {
		t.Error("앱이 물러날 곳이 없다는데 홈으로 안 왔다")
	}
}

// esc 사슬의 첫 칸은 "방금 시킨 일"이다.
//
// 선곡이 7초라 그 사이에 잘못 물어본 것을 알아채는데, 지금까지는 기다리는
// 수밖에 없었다. ctrl+c 는 손대지 않는다 — 그것은 터미널의 탈출구다.
func TestEscCancelsThePendingRequest(t *testing.T) {
	var cancelled int
	hm := New(stubApp{name: "alpha", cancelled: &cancelled})
	hm.LeaveHome()
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 28})

	// 라우터를 거치지 않고 곧장 물어본 상태를 만든다.
	m, _ = m.Update(routedMsg{seq: state(t, m).routeSeq, prompt: "조용한 걸로", apps: []string{"alpha"}})
	if len(state(t, m).pending) == 0 {
		t.Fatal("물었는데 기다리는 앱이 없다")
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})

	hs := state(t, m)
	if len(hs.pending) != 0 {
		t.Errorf("esc 를 눌렀는데 아직 %d개를 기다린다", len(hs.pending))
	}
	if cancelled != 1 {
		t.Errorf("앱에게 그만두라고 %d번 말했다 — 한 번이어야 한다", cancelled)
	}
	if hs.home {
		t.Error("요청만 그만둬야 하는데 앱 밖으로 나가 버렸다")
	}
	if len(hs.log) == 0 || hs.log[len(hs.log)-1].who != "host" {
		t.Error("그만뒀다는 말을 로그에 안 남겼다")
	}
}

// 그만둔 뒤 다시 물어보면, 앞 요청의 라우팅 결과가 뒤늦게 와도 무시해야 한다.
func TestCancelledRoutingIsIgnored(t *testing.T) {
	var cancelled int
	hm := New(stubApp{name: "alpha", cancelled: &cancelled})
	hm.LeaveHome()
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 28})

	hs := state(t, m)
	hs.routing = true
	hs.routeSeq = 7
	var mm tea.Model = &hs
	mm, _ = mm.Update(tea.KeyPressMsg{Code: tea.KeyEsc})

	// 그만둔 뒤 도착한 옛 답 — 앱에게 가면 안 된다.
	mm, _ = mm.Update(routedMsg{seq: 7, prompt: "조용한 걸로", apps: []string{"alpha"}})
	if n := len(state(t, mm).pending); n != 0 {
		t.Errorf("그만둔 요청의 라우팅 결과가 앱으로 갔다: %d", n)
	}
}
