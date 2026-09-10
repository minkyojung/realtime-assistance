package main

import (
	"fmt"
	"os"
	"strings"

	"amcli/tui/internal/app"
	"amcli/tui/internal/data"
	"amcli/tui/internal/data/fixture"
	"amcli/tui/internal/host"
	"amcli/tui/internal/musicapp"
	tea "charm.land/bubbletea/v2"
)

// 리팩터링 전후를 비교할 골든 파일을 만든다.
//
// **아래 시나리오 목록은 internal/host/golden_test.go 와 같아야 한다.**
// 실제 Music.app 상태에 의존하지 않도록 폴링 결과는 넣지 않는다.
// 골든은 AI 가 **켜진** 화면이다.
//
// 시작 모드가 키 유무로 갈린다(host.New). 키체인을 보는 값이라 기계마다
// 다르고, 그러면 "기계와 무관해야 한다"는 이 파일의 전제가 깨진다 — 키를
// 가진 사람과 아닌 사람이 서로 다른 골든을 떠 왔다.
//
// 환경 변수가 키체인보다 먼저다(secrets.OpenAIKey). 가짜를 하나 심어 켜진
// 쪽으로 고정한다. 그리기만 하므로 이 값으로 어디에도 붙지 않는다.
func fixKeyState() { os.Setenv("OPENAI_API_KEY", "golden-fixture") }

func main() {
	// 골든은 기계·시각과 무관해야 한다. 픽스처를 고정으로 심는다.
	data.Set(fixture.Lib())
	fixKeyState()

	var b strings.Builder
	for _, c := range []struct {
		name, keys string
		msgs       []tea.Msg // 키로는 못 만드는 상태. 키 다음에 넣는다
	}{
		{"home", "", nil},
		{"default", "", nil},
		{"search", "\x0Eoasis", nil}, // shift+tab + oasis
		{"commands", "/", nil},
		{"help", "?", nil},
		{"prompt", "something quiet", nil},
		// 묶음 파고들기 — tab 으로 Artists 에 가서 enter.
		{"drill", "\t\n", nil},
		// 대화 띠 — 묻고, 앱이 답한 화면. 내 말은 바탕색, 답은 접혀서.
		{"talk", "something quiet\n", []tea.Msg{app.SayMsg{App: "music",
			Text: "A quiet hour from what you own — five tracks you have not played in months, " +
				"starting soft and staying there."}}},
		// 가려진 입력 — 키를 치는 동안 화면에 점만 보인다.
		{"secret", "/ai\nsk-golden-0123456789", nil},
	} {
		hm := host.New(musicapp.New())
		if c.name != "home" {
			hm.LeaveHome()
		}
		var m tea.Model = &hm
		m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 28})
		for _, r := range c.keys {
			switch r {
			case '\x0E':
				m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
			case '\t':
				m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
			case '\n':
				m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			default:
				m, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
			}
		}
		for _, msg := range c.msgs {
			m, _ = m.Update(msg)
		}
		fmt.Fprintf(&b, "=== %s ===\n%s\n", c.name, m.View().Content)
	}
	os.WriteFile("internal/host/testdata/golden.txt", []byte(b.String()), 0o644)
	fmt.Println("wrote", len(b.String()), "bytes")
}
