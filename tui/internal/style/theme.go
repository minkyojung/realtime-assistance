package style

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// tint 는 배경 위에 색을 a 만큼 옅게 얹은 결과다.
//
// 터미널에는 투명도가 없다. 알파를 넘길 수 없으므로 미리 섞어 불투명한 한
// 색으로 만든다. 계산해 두면 브랜드 색이 바뀔 때 얹은 색도 따라 바뀐다 —
// 16진수로 박아 두면 그때부터 둘이 조용히 어긋난다.
func tint(bg, fg color.Color, a float64) color.Color {
	br, bgn, bb, _ := bg.RGBA()
	fr, fgn, fb, _ := fg.RGBA()
	mix := func(b, f uint32) uint8 {
		// RGBA() 는 16비트로 돌려준다. 8비트로 내려서 섞는다.
		return uint8(float64(b>>8)*(1-a) + float64(f>>8)*a)
	}
	return color.RGBA{R: mix(br, fr), G: mix(bgn, fgn), B: mix(bb, fb), A: 0xFF}
}

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
	ColBrand       = lipgloss.Color("#FF5A75") // Apple Music red
	ColBrandSoft   = lipgloss.Color("#FF8FA3") // 본문에 얹을 때 읽기 좋은 톤
	ColBrandDeep   = lipgloss.Color("#A8253F")
	ColBrandShadow = lipgloss.Color("#7A2036") // 워드마크 뒤에 선 그림자 글자

	ColFg    = lipgloss.Color("#E8E8EA")
	ColDim   = lipgloss.Color("#8A8A92")
	ColFaint = lipgloss.Color("#55555E")
	ColRule  = lipgloss.Color("#2E2E36")
	ColWarn  = lipgloss.Color("#E8A33D")

	// 터미널 배경. 실제 화면을 픽셀로 재서 잡은 값이다.
	//
	// 우리가 칠하는 색이 아니라 **뒤에 비치는 색**이다. 투명도를 미리 섞어
	// 두려면 무엇 위에 얹는지를 알아야 해서 여기 적어 둔다.
	ColBase = lipgloss.Color("#1C1B27")

	// 선택된 줄의 배경 — 브랜드 색을 옅게 얹은 것.
	//
	// 터미널에는 투명도가 없다. 그래서 배경 위에 14%만큼 미리 섞어 불투명한
	// 한 색으로 만든다. 브랜드 색을 그대로 채우지 않는 이유는, 이 화면에서
	// **채워진 빨강이 "오류"라는 뜻**이기 때문이다(ErrorBadge). 옅게 얹은
	// 것은 그 규칙과 부딪히지 않는다 — 다른 것은 색조가 아니라 진하기다.
	//
	// 14% 는 눈대중이 아니다. 이미 화면에 있는 띠(ColSaidByMe)가 배경보다
	// 채널당 평균 17 밝은데, 그와 같은 무게가 되는 값이다.
	ColRowSel = tint(ColBase, ColBrand, 0.14)

	// 대화 띠에서 **내가 친 문장**의 바탕. 답에는 깔지 않는다.
	//
	// 둘 다 칠하면 이번에는 둘이 구별되지 않는다. 하나만 칠하면 나머지는
	// 저절로 갈린다. 칠하는 쪽을 내 말로 정한 것은 답이 길고 내 말이 짧기
	// 때문이다 — 긴 쪽을 칠하면 화면의 절반이 색면이 된다.
	//
	// 어둡되 보여야 한다. 처음에 #23232E 로 뒀더니 터미널 배경(#1C1B27)과
	// 채널당 7 차이라 제대로 칠해도 안 보였다. 실제 화면을 픽셀로 재서
	// 잡은 값이다.
	ColSaidByMe = lipgloss.Color("#2C2C3A")
)

var (
	Title          = lipgloss.NewStyle().Bold(true).Foreground(ColFg)
	Track          = lipgloss.NewStyle().Bold(true).Foreground(ColFg)
	Body           = lipgloss.NewStyle().Foreground(ColFg)
	Meta           = lipgloss.NewStyle().Foreground(ColDim)
	Dim            = lipgloss.NewStyle().Foreground(ColDim)
	Faint          = lipgloss.NewStyle().Foreground(ColFaint)
	RuleStyle      = lipgloss.NewStyle().Foreground(ColRule)
	RuleBrandStyle = lipgloss.NewStyle().Foreground(ColBrandDeep)

	Brand     = lipgloss.NewStyle().Foreground(ColBrand)
	BrandBold = lipgloss.NewStyle().Bold(true).Foreground(ColBrand)
	BrandSoft = lipgloss.NewStyle().Foreground(ColBrandSoft)
	TrackHead = lipgloss.NewStyle().Foreground(ColBrandDeep)
	Warn      = lipgloss.NewStyle().Foreground(ColWarn)

	// 에러는 색이 아니라 배지로 구분한다.
	ErrorBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(ColBrand).
			Bold(true).
			Padding(0, 1)
)

// 앨범 아트 자리. Bubble Tea 는 셀 기반 렌더러라 진짜 이미지를 띄울 수 없어
// 색 블록으로 대체한다.
//
// 여기는 브랜드 색을 크게 써도 되는 유일한 자리다. 원래 앨범 커버가 놓이는
// 곳이라 색이 있어도 경보로 읽히지 않는다.
var ArtShades = []string{"#4A1420", "#752232", "#A8253F", "#FF5A75", "#A8253F", "#752232"}

// 묘비 화면은 무채색을 쓴다. 되살릴 때 색이 돌아오는 대비가 이 제품의 핵심이다.
var ArtShadesDead = []string{"#2E2E36", "#3A3A42", "#44444E", "#3A3A42", "#2E2E36", "#26262C"}

func AlbumArt(shades []string, w, h int) string {
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
