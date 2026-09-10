package data

import (
	"os"
	"path/filepath"
	"testing"
)

// 옛 줄에는 turnId 가 없다. 마이그레이션 없이 그대로 읽혀야 한다 —
// 없으면 0 이고, 0 은 "요청 없이 튼 곡"이라는 뜻이다.
func TestPlaysOldLinesStillRead(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AMCLI_STATE_DIR", dir)
	old := `{"at":"2026-09-09T12:00:00Z","trackId":41,"playedMs":12000,"durMs":215000,"endedBy":"skipped","context":"Calm First-Listen Hour"}
{"at":"2026-09-10T12:00:00Z","trackId":42,"playedMs":9000,"durMs":200000,"endedBy":"skipped","turnId":1789003401854834000}
`
	if err := os.WriteFile(filepath.Join(dir, "plays.jsonl"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPlays()
	if err != nil || len(got) != 2 {
		t.Fatalf("두 줄을 못 읽었다: %v %v", got, err)
	}
	if got[0].TurnID != 0 || got[0].Context != "Calm First-Listen Hour" {
		t.Fatalf("옛 줄이 깨졌다: %+v", got[0])
	}
	if got[1].TurnID == 0 {
		t.Fatalf("새 줄의 번호를 못 읽었다: %+v", got[1])
	}
}
