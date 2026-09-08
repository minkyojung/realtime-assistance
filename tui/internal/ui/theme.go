package ui

import "charm.land/lipgloss/v2"

// 색은 한 곳에서만 정한다. 화면이 늘어나도 팔레트는 여기 하나다.
var (
	colFg      = lipgloss.Color("#CAD3F5")
	colDim     = lipgloss.Color("#6E738D")
	colFaint   = lipgloss.Color("#494D64")
	colAccent  = lipgloss.Color("#8AADF4") // 강조 · 진행 바
	colNever   = lipgloss.Color("#F5A97F") // 미재생 — 이 서비스의 주제색
	colPlaying = lipgloss.Color("#A6DA95")
	colBorder  = lipgloss.Color("#363A4F")
)

var (
	stTitle   = lipgloss.NewStyle().Bold(true).Foreground(colFg)
	stDim     = lipgloss.NewStyle().Foreground(colDim)
	stFaint   = lipgloss.NewStyle().Foreground(colFaint)
	stAccent  = lipgloss.NewStyle().Foreground(colAccent)
	stNever   = lipgloss.NewStyle().Foreground(colNever)
	stPlaying = lipgloss.NewStyle().Foreground(colPlaying)
	stRule    = lipgloss.NewStyle().Foreground(colBorder)
	stTrack   = lipgloss.NewStyle().Bold(true).Foreground(colFg)
	stMeta    = lipgloss.NewStyle().Foreground(colDim)
)

// 앨범 아트 자리. Bubble Tea 는 셀 기반 렌더러라 진짜 이미지를 띄울 수 없어
// 색 블록으로 대체한다. docs/07 참조.
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
