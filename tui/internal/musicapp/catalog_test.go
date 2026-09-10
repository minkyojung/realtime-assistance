package musicapp

import (
	"amcli/tui/internal/app"
	"amcli/tui/internal/secrets"
	"amcli/tui/internal/style"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"amcli/tui/internal/api"
	"amcli/tui/internal/applemusic"
	"amcli/tui/internal/data"
	"amcli/tui/internal/data/fixture"
	"charm.land/lipgloss/v2"
)

func TestMain(m *testing.M) {
	data.Set(fixture.Lib())
	m.Run()
}

func catalogTrack(title, artist string) api.CatalogTrack {
	return api.CatalogTrack{AppleMusicId: "1", Title: title, ArtistName: artist}
}

// 카탈로그와 Music.app 은 같은 곡을 다르게 적는다.
// 곱슬 어포스트로피가 가장 흔한 어긋남이다.
func TestLocalMatchNormalizesQuotes(t *testing.T) {
	// 픽스처에 있는 곡. 곱슬 어포스트로피로 물어도 찾아야 한다.
	got, ok := localMatch(catalogTrack(
		"I Can’t Make You Love Me (AIR Studios – 4AD/Jagjaguwar Session)", "Bon Iver"))
	if !ok {
		t.Fatal("곱슬 어포스트로피 제목을 못 찾았다")
	}
	if got.Title == "" {
		t.Error("빈 곡이 돌아왔다")
	}

	// 대소문자도 무시한다.
	if _, ok := localMatch(catalogTrack("holocene", "bon iver")); !ok {
		t.Error("대소문자 차이로 못 찾았다")
	}

	// 없는 곡은 없다고 해야 한다.
	if _, ok := localMatch(catalogTrack("전혀 없는 곡", "아무개")); ok {
		t.Error("없는 곡을 찾았다고 한다")
	}
}

// 이미 가진 곡은 담을 필요가 없다. 그 판정이 재생 경로를 가른다.
func TestMarkInLibrary(t *testing.T) {
	items := markInLibrary([]api.CatalogTrack{
		catalogTrack("Holocene", "Bon Iver"),
		catalogTrack("존재하지 않는 곡", "Nobody"),
	})
	if !inLibrary(items[0]) {
		t.Error("가진 곡을 못 알아봤다")
	}
	if inLibrary(items[1]) {
		t.Error("없는 곡을 가졌다고 한다")
	}
	// 원본을 건드리면 안 된다 — 상태는 Update 에서만 바뀐다.
	if items[0].InLibrary == nil {
		t.Error("InLibrary 가 안 채워졌다")
	}
}

// Apple 이 답을 줬으면 그 위에 짐작을 덮어쓰지 않는다.
//
// 이름으로 맞추는 판정은 같은 곡의 다른 판을 못 가른다. Apple 은 가른다.
// 여기서 다시 짐작하면 이 커밋 전체가 없던 일이 된다.
func TestAppleAnswerIsNotOverwrittenByGuessing(t *testing.T) {
	m := New()
	yes := true
	// 픽스처 라이브러리에 없는 곡이다 — 이름으로 맞추면 반드시 false 가 된다.
	held := catalogTrack("어느 라이브러리에도 없는 제목", "아무개")
	held.InLibrary = &yes

	mm, _, _ := m.applyCatalog(catalogMsg{
		seq: m.catSeq, term: "x", items: []api.CatalogTrack{held}, marked: true,
	})
	got := mm.(Model)
	if len(got.catHits) != 1 || !inLibrary(got.catHits[0]) {
		t.Error("Apple 이 담겼다고 한 곡을 이름으로 짐작해 덮었다")
	}
}

