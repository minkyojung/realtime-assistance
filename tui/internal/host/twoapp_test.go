package host

import (
	"strings"
	"testing"

	"amcli/tui/internal/app"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

	// 이 앱이 쓴 것. 상태줄 오른쪽 끝에 앉는다.
	spend string

	// 포커스 신호를 센다. 화면 앞에 있는 앱만 장치를 잡게 하는 통로라
	// 새거나 빠지면 안 보는 동안에도 카메라 불이 켜져 있게 된다.
	focus, blur *int
}

func (s stubApp) Name() string               { return s.name }
func (s stubApp) Description() string        { return s.name + " does things" }
func (s stubApp) Tagline() string            { return s.name + " does things" }
func (s stubApp) Init(func(tea.Msg)) tea.Cmd { return nil }
func (s stubApp) Ready() error               { return s.ready }
func (s stubApp) Spend() string              { return s.spend }
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

	// 기본이 펼침이다. 근거를 보려고 키를 눌러야 했던 시절은 지났다 —
	// 목록 상한을 걷어내면서 대화 띠에 자리가 생겼다.
	out := plain(m.View().Content)
	for _, want := range []string{
		"두 곡을 골랐어요",
		"candidates 195",
		"Perth",
		"40일 전에 담고 한 번도 재생 안 함",
		"$0.0106",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("대화 띠에 %q 가 없다", want)
		}
	}

	// ctrl+j 로 접으면 마지막 한 줄만 남는다. 목록을 더 볼 때다.
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl})
	shut := plain(m.View().Content)
	if strings.Contains(shut, "candidates 195") {
		t.Error("접었는데 근거가 남아 있다")
	}
	if !strings.Contains(shut, "두 곡을 골랐어요") {
		t.Error("접었는데 마지막 한 줄까지 사라졌다")
	}

	// 다시 누르면 펼쳐진다.
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl})
	if !strings.Contains(plain(m.View().Content), "candidates 195") {
		t.Error("다시 눌렀는데 안 펼쳐졌다")
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

// 비용은 좁아져도 살아남는다.
//
// 상태줄이 한 줄이라 좁아지면 왼쪽의 끝부터 잘리는데, 비용이 그 끝에 있었다.
// 쓴 돈이 안 보이는 것은 안 쓴 것처럼 보이는 것과 같다.
func TestSpendSurvivesANarrowStatusLine(t *testing.T) {
	hm := New(stubApp{name: "alpha", spend: "$0.0031"})
	hm.LeaveHome()
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 28})

	hs := state(t, m)
	for _, w := range []int{40, 60, 90} {
		if got := plain(hs.viewStatus(w)); !strings.Contains(got, "$0.0031") {
			t.Errorf("폭 %d 에서 비용이 사라졌다: %q", w, got)
		}
	}
}

// 없는 명령은 지나가는 사건이다. 상태줄이 아니라 로그로 간다 —
// 상태줄은 "지금 어디인가"를 말하는 자리이고, 알림이 그것을 덮으면
// 어디를 보고 있는지가 사라진다.
func TestNoSuchCommandGoesToTheLog(t *testing.T) {
	hm := New(stubApp{name: "alpha"})
	hm.LeaveHome()
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 28})

	for _, r := range "/nope" {
		m, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	hs := state(t, m)
	if len(hs.log) == 0 {
		t.Fatal("없는 명령을 쳤는데 로그에 아무것도 없다")
	}
	if last := hs.log[len(hs.log)-1]; last.who != "host" || !strings.Contains(last.text, "No such command") {
		t.Errorf("로그에 안 남았다: %+v", last)
	}
	if got := plain(hs.viewStatus(90)); !strings.Contains(got, "alpha") {
		t.Errorf("알림이 상태줄을 덮었다: %q", got)
	}
}

