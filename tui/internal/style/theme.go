package style

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
	ColBrand       = lipgloss.Color("#FF5A75") // Apple Music red
	ColBrandSoft   = lipgloss.Color("#FF8FA3") // 본문에 얹을 때 읽기 좋은 톤
	ColBrandDeep   = lipgloss.Color("#A8253F")
	ColBrandShadow = lipgloss.Color("#7A2036") // 워드마크 뒤에 선 그림자 글자

	ColFg    = lipgloss.Color("#E8E8EA")
	ColDim   = lipgloss.Color("#8A8A92")
	ColFaint = lipgloss.Color("#55555E")
	ColRule  = lipgloss.Color("#2E2E36")
	ColWarn  = lipgloss.Color("#E8A33D")

	// 대화 띠에서 **내가 친 문장**의 바탕. 답에는 깔지 않는다.
	//
	// 둘 다 칠하면 이번에는 둘이 구별되지 않는다. 하나만 칠하면 나머지는
	// 저절로 갈린다. 칠하는 쪽을 내 말로 정한 것은 답이 길고 내 말이 짧기
	// 때문이다 — 긴 쪽을 칠하면 화면의 절반이 색면이 된다.
	//
	// 거의 검정이다. 덩어리가 나뉘는 것만 보이면 되고, 그 이상 밝아지면
	// 글자보다 바탕이 먼저 눈에 든다.
	ColSaidByMe = lipgloss.Color("#23232E")
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
