package style

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// 화면이 6개로 늘어나도 이 뼈대는 바뀌지 않는다.
// 헤더 · 본문 · 입력창 · 상태줄 중 본문만 갈아끼운다. docs/03 6절.

const MaxContentWidth = 84

func ContentWidth(termWidth int) int {
	w := termWidth - 2
	if w > MaxContentWidth {
		w = MaxContentWidth
	}
	if w < 30 {
		w = 30
	}
	return w
}

// Row 는 왼쪽과 오른쪽을 폭 안에서 양끝으로 벌린다.
// lipgloss.Width 를 쓰므로 한글(폭 2)도 정확히 맞는다.
func Row(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return Truncate(left, width)
	}
	return left + strings.Repeat(" ", gap) + right
}

// Truncate 는 표시폭 기준으로 자른다. 한글 한 글자는 2칸을 먹는다.
func Truncate(sTxt string, width int) string {
	if lipgloss.Width(sTxt) <= width {
		return sTxt
	}
	if width <= 1 {
		return ""
	}
	var b strings.Builder
	for _, r := range sTxt {
		if lipgloss.Width(b.String()+string(r)) > width-1 {
			break
		}
		b.WriteRune(r)
	}
	return b.String() + "…"
}

func Rule(width int) string {
	return RuleStyle.Render(strings.Repeat("─", width))
}

// 헤더 아래에만 쓴다. 화면 상단을 브랜드 색으로 닫는다.
func RuleBrand(width int) string {
	return RuleBrandStyle.Render(strings.Repeat("─", width))
}

// Progress 는 재생 위치 막대다. 머리(●)가 현재 위치를 가리킨다.
func Progress(width int, ratio float64) string {
	if width < 4 {
		return ""
	}
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	filled := int(float64(width-1) * ratio)
	return Brand.Render(strings.Repeat("━", filled)+"●") +
		Faint.Render(strings.Repeat("─", width-1-filled))
}

func MMSS(ms int) string {
	total := ms / 1000
	return fmt.Sprintf("%d:%02d", total/60, total%60)
}

func HumanMinutes(ms int) string {
	return fmt.Sprintf("%d min", ms/60000)
}

// Tokens 는 1240 을 1.2k 로 줄인다. 상태줄이 좁기 때문이다.
func Tokens(n int) string {
	if n < 1000 {
		return fmt.Sprint(n)
	}
	return fmt.Sprintf("%.1fk", float64(n)/1000)
}

// Max · Min · Clamp — 폭 계산에 계속 쓰이는 것들.
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
