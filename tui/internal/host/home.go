package host

import (
	"strings"

	"amcli/tui/internal/app"
	"amcli/tui/internal/style"
	"charm.land/lipgloss/v2"
)

// 홈 — 켜면 여기서 시작하고, esc 로 여기로 돌아온다.
//
// 커튼이 아니라 화면이다. 글자를 쳐도 걷히지 않는다. 걷히는 것은 목적지를
// 골랐을 때뿐이고, 고르는 길은 셋이다 — 목록에서 enter, `/앱이름`,
// 그리고 문장을 쳐서 라우터가 앱을 지목했을 때.
//
// 그래서 이 목록은 진짜 목록이다. ↑↓ 로 고르고 enter 로 들어간다.
// 팔레트와 같은 조작이므로 새로 배울 것이 없다.
//
// 여기서 말하는 것은 둘뿐이다. **여기 뭐가 들어 있는가**와
// **그중 무엇이 지금 안 되는가**. 환영 인사도 버전도 팁도 없다.
//
// 본문 자리를 통째로 쓴다. 팔레트는 보고 있던 목록을 가리면 맥락을
// 잃지만(overlay.go), 홈에는 가릴 맥락이 없다 — 홈이 맥락이다.
//
// 호스트의 두 줄은 그대로 산다. 계약이 "그 위만 앱의 것"이므로
// 홈도 그 위에만 있어야 한다.

// 선글라스. 검은 안경을 검은 배경에 그릴 수 없어 실루엣을 반전시킨다.
//
// 브랜드 색은 쓰지 않는다 — 빨강은 "소리가 나고 있다"는 뜻으로만 쓴다(theme.go).
// 로고에 쓰면 그 규칙이 첫 화면부터 깨진다.
var sunglasses = []string{
	"▄▄▄▄▄▄▄▄▄▄  ▄▄▄▄▄▄▄▄▄▄",
	"██████████▄▄██████████",
	"▝▀██████▀▘  ▝▀██████▀▘",
}

// 이름을 적을 칸. 앱 이름이 길면 그만큼 밀린다.
const homeNameCol = 12

// viewHome 은 본문 자리를 정확히 h 줄로 채운다.
// 앱으로 들어갈 때 화면이 튀지 않아야 한다.
func (m Model) viewHome(w, h int) string {
	rows := make([]string, 0, h)

	// 세로가 모자라면 안경부터 접는다. 앱 목록이 안경보다 먼저다.
	if h >= len(m.apps)+len(sunglasses)+3 {
		rows = append(rows, "")
		for _, g := range sunglasses {
			rows = append(rows, "  "+style.Body.Render(g))
		}
		rows = append(rows, "")
	}
	for i, a := range m.apps {
		rows = append(rows, homeRow(a, i == m.pick, w))
	}
	for len(rows) < h {
		rows = append(rows, "")
	}
	return strings.Join(rows[:h], "\n")
}

// homeRow — 앱 하나를 한 줄로. 고른 줄에는 팔레트와 같은 레일이 선다.
//
// 관문이 막혀 있으면 소개 대신 막힌 사유를 쓴다. 못 하는 것이 먼저다.
//
// 사유는 그릴 때마다 Ready() 를 읽는다. 확정되는 시점이 앱마다 다르기
// 때문이다 — 슬랙은 토큰이 없으면 즉시 알지만, 음악은 첫 폴링이 와야 안다.
// 그래서 멀쩡해 보이다가 잠시 뒤 경고가 붙을 수 있다. 확인 중임을 알리는
// 스피너를 앱마다 돌리는 것보다, 그 깜빡임을 받아들이는 쪽이 조용하다.
func homeRow(a app.App, picked bool, w int) string {
	rail := "  "
	if picked {
		rail = style.Brand.Render("▌ ")
	}
	name := "/" + a.Name()
	col := style.Max(homeNameCol, lipgloss.Width(name)+1)
	left := rail + style.Body.Render(name) +
		strings.Repeat(" ", col-lipgloss.Width(name))
	rest := style.Max(w-2-col, 0)

	if err := a.Ready(); err != nil {
		// 경고는 "! " 다. 앱 본문이 이미 그렇게 그리므로(musicapp/player.go)
		// 여기서 새 기호를 들이면 화면에 경고가 두 종류가 된다.
		return left + style.Warn.Render("! ") +
			style.Dim.Render(style.Truncate(err.Error(), style.Max(rest-2, 0)))
	}
	return left + style.Faint.Render(style.Truncate(a.Tagline(), rest))
}
