package musicapp

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// 재생 제어는 shift+화살표 한 가족이다 — ← 이전, ↓ 재생/일시정지, → 다음.
//
// 알파벳은 못 쓴다. 입력창이 늘 활성이라 `s`=스킵을 두면 "sad 한 걸로"의
// 첫 글자가 먹힌다. space 도 못 쓴다 — 한글 조합 중에는 터미널이 preedit 을
// 앱에 넘기지 않아 입력창이 "비어 있음"으로 보이고, 조합을 끝내려고 누른
// space 가 일시정지로 새어 버린다(709a1fb).
//
// 수식키+화살표만 둘 다 피한다. 입력창이 글자로 받지 않으므로 앱까지 온다.
func TestShiftArrowsControlPlayback(t *testing.T) {
	for _, c := range []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{"이전 곡", tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModShift}},
		{"재생/일시정지", tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModShift}},
		{"다음 곡", tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift}},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, cmd := atSection(t, secRecent).Update(c.key)
			if cmd == nil {
				t.Errorf("%s — 아무 일도 일어나지 않았다", c.key.String())
			}
		})
	}
}

// 키는 속도용, 명령은 발견용이다. 팔레트에 없으면 있는 줄도 모른다 —
// tab(섹션 이동)과 /queue 가 공존하는 것과 같은 이유다.
func TestPauseIsAlsoACommand(t *testing.T) {
	for _, c := range New().Commands() {
		if c.Name == "/pause" {
			if c.Run == nil || c.Run("") == nil {
				t.Error("/pause 가 아무것도 하지 않는다")
			}
			return
		}
	}
	t.Error("팔레트에 /pause 가 없다")
}
