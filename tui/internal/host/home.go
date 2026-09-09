package host

import (
	"image/color"
	"strings"

	"amcli/tui/internal/style"

	"charm.land/lipgloss/v2"
)

// 홈 — 켜면 여기서 시작하고, esc 로 여기로 돌아온다.
//
// 커튼이 아니라 화면이다. 글자를 쳐도 걷히지 않는다. 걷히는 것은 목적지를
// 골랐을 때뿐이고, 길은 셋이다 — enter, `/music`, 그리고 문장을 쳐서
// 라우터가 앱을 지목했을 때.
//
// 여기서 말하는 것은 셋이다. **이것이 무엇인가**, **어떻게 조작하는가**,
// 그리고 **지금 무엇이 막혀 있는가**. 환영 인사는 없다.
//
// 자리가 넉넉하면 넷째가 붙는다 — **무엇에 붙어 있는가**(homebox.go).
// 오래 버전을 적지 않았던 이유는 그것이 자기 이야기여서였는데, 붙은
// 곳을 적는 것은 다르다. "네 것만 튼다"는 주장이 진짜 그 사람의
// Music.app 에 붙어 있을 때만 사실이기 때문이다. 버전은 그 목록의 한
// 줄로만 따라 들어간다. 자리가 없으면 넷째부터 사라진다.
//
// 한때 여기에 앱 목록이 있었다. 앱이 하나가 되면서 한 줄짜리 목록이
// 남았는데, 고를 것이 하나뿐인 목록은 목록이 아니라 그냥 버튼이다.
// 그래서 목록을 걷어내고 그 자리에 이름과 조작법을 뒀다. 앱이 다시
// 늘어나면 목록도 돌아와야 한다 — `git show 4d3828c:tui/internal/host/home.go`.
//
// 본문 자리를 통째로 쓴다. 팔레트는 보고 있던 목록을 가리면 맥락을
// 잃지만(overlay.go), 홈에는 가릴 맥락이 없다 — 홈이 맥락이다.
//
// 호스트의 두 줄은 그대로 산다. 계약이 "그 위만 앱의 것"이므로
// 홈도 그 위에만 있어야 한다.

// 이름표. 로고가 아니라 이름을 쓰는 이유는, 어깨너머로 보는 사람에게
// 이것이 무엇인지 한 번에 읽혀야 하기 때문이다.
//
// 두 줄로 쌓는다. 한 줄로 늘이면 80칸 터미널에서 아슬아슬한데, 쌓으면
// 45칸이라 어디서든 들어간다. 두 단어짜리 이름에는 그편이 자연스럽기도 하다.
//
// 글자는 픽셀 그림이다. 픽셀 한 칸이 가로 한 칸 × 세로 반 줄이라 거의
// 정사각형이고, 반블록 문자(▀ ▄)로 한 줄에 픽셀 두 줄을 담는다. 세로
// 해상도를 두 배로 쓰는 셈인데, 그래야 획을 2픽셀로 굵게 그리고 그 위에
// 1픽셀짜리 가는 테두리를 얹을 수 있다. 한 칸을 통째로 한 픽셀로 쓰면
// 획과 테두리가 같은 굵기가 되어 그림자가 "가장자리"가 아니라 "글자 하나
// 더"로 보인다.
// 낱자는 12픽셀 높이, 획은 2픽셀이다. A 의 꼭대기는 양 모서리를 한 칸씩
// 깎았다 — A 는 뾰족한 글자라 윗변이 평평하면 다른 글자처럼 읽힌다.
var wordmarkGlyphs = map[rune][]string{
	'A': {
		".#####.",
		"#######",
		"##...##",
		"##...##",
		"##...##",
		"#######",
		"#######",
		"##...##",
		"##...##",
		"##...##",
		"##...##",
		"##...##",
	},
	'P': {
		"#######",
		"#######",
		"##...##",
		"##...##",
		"##...##",
		"#######",
		"#######",
		"##.....",
		"##.....",
		"##.....",
		"##.....",
		"##.....",
	},
	'L': {
		"##.....",
		"##.....",
		"##.....",
		"##.....",
		"##.....",
		"##.....",
		"##.....",
		"##.....",
		"##.....",
		"##.....",
		"#######",
		"#######",
	},
	'E': {
		"#######",
		"#######",
		"##.....",
		"##.....",
		"##.....",
		"######.",
		"######.",
		"##.....",
		"##.....",
		"##.....",
		"#######",
		"#######",
	},
	'M': {
		"##.....##",
		"###...###",
		"####.####",
		"##.###.##",
		"##..#..##",
		"##.....##",
		"##.....##",
		"##.....##",
		"##.....##",
		"##.....##",
		"##.....##",
		"##.....##",
	},
	'U': {
		"##...##",
		"##...##",
		"##...##",
		"##...##",
		"##...##",
		"##...##",
		"##...##",
		"##...##",
		"##...##",
		"##...##",
		"#######",
		"#######",
	},
	'S': {
		"#######",
		"#######",
		"##.....",
		"##.....",
		"##.....",
		"#######",
		"#######",
		".....##",
		".....##",
		".....##",
		"#######",
		"#######",
	},
	'I': {
		"##", "##", "##", "##", "##", "##",
		"##", "##", "##", "##", "##", "##",
	},
	'C': {
		"#######",
		"#######",
		"##.....",
		"##.....",
		"##.....",
		"##.....",
		"##.....",
		"##.....",
		"##.....",
		"##.....",
		"#######",
		"#######",
	},
}

