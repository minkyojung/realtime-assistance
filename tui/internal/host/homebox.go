package host

import (
	"strings"

	"amcli/tui/internal/app"
	"amcli/tui/internal/style"
	"charm.land/lipgloss/v2"
)

// 홈 박스 — 이름표 아래를 통째로 쓴다.
//
// 왼쪽은 마크, 오른쪽은 앱이 대는 사실이다(app.Fact). 위 테두리에 이름과
// 몇 번째 판인지가 얹힌다.
//
// 이 상자를 두는 이유는 인벤토리를 자랑하기 위해서가 아니다. 이 제품의
// 주장은 "네 것만 튼다"인데, 그 말은 진짜 그 사람의 Music.app 에 붙어
// 있을 때만 사실이다. 어깨너머로 보는 사람에게 그것을 증명하는 가장 짧은
// 방법이 붙은 곳을 이름으로 적는 것이다.
//
// 폭은 본문을 다 쓰고, 높이는 내용에 숨 쉴 자리를 얹은 만큼만 쓴다.
// 남는 높이를 전부 먹으면 창이 클수록 상자 안쪽이 텅 빈 벌판이 된다 —
// 넓은 것과 빈 것은 다르다. 남는 자리는 상자 아래에 두고, 관문이 그
// 한가운데에 선다(home.go).
//
// 테두리를 lipgloss 에 맡기지 않고 직접 긋는다. 위 테두리에 글자를 얹는
// 방법이 없어서다.

// 왼쪽 칸 — 마크와 그 아래 여백. 마크(14칸)보다 넉넉해야 오른쪽 칸이
// 마크에 붙지 않는다.
const homeBoxLeftCol = 22

// 사실의 이름 칸. 이름이 세로로 맞아야 눈이 한 번만 움직인다.
const homeBoxNameCol = 14

// 사실 줄은 묶음 제목보다 두 칸 들어간다.
const homeBoxFactIndent = 2

// 설명에 이만큼도 못 주면 박스를 접는다. 잘린 사실은 사실이 아니다.
const homeBoxMinDetail = 24

// 안여백 — 내용과 테두리 사이.
//
// 테두리에 글자가 붙으면 상자가 꽉 찬 것이 아니라 좁아 보인다. 가로를
// 세로보다 넉넉히 두는 이유는 칸이 세로로 길기 때문이다 — 한 칸은 폭 1에
// 높이 2쯤이라, 같은 여백으로 보이려면 가로를 두 배 가까이 줘야 한다.
const (
	homeBoxPad     = 3 // 위아래 빈 줄
	homeBoxSidePad = 4 // 좌우 빈 칸
)

// 내용이 이만큼도 못 들어가면 접는다 — 마크 여섯 줄에 사실 몇 줄은 있어야
// 상자라고 부를 만하다.
const homeBoxMinRows = 10

// homeBoxRowCount 는 사실들이 몇 줄을 먹는지 센다. 묶음이 바뀔 때마다
// 제목 한 줄이 서고, 두 번째 묶음부터는 그 앞에 빈 줄이 하나 더 붙는다.
// 마크보다 짧으면 마크에 맞춘다 — 마크는 잘리면 안 된다.
func homeBoxRowCount(facts []app.Fact) int {
	n, group := 0, ""
	for _, f := range facts {
		if f.Group != group {
			if group != "" {
				n++
			}
			group = f.Group
			n++
		}
		n++
	}
	return style.Max(n, len(markRows))
}