// 긴 답은 잘리지 않고 접힌다. 답이 잘리면 무슨 말인지 모른다.
func TestLongAnswerWraps(t *testing.T) {
	m, _ := twoAppHost()
	long := "최근에 거의 안 들은 조용한 트랙을 중심으로 25분에 맞췄어요. " +
		"담아두고 한 번도 재생하지 않은 곡을 앞에 두고, 같은 아티스트가 " +
		"연달아 나오지 않게 사이를 벌렸습니다."
	m, _ = m.Update(app.SayMsg{App: "alpha", Text: long})

	out := plain(m.View().Content)
	// 답이 실린 줄만 본다. 화면 전체를 보면 다른 자리의 말줄임(placeholder 등)에
	// 걸려 오탐이 난다.
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "최근에") || strings.Contains(l, "담아두고") ||
			strings.Contains(l, "사이를 벌렸") {
			if strings.Contains(l, "…") {
				t.Errorf("답이 잘렸다 — 접혀야 한다: %q", strings.TrimSpace(l))
			}
		}
	}
	// 끝 문장이 화면에 있어야 한다.
	if !strings.Contains(out, "사이를 벌렸습니다") {
		t.Error("답의 끝이 화면에 없다")
	}
	// 두 줄 이상으로 접혔는가.
	var rows int
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "않은 곡을 앞에") || strings.Contains(l, "최근에 거의") {
			rows++
		}
	}
	if rows < 2 {
		t.Errorf("한 줄에 다 넣으려 했다 (%d줄)", rows)
	}
}

// 내가 한 말에만 바탕색이 깔리고, **줄 끝까지** 이어져야 한다.
//
// 조각마다 색을 주지 않고 밖에서 한 번 감싸면, 조각 안쪽의 리셋이
// 바탕색까지 꺼서 두세 칸 만에 색이 사라진다. 실제로 그렇게 났었다.
func TestOnlyMyWordsAreFilled(t *testing.T) {
	m, _ := twoAppHost()
	long := "조용한 거 25분치로 골라줘, 담아두고 한 번도 안 들은 곡 위주로 " +
		"부탁하고 같은 아티스트가 연달아 나오지 않게 사이도 좀 벌려줘"
	m = typeText(m, long)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m, _ = m.Update(app.SayMsg{App: "alpha", Text: "골랐어요", Detail: []string{"195 → 6 tracks"}})

	var mineRows int
	for _, l := range strings.Split(m.View().Content, "\n") {
		p := plain(l)
		switch {
		case strings.Contains(p, "조용한 거 25분치") || strings.Contains(p, "사이도 좀"):
			mineRows++
			if !strings.Contains(l, "48;2;") {
				t.Errorf("내 말에 바탕색이 없다: %q", strings.TrimSpace(p))
			}
			// 색이 줄 끝까지 가야 한다. 칠해진 칸 수를 실제로 세어 본다.
			// 프레임 좌우 여백 한 칸씩은 호스트 바깥이라 안 칠해지는 것이 맞다.
			if painted, total := paintedWidth(l); painted < total-2*framePad {
				t.Errorf("바탕색이 %d/%d 칸에서 끊겼다: %q", painted, total, strings.TrimSpace(p))
			}
		case strings.Contains(p, "골랐어요"), strings.Contains(p, "195 → 6 tracks"):
			if strings.Contains(l, "48;2;") {
				t.Errorf("답·근거에 바탕색이 깔렸다: %q", strings.TrimSpace(p))
			}
		}
	}
	if mineRows < 2 {
		t.Errorf("내 말이 %d줄 — 접혀서 두 줄 이상이어야 한다", mineRows)
	}
}

// paintedWidth — 그 줄에서 바탕색이 켜진 채로 그려진 칸 수와 전체 칸 수.
//
// 리셋(\x1b[m)은 글자색만이 아니라 바탕색도 끈다. 조각마다 색을 다시
// 주지 않으면 첫 리셋에서 끊기는데, 눈으로는 "안 들어갔다"로 보인다.
// 그 착시를 테스트가 대신 본다.
func paintedWidth(l string) (painted, total int) {
	on := false
	for i := 0; i < len(l); {
		if l[i] == 0x1b {
			j := strings.IndexByte(l[i:], 'm')
			if j < 0 {
				break
			}
			seq := l[i : i+j+1]
			switch {
			case strings.Contains(seq, "48;2;"):
				on = true
			case seq == "\x1b[m" || seq == "\x1b[0m":
				on = false
			}
			i += j + 1
			continue
		}
		r := []rune(l[i:])[0]
		wid := lipgloss.Width(string(r))
		total += wid
		if on {
			painted += wid
		}
		i += len(string(r))
	}
	return painted, total
}

