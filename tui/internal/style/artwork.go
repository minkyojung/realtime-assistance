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
// 셀 하나에 픽셀 둘을 넣는다 — 위 반칸(▀)에 글자색, 아래 반칸에 배경색.
// 그래서 24칸 × 12줄이면 24 × 24 픽셀이 되고, 셀이 가로1 세로2 비율이므로
// 화면에서 정사각형으로 보인다.
//
// kitty 그래픽 프로토콜이나 iTerm2 인라인 이미지를 쓰면 훨씬 선명하지만,
// **터미널에 따라 되다 안 되다 한다**(docs/07 2절). 반칸은 색만 있으면
// 어디서든 된다. 거칠어도 앨범 커버는 원래 색 덩어리라 알아볼 수 있다.

// 반칸 — 위는 글자, 아래는 배경.
const upperHalf = "▀"

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
const artworkCache = 128

// Artwork 는 그림을 cols 칸 × rows 줄로 그린다. 픽셀로는 cols × 2rows 다.
//
// 원본 비율은 지키지 않는다. 앨범 커버는 정사각형이고, 부르는 쪽이
// 정사각형이 되는 칸 수를 넘긴다.
func Artwork(img image.Image, cols, rows int) []string {
	if img == nil || cols < 1 || rows < 1 {
		return nil
	}
	px := resize(img, cols, rows*2)
	out := make([]string, 0, rows)
	for r := 0; r < rows; r++ {
		var b strings.Builder
		for c := 0; c < cols; c++ {
			top := at(px, c, r*2)
			bottom := at(px, c, r*2+1)
			b.WriteString(lipgloss.NewStyle().
				Foreground(top).Background(bottom).Render(upperHalf))
		}
		out = append(out, b.String())
	}
	return out
}

func at(img *image.RGBA, x, y int) color.Color {
	return lipgloss.Color(hex(img.RGBAAt(x+img.Rect.Min.X, y+img.Rect.Min.Y)))
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
