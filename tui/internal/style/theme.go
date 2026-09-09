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
//
// **sRGB 값 그대로 섞는다.** 선형 공간에서 섞는 것이 광학적으로는 옳지만,
// 그러면 같은 a 에서 빨강이 48이나 더 밝아져(a=0.17 에 #762D3D) 옅게 얹은
// 것이 아니라 자주색 띠가 된다. 여기서 원하는 것은 정확함이 아니라
// "살짝 물든" 것이다. 옳아 보인다고 선형으로 바꾸면 화면이 망가진다.
func tint(bg, fg color.Color, a float64) color.Color {
	br, bgn, bb, _ := bg.RGBA()
	fr, fgn, fb, _ := fg.RGBA()
	mix := func(b, f uint32) uint8 {
		// RGBA() 는 16비트로 돌려준다. 8비트로 내려서 섞는다.
		//
		// 반올림한다. float→uint8 은 자르기라서 그냥 두면 세 채널이 저마다
		// 1씩 깎이고, 얹은 색이 늘 의도보다 옅어진다.
		return uint8(float64(b>>8)*(1-a) + float64(f>>8)*a + 0.5)
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
//
// **값이 아니라 계산 결과다.** 한때 여기 전부가 상수였고, 그중에는
// "터미널 배경은 #1C1B27 일 것이다" 라는 추측도 있었다. 밝은 배경을 쓰는
// 사람에게는 흰 글씨가 흰 바탕에 얹혀 앱이 통째로 안 보였다.
//
// 터미널은 자기 배경색을 알려준다(tea.BackgroundColorMsg). 추측할 이유가
// 없다 — Retheme 이 그 값을 받아 여기를 다시 만든다.
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

	// 글자로 쓰는 브랜드 색. **채우는 색과 나눈 이유가 있다.**
	//
	// #FF5A75 는 채워 놓으면 어느 배경에서든 애플뮤직이지만, 흰 바탕에
	// 글씨로 얹으면 대비가 모자라 읽히지 않는다. 정체성은 채우는 쪽이
	// 지키고(진행 바·배지), 읽혀야 하는 쪽만 배경을 따라간다.
	colBrandText = ColBrand

	// 터미널 배경. **우리가 칠하는 색이 아니라 뒤에 비치는 색이다.**
	//
	// 투명도를 미리 섞어 두려면 무엇 위에 얹는지를 알아야 해서 필요하다.
	// 시작할 때는 이 기본값이고, 터미널이 답하면 진짜 값으로 바뀐다.
	// 기본을 어두운 쪽으로 둔 것은 대부분의 터미널이 어둡고, 답이 오면
	// 한 프레임 뒤에 정정되기 때문이다.
	ColBase = lipgloss.Color("#1C1B27")

	// 선택된 줄의 배경 — 브랜드 색을 옅게 얹은 것.
	//
	// 터미널에는 투명도가 없다. 그래서 배경 위에 미리 섞어 불투명한 한
	// 색으로 만든다. 브랜드 색을 그대로 채우지 않는 이유는, 이 화면에서
	// **채워진 빨강이 "오류"라는 뜻**이기 때문이다(ErrorBadge). 옅게 얹은
	// 것은 그 규칙과 부딪히지 않는다 — 다른 것은 색조가 아니라 진하기다.
	//
	// 17% 는 눈대중이 아니다. 이미 화면에 있는 띠(ColSaidByMe)가 배경보다
	// **밝기(luma)** 로 16.9 밝은데, 브랜드 색을 그와 같은 무게로 얹는 값이다.
	//
	// 채널당 평균으로 재면 안 된다. 빨강은 R 이 227 움직이는 동안 G 는 63밖에
	// 안 움직이는데 눈은 G 를 72% 로 보므로, 평균을 맞추면 실제로는 17% 어두운
	// 띠가 나온다. 한 번 그렇게 잡았다가 두 띠의 무게가 눈에 띄게 달랐다.
	ColRowSel = tint(ColBase, ColBrand, rowSelMix)

	// 대화 띠에서 **내가 친 문장**의 바탕. 답에는 깔지 않는다.
	//
	// 둘 다 칠하면 이번에는 둘이 구별되지 않는다. 하나만 칠하면 나머지는
	// 저절로 갈린다. 칠하는 쪽을 내 말로 정한 것은 답이 길고 내 말이 짧기
	// 때문이다 — 긴 쪽을 칠하면 화면의 절반이 색면이 된다.
	//
	// 배경에서 글씨색 쪽으로 살짝 들어 올린 값이다. 한때 #2C2C3A 로 박혀
	// 있었는데, 그것은 한 터미널에서 픽셀로 재서 잡은 값이라 다른 배경에서는
	// 배경과 붙어 안 보이거나 너무 튀었다. 계산하면 어느 배경에서든 같은
	// 만큼 들린다.
	ColSaidByMe = tint(ColBase, ColFg, saidMix)
)

// 얹는 비율. 이름을 붙여 두는 것은 위 주석이 가리키는 값이기 때문이다.
const (
	rowSelMix = 0.17
	saidMix   = 0.085
)

// Retheme 은 터미널이 알려준 실제 배경색으로 팔레트를 다시 만든다.
//
// **Update 에서만 부른다.** 여기서 갈아끼우는 것은 패키지 전역이고,
// 명령(Cmd)은 다른 고루틴에서 돈다. 지금 명령 안에서 색을 읽는 곳은
// 없으므로 안전하지만, 생기면 이 규칙이 깨진다.
//
// 밝기로 어느 쪽인지 정한다. 사람 눈의 가중치를 쓴다 — 같은 값이라도
// 초록이 밝게 보이고 파랑이 어둡게 보이므로, 채널 평균으로 재면 파란
// 배경을 어둡다고 잘못 읽는다.
func Retheme(bg color.Color) {
	ColBase = bg
	if isDark(bg) {
		ColFg = lipgloss.Color("#E8E8EA")
		ColDim = lipgloss.Color("#8A8A92")
		ColFaint = lipgloss.Color("#55555E")
		ColRule = lipgloss.Color("#2E2E36")
		ColBrandSoft = lipgloss.Color("#FF8FA3")
		ColWarn = lipgloss.Color("#E8A33D")
		colBrandText = ColBrand
	} else {
		// 밝은 배경에서는 **전부 뒤집는다.** 흐릿함은 "배경 쪽으로 가까운
		// 것"이지 "어두운 것"이 아니다 — 흰 바탕에서 흐릿한 회색은 밝은
		// 회색이다. 여기서 뒤집기를 빠뜨리면 부제가 본문보다 진해진다.
		ColFg = lipgloss.Color("#1C1B27")
		ColDim = lipgloss.Color("#6C6C76")
		ColFaint = lipgloss.Color("#A0A0AA")
		ColRule = lipgloss.Color("#D8D8DE")
		// 밝은 바탕에서 #FF8FA3 은 거의 안 읽힌다. 진한 쪽을 쓴다.
		ColBrandSoft = lipgloss.Color("#A8253F")
		ColWarn = lipgloss.Color("#9A6A14")
		colBrandText = ColBrandDeep
	}
	ColRowSel = tint(ColBase, ColBrand, rowSelMix)
	ColSaidByMe = tint(ColBase, ColFg, saidMix)
	rebuild()
}

// isDark 는 이 색 위에 밝은 글씨를 얹어야 하는가다.
func isDark(c color.Color) bool { return luma(c) < 0.5 }

// luma 는 사람 눈이 느끼는 밝기다 (0~1).
func luma(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	return (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)) / 65535
}

