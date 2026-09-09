package lyrics

import (
	"testing"
	"time"
)

const sample = `[ar:Jack Johnson]
[00:12.50] Well I was sitting, waiting, wishing
[00:15.80] You believed in superstitions
[01:02.00] Must I always be waiting
[00:20.123] Then maybe you'd see the signs
[02:16] Oh, oh, oh
`

func TestParseLRC(t *testing.T) {
	l := &Lyrics{Lines: ParseLRC(sample)}
	if !l.Synced() {
		t.Fatal("싱크 가사로 안 읽혔다")
	}
	if len(l.Lines) != 5 {
		t.Fatalf("줄 수 %d, 기대 5 — [ar:] 머리표는 버려야 한다", len(l.Lines))
	}
	// 원본이 뒤섞여 있어도 시각순으로 서야 한다.
	for i := 1; i < len(l.Lines); i++ {
		if l.Lines[i].At < l.Lines[i-1].At {
			t.Fatal("시각순이 아니다")
		}
	}
	if got := l.Lines[0].At; got != 12500*time.Millisecond/1000*1000/1000 {
		// 12.50초
		if got != 12*time.Second+500*time.Millisecond {
			t.Errorf("첫 줄 시각 %v", got)
		}
	}
	// 세 자리 소수는 1/1000초다.
	if got := l.Lines[2].At; got != 20*time.Second+123*time.Millisecond {
		t.Errorf("소수 세 자리를 잘못 읽었다: %v", got)
	}
	// 소수가 없어도 읽어야 한다.
	if got := l.Lines[4].At; got != 2*time.Minute+16*time.Second {
		t.Errorf("소수 없는 시각을 잘못 읽었다: %v", got)
	}
}

func TestAt(t *testing.T) {
	l := &Lyrics{Lines: ParseLRC(sample)}
	for _, c := range []struct {
		pos  time.Duration
		want int
	}{
		{0, -1},                             // 아직 첫 줄 전
		{12 * time.Second, -1},              // 첫 줄 직전
		{13 * time.Second, 0},               //
		{16 * time.Second, 1},               //
		{2*time.Minute + 20*time.Second, 4}, // 마지막 줄
		{9 * time.Minute, 4},                // 끝나고도 마지막 줄에 머문다
	} {
		if got := l.At(c.pos); got != c.want {
			t.Errorf("%v → %d, 기대 %d", c.pos, got, c.want)
		}
	}
}

// 시각이 없으면 줄글로 읽힌다. 하이라이트는 안 되지만 보여줄 수는 있다.
func TestPlainOnly(t *testing.T) {
	l := payload{Plain: "one\ntwo\n\nthree"}.lyrics()
	if l.Synced() {
		t.Error("시각이 없는데 싱크로 읽혔다")
	}
	if len(l.Plain) != 4 || l.Plain[0] != "one" {
		t.Errorf("줄글을 잘못 읽었다: %q", l.Plain)
	}
	if l.At(time.Minute) != -1 {
		t.Error("싱크가 아닌데 줄을 짚었다")
	}
}

func TestClean(t *testing.T) {
	for in, want := range map[string]string{
		"Wonderwall (Live at Knebworth, 11 August '96)": "Wonderwall",
		"Sitting, Waiting, Wishing - Jack Johnson":      "Sitting, Waiting, Wishing",
		"California Sober (feat. Chris Stapleton)":      "California Sober",
		"Perth": "Perth",
	} {
		if got := Clean(in); got != want {
			t.Errorf("%q → %q, 기대 %q", in, got, want)
		}
	}
}

func TestEmpty(t *testing.T) {
	if !(&Lyrics{}).Empty() || !(*Lyrics)(nil).Empty() {
		t.Error("빈 가사를 비었다고 안 한다")
	}
	if (&Lyrics{Plain: []string{"x"}}).Empty() {
		t.Error("줄글이 있는데 비었다고 한다")
	}
}