// 반대로 아무도 안 채웠으면 그때는 이름으로 짐작한다.
// 로그인 전에도 검색은 되어야 하고, 그때 유일한 단서가 이름이다.
func TestUnmarkedResultsStillFallBackToGuessing(t *testing.T) {
	m := New()
	mm, _, _ := m.applyCatalog(catalogMsg{
		seq: m.catSeq, term: "x", marked: false,
		items: []api.CatalogTrack{catalogTrack("Holocene", "Bon Iver")},
	})
	got := mm.(Model)
	if len(got.catHits) != 1 || !inLibrary(got.catHits[0]) {
		t.Error("로그인 전 대비책이 안 돌았다")
	}
}

// 늦게 온 응답은 버려야 한다. /catalog a 직후 /catalog b 를 칠 수 있다.
func TestStaleCatalogResponseIsDropped(t *testing.T) {
	m := New()
	m.catSeq = 2
	m.catHits = []api.CatalogTrack{catalogTrack("keep", "me")}

	mm, _, handled := m.applyCatalog(catalogMsg{
		seq:   1, // 지나간 요청
		term:  "old",
		items: []api.CatalogTrack{catalogTrack("stale", "result")},
	})
	if !handled {
		t.Fatal("처리되지 않았다")
	}
	got := mm.(Model)
	if len(got.catHits) != 1 || got.catHits[0].Title != "keep" {
		t.Errorf("낡은 답이 결과를 덮었다: %+v", got.catHits)
	}
}

// 설정이 없으면 카탈로그 섹션 자체가 없어야 한다.
// 갈 수 없는 곳으로 tab 이 가면 안 된다.
func TestNoCatalogSectionWhenUnconfigured(t *testing.T) {
	m := New()
	for _, s := range m.sections {
		if s.kind == secCatalog {
			t.Fatal("설정이 없는데 카탈로그 섹션이 있다")
		}
	}
}

// 카탈로그 행이 라이브러리 행과 같은 격자에 그려져야 한다.
// 한글 제목은 폭이 2 라, 칸 계산이 틀리면 화면이 넘친다.
func TestCatalogRowFitsWidth(t *testing.T) {
	m := New()
	m.catHits = markInLibrary([]api.CatalogTrack{
		{AppleMusicId: "1", Title: "바이, 썸머", ArtistName: "아이유", Year: intp(2024)},
		{AppleMusicId: "2", Title: "Holocene", ArtistName: "Bon Iver"},
	})
	m.sections = append(m.sections, section{kind: secCatalog, label: "Apple Music"})
	m.sectionIdx = len(m.sections) - 1

	for _, w := range []int{40, 60, 80, 100, 120} {
		out := m.viewList(w, 5)
		for _, line := range splitLines(out) {
			if got := visibleWidth(line); got > w {
				t.Errorf("폭 %d 에서 줄이 넘쳤다 (%d): %q", w, got, line)
			}
		}
	}

	// 담긴 곡과 안 담긴 곡의 표시가 달라야 한다 — 소속이 한눈에 보여야 한다.
	rows := m.rows()
	if len(rows) != 2 {
		t.Fatalf("행 수: got %d, want 2", len(rows))
	}
	if rows[0].catalog == nil {
		t.Error("카탈로그 행이 아니다")
	}
}

func intp(n int) *int { return &n }