// 대화 띠는 세 군데가 비어 있다 — 위, 내 말과 답 사이, 아래.
//
// 목록에 붙으면 내 말도 목록으로 읽히고, 답에 붙으면 색이 다르다는 것만으로는
// 한 마디가 끝난 자리를 못 찾고, 입력창 테두리에 붙으면 테두리가 근거의
// 밑줄로 보인다. 근거는 답의 일부이므로 답에 붙인다.
func TestLogBandBreathes(t *testing.T) {
	m, _ := twoAppHost()
	m = typeText(m, "hi")
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m, _ = m.Update(app.SayMsg{App: "alpha", Text: "골랐어요", Detail: []string{"195 → 6"}})

	lines := strings.Split(m.View().Content, "\n")
	at := func(want string) int {
		for i, l := range lines {
			if strings.Contains(plain(l), want) {
				return i
			}
		}
		return -1
	}
	blank := func(i int) bool {
		return i >= 0 && i < len(lines) && strings.TrimSpace(plain(lines[i])) == ""
	}

	mine, theirs := at("› hi"), at("골랐어요")
	if mine < 0 || theirs < 0 {
		t.Fatal("대화가 화면에 없다")
	}
	if !blank(mine - 1) {
		t.Error("내 말 위가 안 비었다 — 목록에 붙는다")
	}
	if theirs != mine+2 || !blank(mine+1) {
		t.Errorf("내 말과 답 사이가 안 비었다 (%d줄 차이)", theirs-mine)
	}
	if why := at("195 → 6"); why != theirs+1 {
		t.Error("근거가 답에서 떨어졌다 — 근거는 답의 일부다")
	}
	if box := at("╭"); box < 0 || !blank(box-1) {
		t.Error("대화와 입력창 사이가 안 비었다")
	}
}

// 스피너는 답이 앉을 자리에 그대로 앉는다.
//
// 기다릴 때와 답이 왔을 때 줄 자리가 달라지면 답이 도착하는 순간 화면이
// 한 번 튄다. 7초를 기다린 끝에 튀는 것은 그 자체로 실패로 보인다.
func TestSpinnerSitsWhereAnswerWill(t *testing.T) {
	m, _ := twoAppHost()
	m = typeText(m, "hi")
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	rowOf := func(mm tea.Model, want string) int {
		for i, l := range strings.Split(mm.View().Content, "\n") {
			if strings.Contains(plain(l), want) {
				return i
			}
		}
		return -1
	}
	spin := rowOf(m, "asking")
	if spin < 0 {
		spin = rowOf(m, "where this goes")
	}
	if d := spin - rowOf(m, "› hi"); d != 2 {
		t.Fatalf("스피너가 내 말에서 %d줄 아래다 — 안여백 다음이어야 한다", d)
	}

	m, _ = m.Update(app.SayMsg{App: "alpha", Text: "골랐어요"})
	m, _ = m.Update(app.SayMsg{App: "beta", Text: "베타도 했어요"})
	if got := rowOf(m, "골랐어요") - rowOf(m, "› hi"); got != 2 {
		t.Errorf("답이 스피너 자리에 안 앉았다 (내 말에서 %d줄 아래)", got)
	}
}

// 내 말은 화면 왼쪽 끝에 딱 붙지 않는다. 바탕색 덩어리가 벽에 눌려 보인다.
func TestMyWordsHaveLeftPadding(t *testing.T) {
	m, _ := twoAppHost()
	m = typeText(m, "hi")
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	for _, l := range strings.Split(m.View().Content, "\n") {
		p := plain(l)
		if !strings.Contains(p, "› hi") {
			continue
		}
		// 프레임 여백 한 칸 + 안여백 한 칸 = 기호는 세 번째 칸부터.
		if i := strings.Index(p, "›"); i != framePad+1 {
			t.Errorf("기호가 %d번째 칸에 있다 — %d번째여야 한다", i, framePad+1)
		}
		return
	}
	t.Fatal("내 말이 화면에 없다")
}
