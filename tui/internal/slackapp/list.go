package slackapp

import (
	"fmt"
	"strings"

	"amcli/tui/internal/style"
	"charm.land/lipgloss/v2"
)

// 대화 목록. 채널 사이드바를 두지 않는 이유는 음악이 사이드바를 버린 것과
// 같다 — 갈 수 있는 곳은 상시로 볼 필요가 없고, `/` 와 `ctrl+f` 가 대신한다.

// 칸 선언. 좁아지면 시각 → 마지막 메시지 순으로 사라지고,
// 남는 공간은 마지막 메시지가 가져간다.
//
// **대화 이름은 절대 안 버린다.** 이름이 없으면 그 줄은 아무 뜻이 없다.
var convCols = []style.Col{
	{Min: 12, Weight: 1, Max: 24}, // 대화 이름 — Drop 0
	{Min: 16, Weight: 3, Drop: 1}, // 마지막 메시지
	{Min: 5, Drop: 2},             // 시각 — 제일 먼저 버린다
}

const colGap = 2

// 목록 줄 수의 상한. 음악과 같은 값이다 — 화면을 꽉 채우면 읽을 것이 아니라
// 스캔할 것이 되어버린다.
const maxListRows = 14

// rows 는 지금 보여줄 대화들이다. 검색어와 /unread 가 여기서만 걸린다.
func (m Model) rows() []Conversation {
	q := strings.ToLower(strings.TrimSpace(m.filter))
	out := make([]Conversation, 0, len(m.convs))
	for _, c := range m.convs {
		if m.onlyUnread && c.Unread == 0 {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(c.Name), q) &&
			!strings.Contains(strings.ToLower(c.Last), q) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func (m Model) rowCount() int { return len(m.rows()) }

// 본문 세로 예산: 머리1 + 룰1 + 목록 + 룰1 + 안내(0|1)
func (m Model) listHeight(h int) int {
	reserved := 3
	if m.hint() != "" {
		reserved++
	}
	return style.Min(style.Max(h-reserved, 3), maxListRows)
}

func (m Model) viewList(w, h int) string {
	rows := m.rows()
	if len(rows) == 0 {
		msg := "아직 대화를 읽는 중입니다"
		switch {
		case m.loadErr != nil:
			msg = "대화를 읽지 못했습니다"
		case !m.loaded:
		case strings.TrimSpace(m.filter) != "":
			msg = "No matches"
		case m.onlyUnread:
			msg = "안 읽은 것이 없습니다"
		default:
			msg = "대화가 없습니다"
		}
		return lipgloss.NewStyle().Width(w).Height(h).Render(style.Faint.Render(msg))
	}

	start := style.Clamp(m.top, 0, style.Max(len(rows)-h, 0))
	end := style.Min(start+h, len(rows))
	widths := style.Columns(w-2, colGap, convCols)

	var b strings.Builder
	for i := start; i < end; i++ {
		b.WriteString(m.renderRow(rows[i], i == m.sel, widths))
		if i < end-1 {
			b.WriteString("\n")
		}
	}
	out := b.String()
	for n := end - start; n < h; n++ {
		out += "\n"
	}
	return out
}

func (m Model) renderRow(c Conversation, selected bool, widths []int) string {
	// 선택 표시는 왼쪽 레일이다. 배경을 채우면 "채워진 빨강은 오류" 규칙과 부딪힌다.
	rail := "  "
	if selected {
		rail = style.Brand.Render("▌ ")
	}

	// 안 읽음은 이름 뒤에 붙는 숫자다. 별도 칸을 주면 대개 비어 있다.
	name := c.Name
	if c.Unread > 0 {
		name = fmt.Sprintf("%s %d", c.Name, c.Unread)
	}
	cells := []string{}

	nameCell := pad(name, widths[0])
	if c.Unread > 0 {
		nameCell = style.BrandBold.Render(nameCell)
	} else {
		nameCell = style.Body.Render(nameCell)
	}
	cells = append(cells, nameCell)

	if widths[1] > 0 {
		last := c.Last
		if by := m.users[c.LastBy]; by != "" && !c.IsIM {
			last = by + ": " + last
		}
		cells = append(cells, style.Faint.Render(pad(oneLine(last), widths[1])))
	}
	if widths[2] > 0 {
		at := ""
		if !c.At.IsZero() {
			at = c.At.Format("15:04")
		}
		cells = append(cells, style.Faint.Render(rpad(at, widths[2])))
	}
	return rail + strings.Join(cells, strings.Repeat(" ", colGap))
}

// 슬랙 메시지에는 줄바꿈이 흔하다. 한 줄짜리 칸에 그대로 넣으면 표가 깨진다.
func oneLine(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
}

func pad(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = style.Truncate(s, width)
	if d := width - lipgloss.Width(s); d > 0 {
		s += strings.Repeat(" ", d)
	}
	return s
}

func rpad(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = style.Truncate(s, width)
	if d := width - lipgloss.Width(s); d > 0 {
		s = strings.Repeat(" ", d) + s
	}
	return s
}
