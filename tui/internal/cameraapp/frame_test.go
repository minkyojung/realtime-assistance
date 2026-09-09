package cameraapp

import (
	"regexp"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

const (
	srcW = 160
	srcH = 120
)

// 왼쪽이 검정, 오른쪽이 흰색인 가로 그라데이션.
func gradient() []byte {
	px := make([]byte, srcW*srcH*3)
	for y := range srcH {
		for x := range srcW {
			v := byte(x * 255 / (srcW - 1))
			p := (y*srcW + x) * 3
			px[p], px[p+1], px[p+2] = v, v, v
		}
	}
	return px
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func strip(s string) string { return ansi.ReplaceAllString(s, "") }

// 본문은 호스트가 준 높이를 정확히 채워야 한다. 모자라면 아래가 뜨고,
// 넘치면 입력창을 밀어낸다.
func TestAsciiFillsExactHeight(t *testing.T) {
	px := gradient()
	for _, size := range [][2]int{{200, 40}, {40, 40}, {80, 24}, {13, 3}, {200, 3}} {
		got := ascii(px, srcW, srcH, size[0], size[1], false)
		if n := strings.Count(got, "\n") + 1; n != size[1] {
			t.Errorf("%dx%d: 줄 수 %d, 원한 것 %d", size[0], size[1], n, size[1])
		}
	}
}

func TestAsciiNeverExceedsWidth(t *testing.T) {
	px := gradient()
	for _, size := range [][2]int{{200, 40}, {40, 40}, {80, 24}, {13, 3}} {
		for i, line := range strings.Split(ascii(px, srcW, srcH, size[0], size[1], false), "\n") {
			// 호스트의 폭 검사와 같은 자를 쓴다. 이스케이프를 세면 안 된다.
			if n := lipgloss.Width(line); n > size[0] {
				t.Errorf("%dx%d: %d번째 줄이 %d칸, 폭은 %d", size[0], size[1], i, n, size[0])
			}
		}
	}
}

// 거울처럼 뒤집어 그리므로, 원본의 검은 왼쪽이 화면 오른쪽에 온다.
func TestAsciiMirrors(t *testing.T) {
	out := strip(ascii(gradient(), srcW, srcH, 120, 30, true))
	var row string
	for _, line := range strings.Split(out, "\n") {
		if len(line) > len(row) {
			row = line
		}
	}
	row = strings.TrimLeft(row, " ")
	if len(row) < 8 {
		t.Fatalf("이미지 줄을 찾지 못했다: %q", row)
	}
	prev := len(ramp)
	for i, c := range row {
		idx := strings.IndexRune(ramp, c)
		if idx < 0 {
			t.Fatalf("%d번째 문자 %q 가 램프 밖이다", i, c)
		}
		if idx > prev {
			t.Fatalf("%d번째에서 밝아졌다 — 반전이 안 됐다: %q", i, row)
		}
		prev = idx
	}
	// 블록 평균이라 끝 칸도 순백은 아니다. 램프의 맨 위 언저리면 된다.
	if first := strings.IndexRune(ramp, rune(row[0])); first < len(ramp)-2 {
		t.Errorf("왼쪽 끝이 가장 밝아야 한다, 얻은 인덱스 %d", first)
	}
	if last := strings.IndexRune(ramp, rune(row[len(row)-1])); last != 0 {
		t.Errorf("오른쪽 끝이 가장 어두워야 한다, 얻은 인덱스 %d", last)
	}
}

func TestAsciiMonoHasNoEscapes(t *testing.T) {
	px := gradient()
	if out := ascii(px, srcW, srcH, 120, 30, true); strings.Contains(out, "\x1b") {
		t.Error("--mono 인데 이스케이프가 들어 있다")
	}
	if out := ascii(px, srcW, srcH, 120, 30, false); !strings.Contains(out, "\x1b[38;2;") {
		t.Error("컬러 모드인데 트루컬러 이스케이프가 없다")
	}
}

// 색이 줄 밖으로 새면 호스트의 다음 줄까지 물든다.
func TestAsciiResetsEveryLine(t *testing.T) {
	for _, line := range strings.Split(ascii(gradient(), srcW, srcH, 120, 30, false), "\n") {
		if strings.Contains(line, "\x1b[38;2;") && !strings.HasSuffix(line, "\x1b[0m") {
			t.Fatalf("색을 쓴 줄이 되돌리지 않고 끝난다: %q", line)
		}
	}
}

// 창이 극단적으로 작거나 프레임이 덜 왔을 때도 죽지 않아야 한다.
func TestAsciiDegenerate(t *testing.T) {
	px := gradient()
	cases := []struct {
		w, h int
		px   []byte
	}{
		{0, 10, px}, {10, 0, px}, {1, 1, px}, {200, 1, px},
		{80, 24, nil}, {80, 24, px[:10]},
	}
	for _, c := range cases {
		got := ascii(c.px, srcW, srcH, c.w, c.h, false)
		if c.h > 0 {
			if n := strings.Count(got, "\n") + 1; n != c.h {
				t.Errorf("w=%d h=%d px=%d: 줄 수 %d", c.w, c.h, len(c.px), n)
			}
		}
	}
}

func BenchmarkAscii(b *testing.B) {
	px := gradient()
	for b.Loop() {
		ascii(px, srcW, srcH, 160, 44, false)
	}
}