// 낱자 사이 틈. 2픽셀이면 그림자 윤곽선이 틈을 다 먹어 글자끼리 붙는다 —
// 윤곽선이 밀린 글자 바깥으로 한 칸 더 나가기 때문이다.
const wordmarkLetterGap = 3

// wordmarkPixels 는 낱자를 이어 붙여 한 단어의 픽셀 그림을 만든다.
func wordmarkPixels(word string) []string {
	rows := make([]string, len(wordmarkGlyphs['A']))
	for i, r := range word {
		g := wordmarkGlyphs[r]
		for y := range rows {
			if i > 0 {
				rows[y] += strings.Repeat(".", wordmarkLetterGap)
			}
			rows[y] += g[y]
		}
	}
	return rows
}

var wordmarkWords = [][]string{wordmarkPixels("APPLE"), wordmarkPixels("MUSIC")}

// 가장 넓은 단어의 픽셀 폭. 이만큼도 못 그리는 폭이면 이름을 글자로 적는다.
var wordmarkWidth = func() int {
	w := 0
	for _, word := range wordmarkWords {
		w = style.Max(w, len(word[0]))
	}
	return w
}()

// 그림자 글자를 미는 픽셀 수 — 오른쪽 dx, 아래 dy. 픽셀이 정사각형에
// 가까워서 (1, 1) 이 곧 45도, 왼쪽 위에서 오는 빛이다.
const (
	wordmarkShadowDX = 1
	wordmarkShadowDY = 1
)

// 큰 이름표가 실제로 먹는 폭. 밀린 그림자와 그 바깥 테두리 한 칸이 더 붙는다.
var wordmarkDrawWidth = wordmarkWidth + wordmarkShadowDX + 1

// 픽셀 한 칸이 어느 겹에 속하는지. 0 은 아무것도 아니라 투명하게 둔다.
const (
	wordmarkBlank = iota
	wordmarkFace
	wordmarkShadow
)

