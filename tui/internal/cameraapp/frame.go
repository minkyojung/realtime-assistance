package cameraapp

import "strings"

// 프레임 하나를 본문 크기의 ASCII 로 바꾼다.
//
// 이 파일에는 카메라가 없다. 바이트 슬라이스가 들어와 문자열이 나갈 뿐이라
// 권한도 하드웨어도 없이 테스트할 수 있다 — 그러라고 헬퍼를 나눴다.

// 어두운 → 밝은 순서. 인덱스 = 휘도 * (len-1) / 255.
const ramp = " .:-=+*#%@"

// ascii 는 RGB24 프레임을 정확히 h 줄로 그린다.
//
// 색은 lipgloss 를 쓰지 않고 이스케이프를 직접 쓴다. 100x40 이면 칸이
// 4000 개고, 칸마다 Style.Render 를 부르면 프레임당 4000 번이다. 여기는
// 그 비용을 감당할 수 있는 자리가 아니다.
func ascii(px []byte, srcW, srcH, w, h int, mono bool) string {
	if w <= 0 || h <= 0 || srcW <= 0 || srcH <= 0 || len(px) < srcW*srcH*3 {
		return strings.Repeat("\n", max(h-1, 0))
	}

	// 터미널 문자 칸은 대략 세로:가로 = 2:1 이다. 가로 칸을 두 배로 잡아야
	// 원본 비율이 나온다. 그러고도 넘치면 가로에 맞추고 세로를 줄인다.
	rows, cols := h, h*2*srcW/srcH
	if cols > w {
		cols = w
		rows = cols * srcH / (2 * srcW)
	}
	if rows <= 0 || cols <= 0 {
		return strings.Repeat("\n", max(h-1, 0))
	}

	padTop := (h - rows) / 2
	padLeft := strings.Repeat(" ", (w-cols)/2)

	var b strings.Builder
	b.Grow(w * h * 6)
	for i := 0; i < padTop; i++ {
		b.WriteByte('\n')
	}

	for oy := range rows {
		y0 := oy * srcH / rows
		y1 := max(y0+1, (oy+1)*srcH/rows)
		b.WriteString(padLeft)
		lastKey := -1

		for ox := range cols {
			x0 := ox * srcW / cols
			x1 := max(x0+1, (ox+1)*srcW/cols)

			var sumR, sumG, sumB, n int
			for y := y0; y < y1; y++ {
				row := y * srcW
				for x := x0; x < x1; x++ {
					// 좌우 반전: 거울처럼 보이는 쪽이 자연스럽다.
					p := (row + srcW - 1 - x) * 3
					sumR += int(px[p])
					sumG += int(px[p+1])
					sumB += int(px[p+2])
					n++
				}
			}
			r, g, bl := sumR/n, sumG/n, sumB/n

			if !mono {
				// 색이 실질적으로 같으면 이스케이프를 생략한다. 한 줄이
				// 통째로 어두울 때 바이트 수가 몇 배로 줄어든다.
				key := r>>3<<10 | g>>3<<5 | bl>>3
				if key != lastKey {
					b.WriteString("\x1b[38;2;")
					writeInt(&b, r)
					b.WriteByte(';')
					writeInt(&b, g)
					b.WriteByte(';')
					writeInt(&b, bl)
					b.WriteByte('m')
					lastKey = key
				}
			}
			b.WriteByte(ramp[(299*r+587*g+114*bl)/1000*(len(ramp)-1)/255])
		}

		if !mono {
			// 줄마다 되돌린다. 호스트가 이 문자열을 다른 것들 사이에
			// 끼워 넣으므로, 색이 줄 밖으로 새면 안 된다.
			b.WriteString("\x1b[0m")
		}
		if padTop+oy < h-1 {
			b.WriteByte('\n')
		}
	}

	for i := padTop + rows; i < h-1; i++ {
		b.WriteByte('\n')
	}
	return b.String()
}

// strconv.Itoa 보다 할당이 없다. 프레임당 수천 번 불린다.
func writeInt(b *strings.Builder, v int) {
	if v >= 100 {
		b.WriteByte(byte('0' + v/100))
	}
	if v >= 10 {
		b.WriteByte(byte('0' + v/10%10))
	}
	b.WriteByte(byte('0' + v%10))
}
