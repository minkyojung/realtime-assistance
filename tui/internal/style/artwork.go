package style

import (
	"image"
	"image/color"
	_ "image/jpeg" // Music.app 이 주는 커버는 대부분 JPEG
	_ "image/png"
	"strings"

	"charm.land/lipgloss/v2"
)

// 터미널에 앨범 커버를 그린다.
//
// 셀 하나에 색을 둘 넣을 수 있다 — 글자색과 배경색. 그 둘을 사분면
// 문자(▘▝▖▗▌▐▀▄▚▞…)로 나눠 칠하면 **한 칸이 2×2 픽셀**이 된다.
// 24칸 × 12줄이면 48 × 24 픽셀이다.
//
// 반칸(▀)만 쓰면 한 칸이 1×2 라 24칸에 가로 표본이 24개뿐이었다.
// 사분면을 쓰면 가로가 두 배가 된다 — 커버의 글자와 윤곽이 그만큼 산다.
//
// 한 칸에 색은 여전히 둘뿐이므로, 네 픽셀을 어떻게 두 무리로 가를지가
// 유일한 판단이다. 경우의 수가 열여섯뿐이라 전부 세어 보고 오차가
// 가장 작은 것을 고른다. 근사가 아니라 최적이다.
//
// kitty 그래픽 프로토콜이나 iTerm2 인라인 이미지를 쓰면 진짜 사진이
// 뜨지만, **터미널에 따라 되다 안 되다 한다**(docs/07 2절).
// 사분면은 색만 있으면 어디서든 된다.

// 사분면 문자. 자리값은 왼쪽위1 · 오른쪽위2 · 왼쪽아래4 · 오른쪽아래8 이고,
// 켜진 자리가 글자색, 꺼진 자리가 배경색이다.
var quadrants = [16]string{
	" ", "▘", "▝", "▀",
	"▖", "▌", "▞", "▛",
	"▗", "▚", "▐", "▜",
	"▄", "▙", "▟", "█",
}

// DecodeArtwork 는 원본 바이트를 그림으로 바꾼다.
//
// 원본은 1200×1200 쯤 된다. 화면에 필요한 것은 백 픽셀도 안 되므로
// 여기서 한 번 줄여 두고, 그릴 때마다 다시 줄이지 않는다.
func DecodeArtwork(b []byte) (image.Image, error) {
	img, _, err := image.Decode(strings.NewReader(string(b)))
	if err != nil {
		return nil, err
	}
	return resize(img, artworkCache, artworkCache), nil
}

// 줄여 둘 크기. 화면이 아무리 커도 이보다 잘게 그리지 않는다.
const artworkCache = 192

// Artwork 는 그림을 cols 칸 × rows 줄로 그린다. 픽셀로는 2cols × 2rows 다.
//
// 원본 비율은 지키지 않는다. 앨범 커버는 정사각형이고, 부르는 쪽이
// 정사각형이 되는 칸 수를 넘긴다.
func Artwork(img image.Image, cols, rows int) []string {
	if img == nil || cols < 1 || rows < 1 {
		return nil
	}
	px := resize(img, cols*2, rows*2)
	out := make([]string, 0, rows)
	for r := 0; r < rows; r++ {
		var b strings.Builder
		for c := 0; c < cols; c++ {
			glyph, fg, bg := cell([4]color.RGBA{
				px.RGBAAt(c*2, r*2), px.RGBAAt(c*2+1, r*2),
				px.RGBAAt(c*2, r*2+1), px.RGBAAt(c*2+1, r*2+1),
			})
			b.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color(hex(fg))).
				Background(lipgloss.Color(hex(bg))).
				Render(glyph))
		}
		out = append(out, b.String())
	}
	return out
}

// cell — 픽셀 넷을 문자 하나와 색 둘로 줄인다.
//
// 열여섯 가지 나누기를 전부 세어 보고 제곱 오차가 가장 작은 것을 고른다.
// 0 과 15 는 네 픽셀이 한 무리인 경우로, 평균색 하나로 칠한다.
func cell(p [4]color.RGBA) (glyph string, fg, bg color.RGBA) {
	best, bestErr := 0, -1.0
	var bestFg, bestBg color.RGBA
	for mask := 0; mask < 16; mask++ {
		f, fn := mean(p, mask, true)
		b, bn := mean(p, mask, false)
		if fn == 0 {
			f = b // 글자색이 안 쓰이는 칸. 배경색을 그대로 둔다
		}
		if bn == 0 {
			b = f
		}
		e := 0.0
		for i, c := range p {
			ref := b
			if mask&(1<<i) != 0 {
				ref = f
			}
			e += dist(c, ref)
		}
		if bestErr < 0 || e < bestErr {
			best, bestErr, bestFg, bestBg = mask, e, f, b
		}
	}
	return quadrants[best], bestFg, bestBg
}

// mask 에 켜진(또는 꺼진) 자리의 평균색.
func mean(p [4]color.RGBA, mask int, on bool) (color.RGBA, int) {
	var r, g, b, n int
	for i, c := range p {
		if (mask&(1<<i) != 0) != on {
			continue
		}
		r += int(c.R)
		g += int(c.G)
		b += int(c.B)
		n++
	}
	if n == 0 {
		return color.RGBA{}, 0
	}
	return color.RGBA{R: uint8(r / n), G: uint8(g / n), B: uint8(b / n), A: 255}, n
}

func dist(a, b color.RGBA) float64 {
	dr := float64(a.R) - float64(b.R)
	dg := float64(a.G) - float64(b.G)
	db := float64(a.B) - float64(b.B)
	// 눈이 초록에 제일 민감하고 파랑에 둔하다. 그 비율로 무게를 준다.
	return 2*dr*dr + 4*dg*dg + 3*db*db
}

const hexDigits = "0123456789abcdef"

func hex(c color.RGBA) string {
	b := []byte{'#', 0, 0, 0, 0, 0, 0}
	for i, v := range []uint8{c.R, c.G, c.B} {
		b[1+i*2] = hexDigits[v>>4]
		b[2+i*2] = hexDigits[v&0xf]
	}
	return string(b)
}

// resize — 상자 평균으로 줄인다.
//
// 최근접 이웃을 쓰면 커버의 글자와 무늬가 들쭉날쭉해진다. 평균을 내면
// 뭉개지지만, 어차피 24픽셀로 줄이는 마당에 선명함은 애초에 없다.
// golang.org/x/image 를 들이지 않는 이유는 이 열 줄이 전부이기 때문이다.
func resize(src image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	b := src.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		return dst
	}
	for y := 0; y < h; y++ {
		y0 := b.Min.Y + y*b.Dy()/h
		y1 := b.Min.Y + (y+1)*b.Dy()/h
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < w; x++ {
			x0 := b.Min.X + x*b.Dx()/w
			x1 := b.Min.X + (x+1)*b.Dx()/w
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var sr, sg, sb, n uint64
			for yy := y0; yy < y1; yy++ {
				for xx := x0; xx < x1; xx++ {
					r, g, bb, _ := src.At(xx, yy).RGBA()
					sr += uint64(r >> 8)
					sg += uint64(g >> 8)
					sb += uint64(bb >> 8)
					n++
				}
			}
			if n == 0 {
				n = 1
			}
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(sr / n), G: uint8(sg / n), B: uint8(sb / n), A: 255,
			})
		}
	}
	return dst
}