// renderWordmark 는 픽셀 그림을 반블록 문자로 옮긴다.
//
// 겹은 둘이다. 본 글자는 속을 채우고, 그 뒤에 (dx, dy) 만큼 밀린 그림자
// 글자가 서는데 속을 비우고 윤곽선만 남긴다 — 윤곽선은 그림자 글자가
// 아니면서 상하좌우 중 하나가 그림자 글자인 칸, 즉 실루엣 바깥 한 칸이다.
// 속을 비우므로 그 안은 아무것도 그리지 않고, 터미널 배경이 그대로 비친다.
// 배경색으로 칠하면 배경이 단색이 아닐 때 사각형 얼룩으로 보인다.
//
// 겹치는 칸은 언제나 본 글자가 이긴다.
func renderWordmark(px []string, dx, dy int, face, shadow color.Color) []string {
	grid := make([][]rune, len(px))
	width := 0
	for i, row := range px {
		grid[i] = []rune(row)
		if len(grid[i]) > width {
			width = len(grid[i])
		}
	}

	filled := func(y, x int) bool {
		if y < 0 || y >= len(grid) || x < 0 || x >= len(grid[y]) {
			return false
		}
		return grid[y][x] == '#'
	}
	// 그림자는 본 글자를 민 것이다. 따로 두지 않고 좌표만 민다.
	behind := func(y, x int) bool { return filled(y-dy, x-dx) }
	layer := func(y, x int) int {
		switch {
		case filled(y, x):
			return wordmarkFace
		case !behind(y, x) && (behind(y-1, x) || behind(y+1, x) || behind(y, x-1) || behind(y, x+1)):
			return wordmarkShadow
		}
		return wordmarkBlank
	}

	// 위/아래 픽셀 조합마다 쓸 문자와 스타일을 미리 정해둔다. 런을 묶을 때
	// 주소를 비교하므로 스타일은 한 번만 만들어야 한다.
	col := [3]color.Color{nil, face, shadow}
	var cellRune [3][3]rune
	var cellStyle [3][3]lipgloss.Style
	for t := range col {
		for b := range col {
			switch {
			case t == wordmarkBlank && b == wordmarkBlank:
				cellRune[t][b] = ' '
			case t == b:
				cellRune[t][b] = '█'
				cellStyle[t][b] = lipgloss.NewStyle().Foreground(col[t])
			case b == wordmarkBlank:
				cellRune[t][b] = '▀'
				cellStyle[t][b] = lipgloss.NewStyle().Foreground(col[t])
			case t == wordmarkBlank:
				cellRune[t][b] = '▄'
				cellStyle[t][b] = lipgloss.NewStyle().Foreground(col[b])
			default:
				// 위아래가 서로 다른 색일 때만 배경을 쓴다. 이 칸은 둘 다
				// 불투명해서 배경이 비칠 일이 없다.
				cellRune[t][b] = '▀'
				cellStyle[t][b] = lipgloss.NewStyle().Foreground(col[t]).Background(col[b])
			}
		}
	}

	// 그림자 윤곽선이 본 글자 바깥으로 한 칸 더 나간다. 그만큼 넓혀 잡지
	// 않으면 오른쪽과 아래 테두리가 잘린다.
	rows, cols := len(grid)+dy+1, width+dx+1

	out := make([]string, (rows+1)/2)
	for r := range out {
		// 칸이 비어 있으면 지금 진행 중인 색을 그대로 물고 간다 —
		// 매번 색을 끊으면 한 번에 그리던 줄이 조각나 달라 보인다.
		var b strings.Builder
		var run []rune
		var runStyle *lipgloss.Style
		flush := func() {
			if len(run) == 0 {
				return
			}
			if runStyle == nil {
				b.WriteString(string(run))
			} else {
				b.WriteString(runStyle.Render(string(run)))
			}
			run = run[:0]
		}
		for x := 0; x < cols; x++ {
			top, bottom := layer(2*r, x), layer(2*r+1, x)

			var want *lipgloss.Style
			if top != wordmarkBlank || bottom != wordmarkBlank {
				want = &cellStyle[top][bottom]
			}
			if want != nil && runStyle != nil && want != runStyle {
				flush()
			}
			if want != nil {
				runStyle = want
			}
			run = append(run, cellRune[top][bottom])
		}
		flush()
		out[r] = b.String()
	}
	return out
}

// 관문 — 홈에서 할 수 있는 일의 전부다.
//
// 조작법을 목록으로 늘어놓지 않고 이 한 줄만 둔다. 홈에서 손이 하는 일은
// 들어가는 것뿐이고, `/` 와 `?` 는 입력창 플레이스홀더가 이미 말한다
// (host.go applyMode) — 같은 것을 두 번 말하면 둘 다 안 읽힌다.
const homeGate = "Press Enter to start"

// centerRow 는 한 줄을 폭 안에서 가운데에 놓는다.
//
// 오른쪽은 채우지 않는다. 빈칸을 채워봐야 보이지 않고, 줄 끝에 공백이
// 붙으면 골든 파일과 폭 검사가 그것까지 세게 된다.
func centerRow(w int, s string) string {
	if pad := (w - lipgloss.Width(s)) / 2; pad > 0 {
		return strings.Repeat(" ", pad) + s
	}
	return s
}

