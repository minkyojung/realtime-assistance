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
}

func (s stubApp) Name() string               { return s.name }
func (s stubApp) Description() string        { return s.name + " does things" }
func (s stubApp) Init(func(tea.Msg)) tea.Cmd { return nil }
func (s stubApp) Ready() error               { return nil }
func (s stubApp) Badge() int                 { return s.badge }
func (s stubApp) Status() string             { return s.name }
func (s stubApp) Filter(string) app.App      { return s }
func (s stubApp) Commands() []app.Command    { return nil }
func (s stubApp) View(w, h int) string       { return strings.Repeat("\n", h-1) }
func (s stubApp) Ask(string) tea.Cmd {
	return func() tea.Msg { return app.SayMsg{App: s.name, Text: s.reply} }
}
func (s stubApp) Update(msg tea.Msg) (app.App, tea.Cmd) {
	if m, ok := msg.(app.AskMsg); ok {
		return s, s.Ask(m.Prompt)
	}
	return s, nil
}

func twoAppHost() (tea.Model, *Model) {
	hm := New(
		stubApp{name: "alpha", reply: "알파가 했습니다"},
		stubApp{name: "beta", reply: "베타가 했습니다", badge: 3},
	)
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
