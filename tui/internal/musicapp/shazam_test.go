package musicapp

import (
	"strings"
	"testing"

	"amcli/tui/internal/api"
	"amcli/tui/internal/app"
	"amcli/tui/internal/shazam"
	tea "charm.land/bubbletea/v2"
)

func heard(title, artist string) shazam.Result {
	return shazam.Result{OK: true, Title: title, Artist: artist, AppleMusicID: "a1"}
}

// 인식 결과가 대조까지 마치고 앉아야 한다.
// 대조를 빠뜨리면 화면에 "안 가진 곡"으로 그려진다.
func TestRecognizedMarksLibrary(t *testing.T) {
	if !inLibrary(recognized(heard("Holocene", "Bon Iver"))) {
		t.Error("가진 곡을 못 알아봤다")
	}
	if inLibrary(recognized(heard("존재하지 않는 곡", "Nobody"))) {
		t.Error("없는 곡을 가졌다고 한다")
	}
}

// S5 가 적는 값을 잃어버리면 안 된다 — docs/06.
func TestRecognizedCarriesFields(t *testing.T) {
	res := heard("Holocene", "Bon Iver")
	res.ISRC, res.Genre, res.Year = "US123", "Alternative", 2011
	ct := recognized(res)
	if ct.Isrc == nil || *ct.Isrc != "US123" {
		t.Error("ISRC 가 사라졌다")
	}
	if ct.Year == nil || *ct.Year != 2011 {
		t.Error("연도가 사라졌다")
	}
	if ct.Genre == nil || *ct.Genre != "Alternative" {
		t.Error("장르가 사라졌다")
	}
}

// 이 서비스가 더 말할 수 있는 것은 곡 이름이 아니라 그 곡과 나의 관계다.
func TestRecognizedLine(t *testing.T) {
	never := recognizedLine(recognized(heard("Jirisan Breeze", "Akimbo")))
	if !strings.Contains(never, "never played") {
		t.Errorf("담아두고 안 들은 곡인데 그 말이 없다: %q", never)
	}
	played := recognizedLine(recognized(heard("Blood Bank", "Bon Iver")))
	if !strings.Contains(played, "played 3 times") {
		t.Errorf("들은 횟수가 없다: %q", played)
	}
	outside := recognizedLine(recognized(heard("존재하지 않는 곡", "Nobody")))
	if !strings.Contains(outside, "not in Your Library") {
		t.Errorf("미보유라고 말하지 않는다: %q", outside)
	}
}

// 한 번 알아맞히면 섹션이 생기고, 화면이 그리로 옮겨간다.
func TestRecognitionOpensSection(t *testing.T) {
	m := New()
	if hasSection(m, secShazam) {
		t.Fatal("아직 아무것도 안 들었는데 섹션이 있다")
	}

	next, _ := m.Update(shazamMsg{res: heard("Holocene", "Bon Iver")})
	m = next.(Model)

	if !hasSection(m, secShazam) {
		t.Fatal("알아맞혔는데 섹션이 없다")
	}
	if m.sections[m.sectionIdx].kind != secShazam {
		t.Error("결과를 보여주지 않는다 — 다른 섹션에 서 있다")
	}
	rows := m.rows()
	if len(rows) != 1 || rows[0].catalog == nil || rows[0].catalog.Title != "Holocene" {
		t.Fatalf("목록에 인식 결과가 없다: %+v", rows)
	}

	// 새것이 위다. 방금 무엇이 흐르는가를 묻는 일이므로.
	next, _ = m.Update(shazamMsg{res: heard("Blood Bank", "Bon Iver")})
	m = next.(Model)
	if got := m.rows()[0].catalog.Title; got != "Blood Bank" {
		t.Errorf("마지막 답이 맨 위가 아니다: %q", got)
	}
}

// 못 알아들은 것은 고장이 아니다. 빨간 줄로 말하면 사용자가 고장난 줄 안다.
func TestNoMatchIsNotAnError(t *testing.T) {
	_, cmd := New().Update(shazamMsg{err: shazam.ErrNoMatch})
	said, ok := runCmd(cmd).(app.SayMsg)
	if !ok {
		t.Fatalf("아무 말도 안 했다: %T", runCmd(cmd))
	}
	if said.Err {
		t.Error("못 알아들은 것을 실패로 말한다")
	}
}

// 마이크 권한이 없는 것은 사용자가 고칠 수 있는 실패다. 그렇게 말해야 한다.
func TestMicrophoneDeniedIsAnError(t *testing.T) {
	_, cmd := New().Update(shazamMsg{err: shazam.ErrMicDenied})
	said, ok := runCmd(cmd).(app.SayMsg)
	if !ok || !said.Err {
		t.Errorf("권한 실패를 실패로 말하지 않는다: %+v", said)
	}
}