var (
	Title          lipgloss.Style
	Track          lipgloss.Style
	Body           lipgloss.Style
	Meta           lipgloss.Style
	Dim            lipgloss.Style
	Faint          lipgloss.Style
	RuleStyle      lipgloss.Style
	RuleBrandStyle lipgloss.Style

	Brand     lipgloss.Style
	BrandBold lipgloss.Style
	BrandSoft lipgloss.Style
	TrackHead lipgloss.Style
	Warn      lipgloss.Style

	// 에러는 색이 아니라 배지로 구분한다.
	ErrorBadge lipgloss.Style
)

func init() { rebuild() }

// rebuild 는 팔레트에서 스타일을 다시 만든다.
//
// 스타일이 색을 값으로 품으므로, 색만 바꾸고 두면 스타일은 옛 색을 계속
// 쓴다. 팔레트를 손대는 곳이 Retheme 하나뿐인 이유이기도 하다.
func rebuild() {
	Title = lipgloss.NewStyle().Bold(true).Foreground(ColFg)
	Track = lipgloss.NewStyle().Bold(true).Foreground(ColFg)
	Body = lipgloss.NewStyle().Foreground(ColFg)
	Meta = lipgloss.NewStyle().Foreground(ColDim)
	Dim = lipgloss.NewStyle().Foreground(ColDim)
	Faint = lipgloss.NewStyle().Foreground(ColFaint)
	RuleStyle = lipgloss.NewStyle().Foreground(ColRule)
	RuleBrandStyle = lipgloss.NewStyle().Foreground(ColBrandDeep)

	Brand = lipgloss.NewStyle().Foreground(colBrandText)
	BrandBold = lipgloss.NewStyle().Bold(true).Foreground(colBrandText)
	BrandSoft = lipgloss.NewStyle().Foreground(ColBrandSoft)
	TrackHead = lipgloss.NewStyle().Foreground(ColBrandDeep)
	Warn = lipgloss.NewStyle().Foreground(ColWarn)

	ErrorBadge = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(ColBrand).
		Bold(true).
		Padding(0, 1)
}

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