// viewHome 은 본문 자리를 정확히 h 줄로 채운다.
// 앱으로 들어갈 때 화면이 튀지 않아야 한다.
func (m Model) viewHome(w, h int) string {
	// 관문은 화면 폭의 한가운데다.
	//
	// 왼쪽에 맞추지 않는 이유는 맞출 것이 없기 때문이다. 홈에는 입력창도
	// 상태줄도 없어서(host.go View) 손이 가는 자리가 따로 없고, 받는 키가
	// 이것 하나뿐인 화면에서는 한가운데가 곧 "여기를 보라"는 뜻이다.
	tail := []string{centerRow(w, style.BrandSoft.Render(homeGate))}

	// 관문이 막혀 있으면 사유를 붙인다. 관문보다 뒤인 이유는 막혀 있어도
	// 들어가는 길은 그대로이기 때문이다.
	//
	// 사유는 그릴 때마다 Ready() 를 읽는다. 확정 시점이 앱마다 달라서다 —
	// 음악은 첫 폴링이 와야 안다. 그래서 멀쩡해 보이다가 잠시 뒤 경고가
	// 붙을 수 있다. 확인 중임을 알리는 스피너를 도는 것보다 그편이 조용하다.
	if err := m.app().Ready(); err != nil {
		// 경고는 "! " 다. 앱 본문이 이미 그렇게 그리므로(musicapp/player.go)
		// 여기서 새 기호를 들이면 화면에 경고가 두 종류가 된다.
		tail = append(tail, centerRow(w, style.Warn.Render("! ")+
			style.Dim.Render(style.Truncate(err.Error(), style.Max(w-4, 0)))))
	}

	// 남은 자리에 이름을 넣는다. 큰 것 → 글자 → 없음 순으로 물러선다.
	//
	// 한 줄짜리 소개 문구는 뺐다. 박스가 이 앱이 무엇에 붙어 있는지를
	// 목록으로 말하게 된 뒤로, 그 위에 얹힌 한 문장은 같은 것을 더 흐리게
	// 말하는 줄이 됐다. 문구 자체는 계약에 남는다 — 라우터와 앱 전환이
	// 그것을 읽는다(app.App.Tagline).
	title := style.Title.Render("APPLE MUSIC")

	// 두 단어를 따로 그린다. 한 번에 그리면 앞 단어 그림자가 뒤 단어
	// 첫 줄까지 새어 든다.
	var big []string
	for i, word := range wordmarkWords {
		if i > 0 {
			big = append(big, "") // 두 단어 사이. 빈 줄에 색을 입히지 않는다
		}
		for _, line := range renderWordmark(word, wordmarkShadowDX, wordmarkShadowDY,
			style.ColBrand, style.ColBrandShadow) {
			big = append(big, line)
		}
	}
	small := []string{"", title, ""}

	// 박스는 이름표와 관문 사이의 자리를 쓴다(homebox.go).
	//
	// 자리가 모자라면 박스가 스스로 접는다. 그때는 지금까지의 화면 그대로다 —
	// 이름표가 이 화면의 본론이고 박스는 그 각주다.
	room := h - len(tail)
	var head []string
	switch {
	case w >= wordmarkDrawWidth && room >= len(big):
		head = big
		// 이름표와 박스 사이 한 줄, 박스와 관문 사이 한 줄.
		if box := m.homeBoxRows(w, room-len(big)-2); len(box) > 0 {
			head = append(append(append([]string{}, big...), ""), box...)
		}
	case room >= len(small):
		head = small
	case room >= 1:
		head = []string{title}
	}

	// 세로로도 남는 자리의 한가운데다. 화면 전체의 한가운데가 아닌 이유는,
	// 그 자리가 이름표 안이기 때문이다. 박스가 서서 남는 자리를 다 쓰면
	// 관문은 박스 바로 아래가 된다.
	rows := append([]string{}, head...)
	for i := 0; i < (room-len(head))/2; i++ {
		rows = append(rows, "")
	}
	return fillTo(append(rows, tail...), h)
}

// fillTo 는 줄들을 정확히 h 줄로 맞춘다. 모자라면 빈 줄로 채우고 넘치면 자른다.
func fillTo(rows []string, h int) string {
	for len(rows) < h {
		rows = append(rows, "")
	}
	return strings.Join(rows[:h], "\n")
}
