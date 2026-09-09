package musicapp

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// Artists·Albums 는 곡이 아니라 묶음을 보여준다. 커서가 서는 줄이므로
// enter 에 뜻이 있어야 한다 — 없으면 tab 으로 갈 수 있는 막다른 길이 된다.

// atSection 은 그 섹션을 보고 있는 모델을 만든다.
func atSection(t *testing.T, kind sectionKind) Model {
	t.Helper()
	m := New()
	m.bodyH = 20
	for i, s := range m.sections {
		if s.kind == kind {
			m.sectionIdx = i
			return m
		}
	}
	t.Fatalf("섹션 %d 가 없다", kind)
	return m
}

// key 는 키를 하나 먹이고 모델을 돌려준다.
func press(t *testing.T, m Model, k tea.KeyPressMsg) Model {
	t.Helper()
	next, _ := m.Update(k)
	mm, ok := next.(Model)
	if !ok {
		t.Fatalf("Update 가 Model 을 안 돌려줬다: %T", next)
	}
	return mm
}

var (
	enter = tea.KeyPressMsg{Code: tea.KeyEnter}
	tab   = tea.KeyPressMsg{Code: tea.KeyTab}
)

func TestEnterOpensAGroup(t *testing.T) {
	m := atSection(t, secArtists)

	rows := m.rows()
	if len(rows) == 0 {
		t.Fatal("픽스처에 아티스트가 없다")
	}
	g := rows[m.listIdx].group
	if g == nil {
		t.Fatalf("%d번 줄이 묶음이 아니다", m.listIdx)
	}
	want := g.TrackCount

	m = press(t, m, enter)

	if m.drill == nil {
		t.Fatal("enter 를 눌렀는데 묶음이 안 열렸다")
	}
	if got := len(m.rows()); got != want {
		t.Errorf("곡이 %d개 보여야 하는데 %d개다", want, got)
	}
	for i, r := range m.rows() {
		if r.track == nil {
			t.Fatalf("%d번 줄이 곡이 아니다", i)
		}
	}
}

// 나온 자리에 커서를 되돌린다. 훑던 중이었으므로 맨 위로 튕기면 안 된다.
func TestBackReturnsToWhereYouWere(t *testing.T) {
	m := atSection(t, secArtists)
	m.move(1)
	before := m.listIdx
	if before == 0 {
		t.Skip("아티스트가 하나뿐이라 커서를 옮길 수 없다")
	}

	m = press(t, m, enter)
	if m.drill == nil {
		t.Fatal("묶음이 안 열렸다")
	}

	back, ok := m.Back()
	if !ok {
		t.Fatal("파고든 상태인데 물러날 곳이 없다고 한다")
	}
	m = back.(Model)

	if m.drill != nil {
		t.Error("물러났는데 아직 파고든 상태다")
	}
	if m.listIdx != before {
		t.Errorf("커서가 %d 로 돌아와야 하는데 %d 다", before, m.listIdx)
	}
}

// 파고들지 않았으면 esc 는 앱의 것이 아니다. 호스트가 자기 차례를 밟아야 한다.
func TestBackDoesNothingWhenNotDrilled(t *testing.T) {
	if _, ok := atSection(t, secRecent).Back(); ok {
		t.Error("파고든 적이 없는데 물러났다고 한다")
	}
}

// 섹션을 옮기면 파고든 것은 없던 일이 된다. 안 그러면 Songs 를 보고 있는데
// 목록은 아까 그 아티스트인 상태가 된다.
func TestChangingSectionLeavesTheGroup(t *testing.T) {
	m := press(t, atSection(t, secArtists), enter)
	if m.drill == nil {
		t.Fatal("묶음이 안 열렸다")
	}
	if m = press(t, m, tab); m.drill != nil {
		t.Error("tab 으로 섹션을 옮겼는데 파고든 상태가 남아 있다")
	}
}

// 상태줄이 어디를 보고 있는지 말한다. 사이드바가 없으므로 여기밖에 없다.
func TestStatusNamesTheGroup(t *testing.T) {
	m := press(t, atSection(t, secArtists), enter)

	got := m.Status()
	if !strings.Contains(got, m.drill.Name) || !strings.Contains(got, "Artists") {
		t.Errorf("상태줄이 어디인지 말하지 않는다: %q", got)
	}
}

// 섹션을 옮기면 파고든 것을 놓고 커서가 맨 위로 간다.
// 섹션마다 줄 수가 달라 자리를 물려주면 없는 줄을 가리킨다.
func TestMovingSectionsResetsTheCursor(t *testing.T) {
	m := atSection(t, secArtists)
	m = press(t, m, enter) // 묶음을 편다
	if m.drill == nil {
		t.Fatal("묶음이 안 열렸다")
	}
	m.listIdx = 3

	m = press(t, m, tab)
	if m.drill != nil {
		t.Error("섹션을 옮겼는데 파고든 것이 남아 있다")
	}
	if m.listIdx != 0 || m.listTop != 0 {
		t.Errorf("커서가 맨 위로 안 갔다: idx=%d top=%d", m.listIdx, m.listTop)
	}
}
