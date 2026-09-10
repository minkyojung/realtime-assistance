package musicapp

import (
	"strings"
	"testing"

	"amcli/tui/internal/style"
	"charm.land/lipgloss/v2"
)

// 커서는 언제나 화면 안에 있어야 한다.
//
// 커버가 뜨면 목록 창이 절반으로 줄어드는데, 커서를 옮기는 쪽이 그것을 모른
// 채 커버 없던 시절의 큰 창으로 계산하면 커서가 창 밖으로 나가도 목록이
// 따라오지 않는다. 실제로 그랬다.
//
// 그래서 "몇 줄인가"를 비교하지 않고 **선택 표시가 그려졌는가**를 본다.
// 원인이 무엇이든 커서가 안 보이면 걸린다.

// selectedIsVisible 은 지금 크기로 목록을 그려 선택 표시를 찾는다.
//
// 표시가 무엇인지(레일이든 배경이든) 테스트가 알 필요는 없다. 지금 쓰는
// 색을 그대로 찍어 보고 그 조각이 화면에 있는지만 본다.
func selectedIsVisible(m Model) bool {
	return strings.Contains(m.viewList(m.bodyW-2, m.listWindow()), selectionSeq())
}

func selectionSeq() string {
	probe := lipgloss.NewStyle().Background(style.ColRowSel).Render("x")
	return probe[:strings.Index(probe, "m")+1]
}

func TestCursorFollowsWhenMovingDown(t *testing.T) {
	m := playingModel()
	// 커버가 뜨는 크기. 목록 창이 그만큼 줄어든다.
	m.bodyW, m.bodyH = 100, 24
	m.clampList()

	if _, big := m.viewNowPlaying(m.bodyW, m.bodyH); !big {
		t.Fatal("이 크기에서 커버가 안 뜬다 — 테스트가 원인을 못 짚는다")
	}
	if n := m.rowCount(); n < 40 {
		t.Fatalf("목록이 %d줄뿐이라 창 밖으로 나갈 수 없다", n)
	}

	for i := 0; i < 40; i++ {
		m.move(1)
		if !selectedIsVisible(m) {
			t.Fatalf("%d번째 ↓ 에서 커서가 화면 밖으로 나갔다 (창 %d줄, 커서 %d, 맨 위 %d)",
				i+1, m.listWindow(), m.listIdx, m.listTop)
		}
	}
}

// 올라갈 때도 마찬가지다.
func TestCursorFollowsWhenMovingUp(t *testing.T) {
	m := playingModel()
	m.bodyW, m.bodyH = 100, 24
	m.clampList()

	for i := 0; i < 40; i++ {
		m.move(1)
	}
	for i := 0; i < 40; i++ {
		m.move(-1)
		if !selectedIsVisible(m) {
			t.Fatalf("%d번째 ↑ 에서 커서가 화면 밖으로 나갔다", i+1)
		}
	}
}

// 커버가 없으면 창이 커진다. 그때도 같은 계산을 써야 한다.
func TestCursorFollowsWithoutArtwork(t *testing.T) {
	m := playingModel()
	m.art = nil
	m.bodyW, m.bodyH = 100, 24
	m.clampList()

	for i := 0; i < 40; i++ {
		m.move(1)
		if !selectedIsVisible(m) {
			t.Fatalf("%d번째 ↓ 에서 커서가 화면 밖으로 나갔다", i+1)
		}
	}
}

// 그리는 쪽과 커서를 옮기는 쪽이 같은 수를 봐야 한다. 이것이 원인이었다.
func TestDrawAndScrollAgreeOnTheWindow(t *testing.T) {
	m := playingModel()
	m.bodyW, m.bodyH = 100, 24

	// View 가 쓰는 계산을 그대로 따라 해 본다.
	drawn := m.listHeight(m.bodyH - headHeight(m.headView(m.bodyW, m.bodyH)) + 1)
	if got := m.listWindow(); got != drawn {
		t.Errorf("그릴 때는 %d줄, 커서를 옮길 때는 %d줄", drawn, got)
	}
}

// 선택은 줄 전체를 밝힌다.
//
// 한 칸짜리 레일은 스크롤이 어긋나 줄이 잘리면 그 칸부터 가려져, 고른 것이
// 어디 있는지를 잃었다. 줄 전체면 어디가 잘려도 남는다.
func TestSelectionSpansTheWholeLine(t *testing.T) {
	m := playingModel()
	m.bodyW, m.bodyH = 100, 30
	m.clampList()

	w := m.bodyW - 2
	lines := strings.Split(m.viewList(w, m.listWindow()), "\n")

	var picked string
	for _, l := range lines {
		if strings.Contains(l, selectionSeq()) {
			picked = l
			break
		}
	}
	if picked == "" {
		t.Fatal("선택된 줄이 없다")
	}
	if got := lipgloss.Width(picked); got != w {
		t.Errorf("선택한 줄이 %d칸이다 — 줄 끝까지 %d칸이어야 한다", got, w)
	}
	// 옛 레일이 남아 있으면 표시가 두 종류가 된다.
	if strings.Contains(picked, "▌") {
		t.Error("줄을 밝히면서 왼쪽 레일도 그린다")
	}
}

// 고를 수 없는 줄(구역 머리글)은 밝히지 않는다.
func TestHeadersAreNeverHighlighted(t *testing.T) {
	m := playingModel()
	m.bodyW, m.bodyH = 100, 30
	got := m.renderRow(listRow{header: "Apple Music"}, true, m.bodyW-2, []int{20, 10, 3, 4}, new(string))
	if strings.Contains(got, selectionSeq()) {
		t.Errorf("머리글을 밝혔다: %q", got)
	}
}
