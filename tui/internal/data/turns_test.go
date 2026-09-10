package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTurnsRoundTrip(t *testing.T) {
	t.Setenv("AMCLI_STATE_DIR", t.TempDir())

	want := Turn{ID: NewTurnID(), At: time.Now(), Prompt: "공부할 때 들을 거", Context: "focus", Title: "Calm Hour"}
	if err := AppendTurn(want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadTurns()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != want.ID || got[0].Prompt != want.Prompt || got[0].Context != "focus" {
		t.Fatalf("돌아온 것이 다르다: %+v", got)
	}
	if TurnsByID(got)[want.ID].Title != "Calm Hour" {
		t.Fatal("번호로 못 찾는다")
	}
}

// 없는 파일은 오류가 아니다. 아직 아무것도 안 물어본 상태다.
func TestTurnsMissingFile(t *testing.T) {
	t.Setenv("AMCLI_STATE_DIR", t.TempDir())
	got, err := LoadTurns()
	if err != nil || got != nil {
		t.Fatalf("빈 상태를 오류로 읽었다: %v %v", got, err)
	}
}

// 쓰다 만 줄 하나 때문에 지난 기록을 통째로 잃지 않는다.
func TestTurnsSkipsBrokenLine(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AMCLI_STATE_DIR", dir)
	good, _ := json.Marshal(Turn{ID: 2, Prompt: "b"})
	body := append([]byte("{\"id\":1,\"prompt\"\n"), append(good, '\n')...)
	if err := os.WriteFile(filepath.Join(dir, "turns.jsonl"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadTurns()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("성한 줄을 못 건졌다: %+v", got)
	}
}

// 번호는 부를 때마다 커진다. 같은 값이 두 번 나오면 요청 둘이 한 줄로 접힌다.
func TestNewTurnIDIncreases(t *testing.T) {
	a, b := NewTurnID(), NewTurnID()
	if b <= a {
		t.Fatalf("번호가 안 늘었다: %d → %d", a, b)
	}
}
