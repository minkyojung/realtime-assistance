package style

import "testing"

// 음악 목록이 쓰는 것과 같은 선언.
var musicCols = []Col{
	{Min: 12, Weight: 3, Max: 58},          // 제목 — 절대 안 버린다
	{Min: 10, Weight: 1, Max: 26, Drop: 2}, // 아티스트
	{Min: 3, Drop: 3},                      // 재생수 — 제일 먼저 버린다
	{Min: 5, Drop: 1},                      // 길이
}

const gap = 2

// 매직 넘버 대신 성질을 검증한다. 숫자는 튜닝하면 바뀌지만 성질은 안 바뀐다.
func TestColumnsNeverOverflow(t *testing.T) {
	for w := 8; w <= 200; w++ {
		got := Columns(w, gap, musicCols)
		sum, n := 0, 0
		for _, v := range got {
			if v > 0 {
				sum, n = sum+v, n+1
			}
		}
		if n > 1 && sum+gap*(n-1) > w {
			t.Fatalf("폭 %d 를 넘겼다: %v", w, got)
		}
	}
}

// Drop 이 0 인 칸은 어떤 폭에서도 살아남아야 한다.
func TestColumnsKeepsEssential(t *testing.T) {
	for w := 8; w <= 200; w++ {
		if got := Columns(w, gap, musicCols); got[0] <= 0 {
			t.Fatalf("폭 %d 에서 제목이 사라졌다: %v", w, got)
		}
	}
}

// Drop 이 큰 것부터 사라져야 한다 — 재생수 → 아티스트 → 길이.
func TestColumnsDropOrder(t *testing.T) {
	var seen [4]int // 각 칸이 처음 나타나는 폭
	for w := 8; w <= 200; w++ {
		for i, v := range Columns(w, gap, musicCols) {
			if v > 0 && seen[i] == 0 {
				seen[i] = w
			}
		}
	}
	// 제목이 가장 먼저, 재생수가 가장 늦게 나타난다.
	if !(seen[0] <= seen[3] && seen[3] <= seen[1] && seen[1] <= seen[2]) {
		t.Errorf("버리는 순서가 선언과 다르다: %v", seen)
	}
}

// 창을 넓히면 제목이 넓어져야 한다. 공백이 아니라 정보가 늘어야 한다.
func TestColumnsGiveSlackToTitle(t *testing.T) {
	prev := 0
	for w := 60; w <= 200; w++ {
		got := Columns(w, gap, musicCols)[0]
		if got < prev {
			t.Fatalf("폭 %d 에서 제목이 오히려 줄었다: %d → %d", w, prev, got)
		}
		prev = got
	}
	// 넓히면 상한까지는 자란다.
	if got := Columns(200, gap, musicCols)[0]; got != musicCols[0].Max {
		t.Errorf("넓은 창에서 제목이 상한(%d)까지 안 자랐다: %d", musicCols[0].Max, got)
	}
}

// 상한을 넘지 않아야 한다. 아주 넓은 창에서 한 칸이 다 먹으면 안 된다.
func TestColumnsRespectMax(t *testing.T) {
	for w := 8; w <= 400; w++ {
		got := Columns(w, gap, musicCols)
		for i, c := range musicCols {
			if c.Max > 0 && got[i] > c.Max {
				t.Fatalf("폭 %d: %d번 칸이 상한 %d 를 넘었다 (%d)", w, i, c.Max, got[i])
			}
		}
	}
}
