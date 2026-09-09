package host

import (
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
// 그리고 **지금 무엇이 막혀 있는가**. 환영 인사도 버전도 없다.
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
// 두 줄로 쌓는다. 한 줄로 늘이면 68칸이 필요해서 80칸 터미널에서 아슬아슬한데,
// 쌓으면 38칸이라 어디서든 들어간다. 두 단어짜리 이름에는 그편이 자연스럽기도 하다.
//
// 브랜드 색을 쓴다. 입체감은 둘로 낸다 — 글자 자체는 위는 밝고 아래는
// 짙은 세로 그라데이션(style.WordmarkGradient), 그리고 그 실루엣 바깥으로
// 한 칸짜리 어두운 외곽선. 오른쪽 아래로 밀어 깐 두꺼운 그림자는 한 번
// 해봤다가 뺐다 — 획 사이 좁은 틈이 뭉개져 글자가 아니라 얼룩으로 보였다.
var wordmark = []string{
	" ████   █████   █████   ██      ██████",
	"██  ██  ██  ██  ██  ██  ██      ██",
	"██████  █████   █████   ██      █████",
	"██  ██  ██      ██      ██      ██",
	"██  ██  ██      ██      ██████  ██████",
	"",
	"██   ██  ██  ██   █████  ██   █████",
	"███ ███  ██  ██  ██      ██  ██",
	"██ █ ██  ██  ██   ████   ██  ██",
	"██   ██  ██  ██      ██  ██  ██",
	"██   ██   ████   █████   ██   █████",
}

// 앞 단어가 끝나는 줄. 두 단어를 나눠 그려야 외곽선이 단어 안에서만 진다 —
// 뒤 단어 첫 줄에 앞 단어 외곽선이 새어 들면 안 된다.
const wordmarkSplit = 5

// 가장 긴 줄. 이만큼도 못 그리는 폭이면 이름을 글자로 적는다.
const wordmarkWidth = 38

// renderWordmarkOutline 은 블록 글자에 외곽선을 둘러 도장처럼 보이게 한다.
// 외곽선은 글자가 아니면서 바로 위나 아래 칸에 글자가 있는 칸이다.
// 위아래만 보는 이유는 좌우까지 보면 글자 사이 좁은 틈(예: M 가운데)이
// 양쪽에서 메워져 글자가 얼룩으로 뭉개지기 때문이다.
//
// fg 는 줄마다 다른 색을 줄 수 있다 — len(fg) 가 len(lines) 보다 짧으면
// 마지막 색을 반복해 쓴다.
func renderWordmarkOutline(lines []string, fg []lipgloss.Style, outline lipgloss.Style) []string {
	grid := make([][]rune, len(lines))
	width := 0
	for i, line := range lines {
		grid[i] = []rune(line)
		if len(grid[i]) > width {
			width = len(grid[i])
		}
	}

	at := func(y, x int) rune {
		if y >= 0 && y < len(lines) && x >= 0 && x < len(grid[y]) {
			return grid[y][x]
		}
		return 0
	}

	out := make([]string, len(lines))
	for y := range out {
		idx := y
		if idx >= len(fg) {
			idx = len(fg) - 1
		}
		fgStyle := &fg[idx] // 줄 안에서 색이 안 바뀌니 x 루프 밖에서 한 번만 잡는다

		// 칸이 비어 있으면 지금 진행 중인 색을 그대로 물고 간다 —
		// 매번 색을 끊으면 원래 한 번에 그리던 줄이 조각나 달라 보인다.
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
		for x := 0; x < width; x++ {
			var want *lipgloss.Style
			ch := ' '
			if r := at(y, x); r != ' ' && r != 0 {
				want, ch = fgStyle, r
			} else if at(y-1, x) != ' ' && at(y-1, x) != 0 || at(y+1, x) != ' ' && at(y+1, x) != 0 {
				want, ch = &outline, '█'
			}
			if want != nil && runStyle != nil && want != runStyle {
				flush()
			}
			if want != nil {
				runStyle = want
			}
			run = append(run, ch)
		}
		flush()
		out[y] = b.String()
	}
	return out
}

// 조작법 — 화면에 아직 안 적혀 있는 것만 적는다.
//
// `/` 와 `?` 는 입력창 플레이스홀더가 이미 말하고 있으므로 넣지 않는다.
// 같은 것을 두 번 말하면 둘 다 안 읽힌다.
var homeTips = []struct{ key, what string }{
	{"type", `a sentence — "something quiet, nothing i've skipped"`},
	{"enter", "open the library"},
	{"ctrl+o", "what the last request actually did"},
}

// 조작키를 적는 칸. 설명이 세로로 맞으면 눈이 한 번만 움직인다.
const homeKeyCol = 10

// viewHome 은 본문 자리를 정확히 h 줄로 채운다.
// 앱으로 들어갈 때 화면이 튀지 않아야 한다.
func (m Model) viewHome(w, h int) string {
	// 아래쪽을 먼저 정한다. 조작법과 관문은 자리가 아무리 없어도 남는다 —
	// 이것이 무엇인지는 몰라도 되지만, 어떻게 쓰고 무엇이 막혔는지는 알아야 한다.
	var tail []string
	for _, t := range homeTips {
		tail = append(tail, "  "+style.BrandSoft.Render(t.key)+
			strings.Repeat(" ", style.Max(homeKeyCol-len(t.key), 1))+
			style.Faint.Render(style.Truncate(t.what, style.Max(w-2-homeKeyCol, 0))))
	}
	// 관문이 막혀 있으면 사유를 붙인다. 조작법보다 뒤인 이유는 막혀 있어도
	// 조작법은 그대로이기 때문이다.
	//
	// 사유는 그릴 때마다 Ready() 를 읽는다. 확정 시점이 앱마다 달라서다 —
	// 음악은 첫 폴링이 와야 안다. 그래서 멀쩡해 보이다가 잠시 뒤 경고가
	// 붙을 수 있다. 확인 중임을 알리는 스피너를 도는 것보다 그편이 조용하다.
	if err := m.app().Ready(); err != nil {
		// 경고는 "! " 다. 앱 본문이 이미 그렇게 그리므로(musicapp/player.go)
		// 여기서 새 기호를 들이면 화면에 경고가 두 종류가 된다.
		tail = append(tail, "", "  "+style.Warn.Render("! ")+
			style.Dim.Render(style.Truncate(err.Error(), style.Max(w-4, 0))))
	}

	// 남은 자리에 이름을 넣는다. 큰 것 → 글자 → 없음 순으로 물러선다.
	title := "  " + style.Title.Render("APPLE MUSIC")
	tagline := "  " + style.Faint.Render(style.Truncate(m.app().Tagline(), style.Max(w-2, 0)))

	big := []string{""}
	for _, line := range renderWordmarkOutline(wordmark[:wordmarkSplit], style.WordmarkGradient, style.BrandOutline) {
		big = append(big, "  "+line)
	}
	big = append(big, "") // 두 단어 사이. 빈 줄에 색을 입히지 않는다
	for _, line := range renderWordmarkOutline(wordmark[wordmarkSplit+1:], style.WordmarkGradient, style.BrandOutline) {
		big = append(big, "  "+line)
	}
	big = append(big, "", tagline, "")
	small := []string{"", title, "", tagline, ""}

	var head []string
	switch room := h - len(tail); {
	case w >= wordmarkWidth+2 && room >= len(big):
		head = big
	case room >= len(small):
		head = small
	case room >= 2:
		head = []string{title, ""}
	}

	rows := append(head, tail...)
	for len(rows) < h {
		rows = append(rows, "")
	}
	return strings.Join(rows[:h], "\n")
}