// 듣는 동안 또 부르면 마이크를 두 번 쥔다.
func TestSecondListenIsRefused(t *testing.T) {
	m := New()
	next, _ := m.Update(shazamStartedMsg{})
	m = next.(Model)
	if _, ok := runCmd(m.shazamCmd("")).(errMsg); !ok {
		t.Error("듣는 중에 또 듣기 시작한다")
	}
}

func hasSection(m Model, k sectionKind) bool {
	for _, s := range m.sections {
		if s.kind == k {
			return true
		}
	}
	return false
}

// runCmd — Cmd 하나를 실행해 메시지를 꺼낸다. Batch 면 첫 것을 본다.
func runCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok && len(batch) > 0 {
		return batch[0]()
	}
	return msg
}

func hasCommand(m Model, name string) bool {
	for _, c := range m.Commands() {
		if c.Name == name {
			return true
		}
	}
	return false
}

// 헬퍼가 없으면 /shazam 이 팔레트에 없어야 한다. 배포판이 그 상태다 —
// 못 하는 일을 올려 두면 눌러 본 사람이 빨간 줄을 받는다.
func TestShazamHiddenWithoutHelper(t *testing.T) {
	m := New()
	if hasCommand(m, "/shazam") {
		t.Error("헬퍼를 확인하기도 전에 /shazam 이 나왔다")
	}

	next, _, handled := m.applyShazam(shazamReadyMsg{ok: false})
	if !handled {
		t.Fatal("shazamReadyMsg 를 아무도 안 받았다")
	}
	if hasCommand(next.(Model), "/shazam") {
		t.Error("헬퍼가 없는데 /shazam 이 나왔다")
	}
}

// 있으면 그대로 나와야 한다. 개발 빌드의 동작은 바뀌지 않는다.
func TestShazamAppearsWithHelper(t *testing.T) {
	next, _, _ := New().applyShazam(shazamReadyMsg{ok: true})
	if !hasCommand(next.(Model), "/shazam") {
		t.Error("헬퍼가 있는데 /shazam 이 안 나왔다")
	}
}

// 알아맞힌 곡을 앉히고 나서 **말은 하지 않는다.** 담긴 곡인지 Apple 에게
// 묻고, 답이 온 뒤에 말한다. 짐작한 문장을 먼저 띄우면 사용자는 틀린 말을
// 먼저 읽는다.
func TestRecognitionWaitsForAppleBeforeSpeaking(t *testing.T) {
	m := New()
	mm, cmd, handled := m.applyRecognition(heard("Holocene", "Bon Iver"))
	if !handled {
		t.Fatal("처리되지 않았다")
	}
	if len(mm.(Model).shzHits) != 1 {
		t.Fatal("줄이 안 앉았다")
	}
	if cmd == nil {
		t.Fatal("아무것도 묻지 않았다")
	}
	// 못 물을 때(m.cat == nil)도 말은 나와야 한다 — 짐작한 채로.
	msg := cmd()
	marked, ok := msg.(shazamMarkedMsg)
	if !ok {
		t.Fatalf("돌아온 것이 %T", msg)
	}
	if _, _, handled := m.applyShazam(marked); !handled {
		t.Error("답을 못 받았다")
	}
}

// Apple 이 담겼다고 하는데 로컬에서 그 줄을 못 찾는 경우가 있다 —
// 이름이 어긋난 것이지 없는 것이 아니다. 그때 "없다"고 말하면 거짓이고,
// 재생 횟수를 지어내면 더 나쁘다.
func TestRecognizedLineTrustsAppleOverTheNameGuess(t *testing.T) {
	yes := true
	ct := api.CatalogTrack{
		AppleMusicId: "1",
		Title:        "어느 라이브러리에도 없는 제목",
		ArtistName:   "아무개",
		InLibrary:    &yes,
	}
	line := recognizedLine(ct)
	if strings.Contains(line, "not in Your Library") {
		t.Errorf("Apple 이 담겼다고 했는데 없다고 말한다: %q", line)
	}
	if strings.Contains(line, "played") {
		t.Errorf("못 찾은 곡의 재생 횟수를 지어냈다: %q", line)
	}
}

// 반대도 같다. Apple 이 없다고 하면 이름이 우연히 맞아도 없는 것이다.
func TestRecognizedLineTrustsAppleWhenItSaysNo(t *testing.T) {
	no := false
	ct := api.CatalogTrack{
		AppleMusicId: "1", Title: "Holocene", ArtistName: "Bon Iver", InLibrary: &no,
	}
	if line := recognizedLine(ct); !strings.Contains(line, "not in Your Library") {
		t.Errorf("Apple 이 없다고 했는데 있다고 말한다: %q", line)
	}
}