func splitLines(s string) []string {
	var out, cur = []string{}, ""
	for _, r := range s {
		if r == '\n' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	return append(out, cur)
}

// ANSI 이스케이프를 뺀 화면상의 폭.
func visibleWidth(s string) int {
	return lipgloss.Width(s)
}

// 검색 한 번이 두 구역을 보여준다. 위는 내 것, 아래는 아직 내 것이 아닌 것.
func TestUnifiedSearchSplitsIntoTwoSections(t *testing.T) {
	m := New()
	m.cat = &applemusic.Client{DevToken: "x"} // nil 이 아니기만 하면 된다
	m = m.Filter("oasis").(Model)
	m.catTerm = "oasis"
	m.catHits = markInLibrary([]api.CatalogTrack{
		{AppleMusicId: "1", Title: "Wonderwall", ArtistName: "Oasis"},
	})

	rows := m.rows()
	var headers []string
	for _, r := range rows {
		if r.header != "" {
			headers = append(headers, r.header)
		}
	}
	// 머리글에는 그 구역이 몇 개인지도 적힌다.
	if len(headers) != 2 ||
		!strings.HasPrefix(headers[0], "Your Library · ") ||
		!strings.HasPrefix(headers[1], "Apple Music · ") {
		t.Fatalf("구역 머리글: %v", headers)
	}
	// 내 것이 먼저, 바깥이 나중이어야 한다.
	if !strings.HasPrefix(rows[0].header, "Your Library") {
		t.Error("내 라이브러리가 위에 있어야 한다")
	}
	if rows[len(rows)-1].catalog == nil {
		t.Error("마지막 줄이 카탈로그 곡이어야 한다")
	}
}

// 낡은 결과가 다른 검색어의 화면에 섞이면 안 된다.
func TestSearchDropsResultsFromAnotherTerm(t *testing.T) {
	m := New()
	m.cat = &applemusic.Client{DevToken: "x"}
	m = m.Filter("oasis").(Model)
	m.catTerm = "아이유" // 다른 검색어의 결과
	m.catHits = markInLibrary([]api.CatalogTrack{
		{AppleMusicId: "1", Title: "Blueming", ArtistName: "아이유"},
	})

	for _, r := range m.rows() {
		if r.catalog != nil {
			t.Fatal("다른 검색어의 결과가 섞였다")
		}
	}
}

// 머리글에는 커서가 서지 않는다. enter 가 아무것도 안 하는 자리를 만들지 않는다.
func TestCursorSkipsHeaders(t *testing.T) {
	m := New()
	m.bodyH = 20
	m.cat = &applemusic.Client{DevToken: "x"}
	m = m.Filter("oasis").(Model)
	m.catTerm = "oasis"
	m.catHits = markInLibrary([]api.CatalogTrack{
		{AppleMusicId: "1", Title: "Wonderwall", ArtistName: "Oasis"},
	})

	rows := m.rows()
	m.listIdx = 0
	m.clampList()
	if !rows[m.listIdx].selectable() {
		t.Fatal("커서가 머리글에 섰다")
	}
	// 끝까지 내려가며 한 번도 머리글에 서지 않아야 한다.
	for i := 0; i < len(rows)+2; i++ {
		m.move(1)
		if !m.rows()[m.listIdx].selectable() {
			t.Fatalf("%d번째 이동에서 머리글에 섰다 (idx=%d)", i, m.listIdx)
		}
	}
	// 다시 올라올 때도 마찬가지다.
	for i := 0; i < len(rows)+2; i++ {
		m.move(-1)
		if !m.rows()[m.listIdx].selectable() {
			t.Fatalf("%d번째 역이동에서 머리글에 섰다 (idx=%d)", i, m.listIdx)
		}
	}
}

// 타이핑하는 동안에는 요청이 나가지 않는다.
func TestCatalogSearchWaitsForTypingToStop(t *testing.T) {
	m := New()
	m.cat = &applemusic.Client{DevToken: "x"}
	m = m.Filter("oas").(Model)

	if cmd := m.maybeSearchCatalog(); cmd != nil {
		t.Error("치는 도중에 요청이 나갔다")
	}
	// 타이핑이 멎으면 나간다.
	m.filterAt = time.Now().Add(-time.Second)
	if cmd := m.maybeSearchCatalog(); cmd == nil {
		t.Error("타이핑이 멎었는데 요청이 안 나갔다")
	}
	// 이미 그 검색어의 결과를 갖고 있으면 다시 안 나간다.
	m.catBusy = false
	m.catTerm = "oas"
	if cmd := m.maybeSearchCatalog(); cmd != nil {
		t.Error("같은 검색어로 또 나갔다")
	}
}

// ── 두 구역의 자리 배분 ──────────────────────────────────────────────

func searching(t *testing.T, q string, hits int) Model {
	t.Helper()
	m := New()
	m.synced, m.bodyH = true, 26
	m.cat = &applemusic.Client{DevToken: "x"}
	mm := m.Filter(q).(Model)
	mm.catTerm = q
	for i := 0; i < hits; i++ {
		mm.catHits = append(mm.catHits, api.CatalogTrack{
			Title: fmt.Sprintf("Catalog %d", i), ArtistName: "Someone"})
	}
	return mm
}

// 이것이 이 변경의 이유다. 라이브러리에 많이 걸려도 Apple Music 은 화면 안에.
//
// 전에는 두 구역이 한 줄로 이어져 있어서 `a` 한 글자에 193곡이 걸리면
// Apple Music 머리글이 194번째 줄이었다. 결과는 오는데 아무도 못 봤다.
func TestAppleMusicStaysOnScreen(t *testing.T) {
	for _, q := range []string{"a", "e", "live", "the"} {
		m := searching(t, q, 25)
		mine := len(data.Lib().Search(q))
		if mine < 10 {
			continue // 접힐 일이 없는 검색어
		}
		out := ansiOff(m.View(78, 26))
		if !strings.Contains(out, "Apple Music · 25") {
			t.Errorf("%q — 라이브러리 %d곡에 밀려 Apple Music 이 화면 밖이다", q, mine)
		}
		if !strings.Contains(out, "Catalog 0") {
			t.Errorf("%q — Apple Music 결과가 한 줄도 안 보인다", q)
		}
	}
}

// 넘치는 만큼은 한 줄로 접고, 그 줄에서 enter 면 펼친다.
func TestMineFoldsAndExpands(t *testing.T) {
	m := searching(t, "live", 25)
	mine := len(data.Lib().Search("live"))
	if mine <= m.mineCap() {
		t.Skip("접힐 만큼 안 걸린다")
	}

	rows := m.rows()
	var more int
	for _, r := range rows {
		if r.isMore() {
			more = r.more
		}
	}
	if more != mine-m.mineCap() {
		t.Fatalf("접힌 수가 %d — %d여야 한다", more, mine-m.mineCap())
	}

	// 접힌 줄에서 enter.
	for i, r := range rows {
		if r.isMore() {
			m.listIdx = i
		}
	}
	next, _ := m.playSelected()
	opened := next.(Model)
	if !opened.searchAll {
		t.Fatal("enter 를 눌렀는데 안 펼쳐졌다")
	}
	if n := len(opened.rows()); n <= len(rows) {
		t.Error("펼쳤는데 줄이 안 늘었다")
	}
	for _, r := range opened.rows() {
		if r.isMore() {
			t.Error("펼쳤는데 접힌 줄이 남아 있다")
		}
	}

	// 검색어를 바꾸면 다시 접힌다.
	if again := opened.Filter("oasis").(Model); again.searchAll {
		t.Error("새 검색어인데 펼친 채로 남았다")
	}
}

// 접힌 줄에서 ↓ 를 누르면 곧장 Apple Music 결과로 간다.
//
// 머리글도 빈 줄도 커서가 서지 않으므로, 숨은 73줄을 지나칠 필요가 없다.
func TestFoldedRowsAreSkipped(t *testing.T) {
	m := searching(t, "live", 25)
	rows := m.rows()
	for i, r := range rows {
		if !r.isMore() {
			continue
		}
		m.listIdx = i
		m.move(1)
		if m.listIdx == i {
			t.Fatal("접힌 줄에서 아래로 못 간다")
		}
		if rows[m.listIdx].catalog == nil {
			t.Errorf("접힌 줄 다음 커서가 %d번째 — Apple Music 결과여야 한다", m.listIdx)
		}
		return
	}
	t.Skip("접힐 만큼 안 걸린다")
}

// 카탈로그가 꺼져 있으면 검색 중에 그 말을 한다. 조용히 라이브러리만
// 걸러 놓으면 왜 그런지 알 방법이 없다.
func TestSaysWhenCatalogIsOff(t *testing.T) {
	m := New()
	m.synced, m.bodyH = true, 26
	m = m.Filter("oasis").(Model) // cat 은 nil 그대로
	out := ansiOff(m.View(78, 26))
	if !strings.Contains(out, "Apple Music search is off") {
		t.Error("카탈로그가 꺼져 있는데 아무 말도 안 한다")
	}
	// 검색 중이 아닐 때는 말하지 않는다. 안 쓸 기능의 실패를 읽게 할 이유가 없다.
	idle := New()
	idle.synced, idle.bodyH = true, 26
	if strings.Contains(ansiOff(idle.View(78, 26)), "Apple Music search is off") {
		t.Error("검색 중도 아닌데 설정 이야기를 한다")
	}
}

var reAnsiOff = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func ansiOff(s string) string { return reAnsiOff.ReplaceAllString(s, "") }

// 구역 머리글은 목록에 묻히면 안 된다 — 글자 뒤로 선을 끌고, 구역끼리 띄운다.
func TestSectionHeadersStandOut(t *testing.T) {
	m := searching(t, "live", 25)
	rows := m.rows()

	var gaps int
	for i, r := range rows {
		if r.gap {
			gaps++
			if r.selectable() {
				t.Error("빈 줄에 커서가 선다")
			}
			if i+1 >= len(rows) || rows[i+1].header == "" {
				t.Error("빈 줄 다음이 머리글이 아니다")
			}
		}
	}
	if gaps != 1 {
		t.Errorf("구역 사이 빈 줄이 %d개 — 하나여야 한다", gaps)
	}

	// 머리글은 딥톤이라 곡 제목(밝은 회색)과 색이 다르다.
	//
	// 밝은 브랜드 색이 아닌 것도 함께 지킨다 — 그쪽은 "지금 소리가 나고
	// 있다"는 뜻이라(docs/03 11절), 상시로 뜨는 머리글이 쓰면 ▶ 가 옅어진다.
	const (
		deep   = "\x1b[38;2;168;37;63m"  // ColBrandDeep
		bright = "\x1b[38;2;255;90;117m" // ColBrand
	)
	for _, want := range []string{"Your Library · ", "Apple Music · "} {
		var found bool
		for _, l := range strings.Split(m.viewList(78, 20), "\n") {
			if !strings.Contains(ansiOff(l), want) {
				continue
			}
			found = true
			if !strings.Contains(l, deep) {
				t.Errorf("%q 머리글이 딥톤이 아니다", want)
			}
			if strings.Contains(l, bright) {
				t.Errorf("%q 머리글에 밝은 브랜드 색을 썼다", want)
			}
		}
		if !found {
			t.Errorf("%q 머리글이 없다", want)
		}
	}
}

// ── 설정 안내 ────────────────────────────────────────────────────────

// 화면은 환경변수 이름이 아니라 **다음 행동**을 말한다.
//
// 키를 놓으라는 것과 Team ID 를 적으라는 것은 사람이 할 일이 다르므로,
// 문구도 달라야 한다.
func TestSetupHintSaysNextStep(t *testing.T) {
	for _, c := range []struct {
		name string
		err  error
		want string
	}{
		{"아무것도 없음", nil, "/setup to connect"},
		{"Team ID 만 없음", applemusic.ErrNoTeamID, "/setup <team ID> to finish"},
	} {
		m := New()
		m.synced, m.bodyH = true, 26
		m.catErr = c.err
		mm := m.Filter("oasis").(Model)
		out := ansiOff(mm.View(78, 26))
		if !strings.Contains(out, c.want) {
			t.Errorf("%s — 화면에 %q 가 없다", c.name, c.want)
		}
		// 환경변수 이름을 화면에 대지 않는다. 그것은 구현 사정이다.
		if strings.Contains(out, "AM_P8") {
			t.Errorf("%s — 화면이 환경변수 이름을 댄다", c.name)
		}
	}
}

// /setup 은 인자가 없으면 무엇을 달라는지 말한다.
func TestSetupNeedsTeamID(t *testing.T) {
	var found bool
	for _, c := range New().Commands() {
		if c.Name != "/setup" {
			continue
		}
		found = true
		if c.Arg == "" {
			t.Error("/setup 이 인자를 요구하지 않는다")
		}
	}
	if !found {
		t.Fatal("/setup 명령이 없다")
	}
}

// ── AI 스위치 ────────────────────────────────────────────────────────

// 키가 없으면 요청을 아예 안 낸다. 빈 키로 나가면 OpenAI 가 401 을 주는데,
// 그 문장은 사람이 할 일을 말해 주지 않는다.
func TestAskWithoutKeyDoesNotFire(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	if secrets.HasOpenAIKey() {
		t.Skip("이 기계 키체인에 실제 키가 들어 있다")
	}
	m := New()
	next, cmd := m.Update(app.AskMsg{Prompt: "조용한 거"})
	if cmd == nil {
		t.Fatal("아무 말도 안 했다 — 왜 안 되는지는 말해야 한다")
	}
	if next.(Model).ask.live {
		t.Error("키가 없는데 요청이 나갔다")
	}
	// 무엇을 하면 되는지를 말한다.
	if !strings.Contains(errAIOff.Error(), "/ai ") {
		t.Errorf("안내가 다음 행동을 안 말한다: %v", errAIOff)
	}
}

// /ai 는 인자가 없으면 무엇을 달라는지 말한다.
//
// 이름이 벤더가 아니라 역할인 것도 함께 지킨다. 프로바이더가 바뀔 때
// 사용자에게 보이는 이름까지 따라 바꾸지 않으려는 것이다.
func TestAIKeyCommandNeedsKey(t *testing.T) {
	var found bool
	for _, c := range New().Commands() {
		if c.Name == "/openai" {
			t.Error("명령 이름에 벤더가 박혀 있다 — /ai 여야 한다")
		}
		if c.Name != "/ai" {
			continue
		}
		found = true
		if c.Arg == "" {
			t.Error("/ai 가 인자를 요구하지 않는다")
		}
	}
	if !found {
		t.Fatal("/ai 명령이 없다")
	}
}

// 첫 화면은 꺼진 것을 꺼졌다고 적는다.
//
// 한때 이 줄이 모델 이름을 무조건 적었다. 모델 이름은 "붙어 있다"는 뜻이
// 아닌데 붙어 있는 것처럼 보였고, 그래서 키가 없는 사람이 들어가서 문장을
// 치고 나서야 안 되는 걸 알았다.
func TestFactsTellTheTruthAboutAI(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	if secrets.HasOpenAIKey() {
		t.Skip("이 기계 키체인에 실제 키가 들어 있다")
	}
	off := factOf(t, New(), "AI")
	if !strings.Contains(off, "off") {
		t.Errorf("키가 없는데 %q 라고 적는다", off)
	}
	// 켜는 법까지 적는다. 꺼진 것만 보여 주고 어떻게 켜는지 말하지 않으면
	// 첫 화면을 나가서 팔레트를 뒤져야 한다.
	if !strings.Contains(off, "/ai ") {
		t.Errorf("켜는 법이 없다: %q", off)
	}

	t.Setenv("OPENAI_API_KEY", "sk-test")
	on := factOf(t, New(), "AI")
	if strings.Contains(on, "off") {
		t.Errorf("키가 있는데 %q 라고 적는다", on)
	}
	// 켜져 있을 때는 벤더를 밝힌다 — 네 돈이 어디로 가는지는 알아야 한다.
	if !strings.Contains(on, "OpenAI") {
		t.Errorf("켜졌는데 어디에 붙었는지 안 적는다: %q", on)
	}
}

func factOf(t *testing.T, m Model, name string) string {
	t.Helper()
	for _, f := range m.Facts() {
		if f.Name == name {
			return f.Detail
		}
	}
	t.Fatalf("%q 줄이 없다", name)
	return ""
}

// 담긴 곡을 이름으로 찾으면 틀린다. 실측에서 카탈로그(kr)가 "로제"라고 준
// 곡을 Music.app 은 "ROSÉ" 로 적었고, 제목도 갈렸다. 그때 옛 경로는
// 담기까지 해놓고 "안 나타났다"고 말했다.
//
// 번호는 어긋나지 않는다.
func TestAddedSinceFindsTheNewTrackWithoutNames(t *testing.T) {
	before := map[string]bool{"AAA": true, "BBB": true}
	now := map[string]bool{"AAA": true, "BBB": true, "CCC": true}

	if got := addedSince(before, now); got != "CCC" {
		t.Errorf("addedSince = %q, want CCC", got)
	}
}

// 아직 안 왔으면 빈 값이다. 그것이 "기다려라"라는 뜻이고,
// 없는 곡을 아무거나 골라 트는 것보다 낫다.
func TestAddedSinceSaysNothingWhenNothingArrived(t *testing.T) {
	same := map[string]bool{"AAA": true}
	if got := addedSince(same, same); got != "" {
		t.Errorf("addedSince = %q, want empty", got)
	}
}

// 담기 **전에** 찍어야 한다. 뒤에 찍으면 차이가 비어 영원히 못 찾는다.
func TestAddedSinceIsEmptyIfTheSnapshotWasTakenTooLate(t *testing.T) {
	tooLate := map[string]bool{"AAA": true, "CCC": true}
	now := map[string]bool{"AAA": true, "CCC": true}
	if got := addedSince(tooLate, now); got != "" {
		t.Errorf("addedSince = %q — 늦게 찍은 스냅샷이 곡을 찾아냈다면 그게 더 이상하다", got)
	}
}

// 아직 안 켠 것이 팔레트 맨 앞에 온다.
//
// 팔레트는 여덟 줄에서 끊긴다. 첫 화면이 "Type / to set up" 이라고 시켜
// 놓고 `/` 를 치면 큐 조작 명령만 여덟 줄 나왔다 — 시킨 대로 했는데
// 시킨 것이 없는 화면이었다.
func TestOffCommandsComeFirst(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	if secrets.HasOpenAIKey() {
		t.Skip("이 기계 키체인에 실제 키가 들어 있다")
	}
	cmds := New().Commands()
	if len(cmds) == 0 {
		t.Fatal("명령이 없다")
	}
	if cmds[0].Name != "/ai" {
		var head []string
		for _, c := range cmds[:style.Min(8, len(cmds))] {
			head = append(head, c.Name)
		}
		t.Errorf("AI 가 꺼졌는데 /ai 가 맨 앞이 아니다: %v", head)
	}
	// 켜는 명령에는 무엇을 달라는지가 붙어 있어야 한다.
	if cmds[0].Arg == "" {
		t.Error("/ai 가 인자를 안 밝힌다")
	}
}

// 켜는 명령의 인자 표기는 한 곳에만 산다.
//
// 팔레트가 `<key>` 라고 적는데 첫 화면이 `<sk-…>` 라고 적으면 같은 것을
// 두 이름으로 부르게 된다.
func TestFixMatchesTheCommand(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	if secrets.HasOpenAIKey() {
		t.Skip("이 기계 키체인에 실제 키가 들어 있다")
	}
	m := New()
	var fix string
	for _, f := range m.Facts() {
		if f.Name == "AI" {
			fix = f.Fix
		}
	}
	for _, c := range m.Commands() {
		if c.Name == "/ai" {
			if want := c.Name + " " + c.Arg; fix != want {
				t.Errorf("첫 화면은 %q, 팔레트는 %q", fix, want)
			}
			return
		}
	}
	t.Fatal("/ai 명령이 없다")
}