// homeBoxRows 는 박스를 정확히 h 줄로 그린다. 자리가 모자라면 nil 이다.
//
// 높이를 밖에서 정해 넘긴다. 남는 자리를 다 쓰는 것이 이 상자의 규칙인데,
// 남는 자리를 아는 것은 viewHome 이기 때문이다.
func (m Model) homeBoxRows(w, h int) []string {
	facts := m.app().Facts()
	if len(facts) == 0 {
		return nil
	}
	// 내용이 몇 줄인지 먼저 세고, 거기에 위아래 숨 쉴 자리를 얹는다.
	// 남는 자리가 그보다 적으면 그만큼만 쓴다.
	h = style.Min(h, homeBoxRowCount(facts)+2*homeBoxPad+2)
	if h < homeBoxMinRows+2 {
		return nil
	}
	// 테두리 두 칸과 좌우 안여백은 글자가 못 쓴다. 바깥 폭은 본문 그대로라
	// 입력창과 테두리가 양쪽 모두 정확히 같은 칸에 선다(host.go inputBox) —
	// 여백을 넓혀도 줄어드는 것은 안쪽뿐이다.
	inner := w - 2 - 2*homeBoxSidePad
	if inner < homeBoxLeftCol+homeBoxFactIndent+homeBoxNameCol+homeBoxMinDetail {
		return nil
	}

	lines := homeBoxBody(facts, inner, h-2)

	side := strings.Repeat(" ", homeBoxSidePad)
	out := make([]string, 0, h)
	out = append(out, homeBoxTop(inner+2*homeBoxSidePad))
	for _, l := range lines {
		out = append(out, style.RuleStyle.Render("│")+side+
			l+strings.Repeat(" ", style.Max(inner-lipgloss.Width(l), 0))+
			side+style.RuleStyle.Render("│"))
	}
	return append(out, style.RuleStyle.Render(
		"╰"+strings.Repeat("─", inner+2*homeBoxSidePad)+"╯"))
}

// 위 테두리에 이름과 판을 얹는다. 오른쪽에 붙이는 이유는 왼쪽 위가 이미
// 이름표의 것이기 때문이다 — 같은 모서리에서 이름을 두 번 말하지 않는다.
func homeBoxTop(width int) string {
	title := "Apple Music CLI " + buildStamp()
	// ╭ 과 ╮, 제목 양옆의 "─ " 와 " ─" 로 여섯 칸이 이미 나간다.
	rule := width - lipgloss.Width(title) - 4
	if rule < 1 {
		return style.RuleStyle.Render("╭" + strings.Repeat("─", width) + "╮")
	}
	return style.RuleStyle.Render("╭"+strings.Repeat("─", rule)+"─ ") +
		style.Dim.Render(title) +
		style.RuleStyle.Render(" ─╮")
}

// homeBoxBody 는 안쪽을 정확히 rows 줄로 채운다.
//
// 관문 사유는 여기 넣지 않는다. 그것은 박스 아래 관문 줄 옆에 선다
// (home.go) — 이 상자가 답하는 질문은 "어디에 붙어 있는가"이고,
// 사유가 답하는 것은 "그래서 지금 무엇이 막혀 있는가"다.
func homeBoxBody(facts []app.Fact, inner, rows int) []string {
	out := make([]string, 0, rows)
	for i := 0; i < homeBoxPad; i++ {
		out = append(out, "")
	}

	group := ""
	for _, f := range facts {
		if f.Group != group {
			if group != "" {
				out = append(out, "")
			}
			group = f.Group
			out = append(out, homeBoxIndentTo(homeBoxLeftCol)+style.Body.Render(f.Group))
		}
		out = append(out, homeBoxFactLine(f, inner))
	}

	// 마크는 왼쪽 칸에 겹쳐 넣는다. 사실이 마크보다 짧으면 줄을 먼저
	// 늘린다 — 사실이 몇 줄이든 마크는 통째로 서야 한다.
	for len(out) < len(markRows)+homeBoxPad {
		out = append(out, "")
	}
	for i, r := range markRows {
		row := i + homeBoxPad // 위 여백만큼 내려서 선다
		out[row] = style.Brand.Render(r) +
			strings.Repeat(" ", style.Max(homeBoxLeftCol-markWidth, 1)) +
			strings.TrimPrefix(out[row], homeBoxIndentTo(homeBoxLeftCol))
	}

	for len(out) < rows {
		out = append(out, "")
	}
	return out[:rows]
}

func homeBoxFactLine(f app.Fact, inner int) string {
	left := homeBoxIndentTo(homeBoxLeftCol + homeBoxFactIndent)
	detail := inner - homeBoxLeftCol - homeBoxFactIndent
	if f.Name != "" {
		name := style.Truncate(f.Name, homeBoxNameCol-1)
		left += style.Dim.Render(name) +
			strings.Repeat(" ", style.Max(homeBoxNameCol-lipgloss.Width(name), 1))
		detail -= homeBoxNameCol
	}
	return left + style.Faint.Render(style.Truncate(f.Detail, style.Max(detail, 0)))
}

func homeBoxIndentTo(n int) string { return strings.Repeat(" ", n) }
