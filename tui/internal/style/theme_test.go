package style

import (
	"image/color"
	"math"
	"testing"

	"charm.land/lipgloss/v2"
)

// 팔레트는 터미널 배경 위에서 읽혀야 한다.
//
// 한때 전부 상수였고 그중에는 "터미널 배경은 #1C1B27 일 것이다" 라는 추측도
// 있었다. 밝은 배경을 쓰는 사람에게는 흰 글씨가 흰 바탕에 얹혀 **앱이 통째로
// 안 보였다.** 아래 테스트가 지키는 것은 색이 아니라 그 성질이다.

var (
	white = lipgloss.Color("#FFFFFF")
	black = lipgloss.Color("#1C1B27")
)

// 원래대로 돌려놓는다. 팔레트가 전역이라 테스트끼리 샌다.
func restore(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { Retheme(black) })
}

// 대비 — WCAG 의 상대 명도비. 1(같음) ~ 21(검정 대 흰색).
//
// 이 패키지의 luma 를 쓰지 않는다. 그것은 감마가 걸린 sRGB 값을 그대로
// 재는 것이라 밝다/어둡다를 가르는 데는 충분하지만, **비율로 쓰면 틀린다.**
// 대비는 빛의 양을 비교하는 것이라 먼저 선형으로 되돌려야 한다.
func contrast(a, b color.Color) float64 {
	l1, l2 := relLum(a), relLum(b)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

// relLum 은 WCAG 의 상대 명도다. sRGB 의 감마를 풀고 눈의 가중치를 씌운다.
func relLum(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	lin := func(v uint32) float64 {
		f := float64(v) / 65535
		if f <= 0.04045 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

// 글씨는 어느 배경에서든 읽혀야 한다.
//
// 3.0 은 큰 글씨에 대한 WCAG 의 하한이다. 터미널 글꼴은 작지만, 여기서
// 잡으려는 것은 "읽기 좋은가" 가 아니라 **"보이기는 하는가"** 다 — 흰
// 바탕의 흰 글씨는 1.0 이다.
func TestTextIsReadableOnEitherBackground(t *testing.T) {
	restore(t)
	for _, bg := range []color.Color{black, white} {
		Retheme(bg)
		for _, c := range []struct {
			name string
			col  color.Color
		}{
			{"본문", ColFg},
			{"부제", ColDim},
			{"브랜드 글자", colBrandText},
			{"경고", ColWarn},
		} {
			if got := contrast(c.col, bg); got < 3.0 {
				t.Errorf("배경 %v 에서 %s 의 대비가 %.2f 다 — 안 보인다", bg, c.name, got)
			}
		}
	}
}

// 흐릿함은 "배경 쪽으로 가까운 것"이지 "어두운 것"이 아니다.
//
// 흰 바탕에서 흐릿한 회색은 밝은 회색이다. 뒤집기를 빠뜨리면 부제가
// 본문보다 진해져서 중요도가 거꾸로 읽힌다.
func TestFaintIsAlwaysCloserToTheBackground(t *testing.T) {
	restore(t)
	for _, bg := range []color.Color{black, white} {
		Retheme(bg)
		if contrast(ColFaint, bg) >= contrast(ColDim, bg) {
			t.Errorf("배경 %v 에서 faint 가 dim 보다 도드라진다", bg)
		}
		if contrast(ColDim, bg) >= contrast(ColFg, bg) {
			t.Errorf("배경 %v 에서 dim 이 본문보다 도드라진다", bg)
		}
	}
}

// 선택 줄은 배경과 달라야 하되, 글씨를 덮으면 안 된다.
func TestSelectedRowShowsWithoutHidingTheText(t *testing.T) {
	restore(t)
	for _, bg := range []color.Color{black, white} {
		Retheme(bg)
		if ColRowSel == bg {
			t.Errorf("배경 %v 에서 선택 줄이 배경과 같다", bg)
		}
		if got := contrast(ColFg, ColRowSel); got < 3.0 {
			t.Errorf("배경 %v 에서 선택된 줄의 글씨 대비가 %.2f 다", bg, got)
		}
	}
}

// 내가 친 문장의 띠도 마찬가지다. 보이되 글씨를 덮지 않는다.
func TestMySaidBandShowsWithoutHidingTheText(t *testing.T) {
	restore(t)
	for _, bg := range []color.Color{black, white} {
		Retheme(bg)
		if ColSaidByMe == bg {
			t.Errorf("배경 %v 에서 띠가 배경과 같다", bg)
		}
		if got := contrast(ColFg, ColSaidByMe); got < 3.0 {
			t.Errorf("배경 %v 에서 띠 위 글씨의 대비가 %.2f 다", bg, got)
		}
	}
}

// 스타일도 같이 다시 만들어져야 한다.
//
// 스타일이 색을 값으로 품으므로, 색만 바꾸고 두면 스타일은 옛 색을 계속
// 쓴다. 화면에 나가는 것은 색이 아니라 스타일이다.
func TestStylesFollowTheRetheme(t *testing.T) {
	restore(t)
	Retheme(black)
	dark := Body.Render("x")
	Retheme(white)
	if light := Body.Render("x"); light == dark {
		t.Error("배경이 바뀌었는데 본문 스타일이 그대로다")
	}
}

// 배경이 어느 쪽인지는 사람 눈의 가중치로 정한다.
//
// 채널 평균으로 재면 파란 배경을 어둡다고 잘못 읽는다 — 눈은 초록을 72%,
// 파랑을 7% 로 본다.
func TestDarknessUsesTheEyeNotTheAverage(t *testing.T) {
	// 채널 평균은 같지만 사람 눈에는 초록이 훨씬 밝다.
	green := lipgloss.Color("#00C000")
	blue := lipgloss.Color("#0000C0")
	if isDark(green) {
		t.Error("밝은 초록을 어둡다고 읽었다")
	}
	if !isDark(blue) {
		t.Error("어두운 파랑을 밝다고 읽었다")
	}
}

// 아무것도 안 물어봤을 때의 기본은 어두운 쪽이다.
// 대부분의 터미널이 어둡고, 답이 오면 한 프레임 뒤에 정정된다.
func TestDefaultAssumesADarkTerminal(t *testing.T) {
	restore(t)
	Retheme(black)
	if !isDark(ColBase) {
		t.Error("기본 배경이 어둡지 않다")
	}
}
