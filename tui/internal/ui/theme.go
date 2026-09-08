package ui

import "charm.land/lipgloss/v2"

// 팔레트는 여기 하나다. 화면이 6개로 늘어나도 색은 여기서만 정한다.
//
// 규칙: 빨강은 "소리가 나고 있다"는 뜻으로만 쓴다. 침묵은 회색이다.
//
//	brand  재생 중 · 진행 바 · 프롬프트 · 되살리기
//	dim    미재생 곡 — 색이 없다는 것 자체가 죽어 있다는 뜻
//	warn   경고
//
// 터미널에서 빨강은 보통 에러를 뜻하므로, 에러는 색이 아니라
// 채워진 배지로 구분한다.
var (
	colBrand     = lipgloss.Color("#FA002C") // Apple Music red
	colBrandSoft = lipgloss.Color("#FF5A75") // 본문에 얹을 때 읽기 좋은 톤
	colBrandDeep = lipgloss.Color("#7A0A1E")

	colFg    = lipgloss.Color("#E8E8EA")
	colDim   = lipgloss.Color("#8A8A92")
	colFaint = lipgloss.Color("#55555E")
	colRule  = lipgloss.Color("#2E2E36")
	colWarn  = lipgloss.Color("#E8A33D")
)

var (
	stTitle = lipgloss.NewStyle().Bold(true).Foreground(colFg)
	stTrack = lipgloss.NewStyle().Bold(true).Foreground(colFg)
	stBody  = lipgloss.NewStyle().Foreground(colFg)
	stMeta  = lipgloss.NewStyle().Foreground(colDim)
	stDim   = lipgloss.NewStyle().Foreground(colDim)
	stFaint = lipgloss.NewStyle().Foreground(colFaint)
	stRule  = lipgloss.NewStyle().Foreground(colRule)
	stRuleBrand = lipgloss.NewStyle().Foreground(colBrandDeep)

	stBrand     = lipgloss.NewStyle().Foreground(colBrand)
	stBrandBold = lipgloss.NewStyle().Bold(true).Foreground(colBrand)
	stBrandSoft = lipgloss.NewStyle().Foreground(colBrandSoft)
	stTrackHead = lipgloss.NewStyle().Foreground(colBrandDeep)
	stWarn      = lipgloss.NewStyle().Foreground(colWarn)

	// 에러는 색이 아니라 배지로 구분한다.
	stErrorBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colBrand).
			Bold(true).
			Padding(0, 1)
)

// 앨범 아트 자리. Bubble Tea 는 셀 기반 렌더러라 진짜 이미지를 띄울 수 없어
// 색 블록으로 대체한다.
//
// 여기는 브랜드 색을 크게 써도 되는 유일한 자리다. 원래 앨범 커버가 놓이는
// 곳이라 색이 있어도 경보로 읽히지 않는다.
var artShades = []string{"#5C0014", "#8C001F", "#C4002A", "#FA002C", "#C4002A", "#8C001F"}

// 묘비 화면은 무채색을 쓴다. 되살릴 때 색이 돌아오는 대비가 이 제품의 핵심이다.
var artShadesDead = []string{"#2E2E36", "#3A3A42", "#44444E", "#3A3A42", "#2E2E36", "#26262C"}

func albumArt(shades []string, w, h int) string {
	out := ""
	for r := 0; r < h; r++ {
		c := lipgloss.Color(shades[r%len(shades)])
		out += lipgloss.NewStyle().Background(c).Width(w).Render("")
		if r < h-1 {
			out += "\n"
		}
	}
	return out
}
