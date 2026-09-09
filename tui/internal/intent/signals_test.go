package intent

import (
	"strings"
	"testing"
	"time"

	"amcli/tui/internal/api"
	"amcli/tui/internal/data"
)

// 플레이어가 이미 세고 있는 것을 모델에게 준다.
//
// 이 값들은 우리가 모으는 것이 아니라 Music.app 이 세는 것이다. 그래서
// 수집을 시작하기 전에도 오늘부터 쓸 수 있다.

func signalTrack(title string, plays, skips int, fav bool) api.Track {
	return api.Track{
		Id: 1, Title: title, Artist: api.Artist{Name: "A"},
		PlayCount: plays, SkipCount: skips, Favorited: fav,
	}
}

func TestSignalsCell(t *testing.T) {
	for _, c := range []struct {
		name  string
		track api.Track
		react data.Reaction
		want  string
	}{
		{"아무것도 없으면", signalTrack("a", 3, 0, false), data.Reaction{}, "-"},
		{"스킵만", signalTrack("b", 3, 2, false), data.Reaction{}, "2 skips"},
		{"좋아요만", signalTrack("c", 3, 0, true), data.Reaction{}, "favorite"},
		{"둘 다", signalTrack("d", 3, 4, true), data.Reaction{}, "4 skips, favorite"},

		// 우리가 본 것. Music.app 의 카운터는 왜 끝났는지를 모른다 —
		// 끝까지 들은 것과 3초 만에 나간 것이 같은 한 번으로 들어간다.
		{"일찍 나간 것", signalTrack("e", 3, 0, false), data.Reaction{Early: 2}, "2 dropped early"},
		{"끝까지 들은 것", signalTrack("f", 3, 0, false), data.Reaction{Done: 5}, "5 finished"},
		{"큐에서 뺀 것", signalTrack("g", 3, 0, false), data.Reaction{Removed: 1}, "1 removed"},
		{"플레이어 것과 우리 것", signalTrack("h", 3, 2, true), data.Reaction{Early: 3, Done: 1},
			"2 skips, favorite, 3 dropped early, 1 finished"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := signals(c.track, c.react); got != c.want {
				t.Errorf("%q, 원한 것 %q", got, c.want)
			}
		})
	}
}

// 값이 없을 때 빈 칸으로 두면 칸이 밀려 다음 값이 이 자리로 읽힌다.
func TestEmptySignalsStillTakesItsColumn(t *testing.T) {
	out := renderLibrary([]api.Track{signalTrack("quiet", 3, 0, false)}, nil, time.Now()).text
	line := lastLine(out)
	if got := strings.Count(line, "|"); got != 10 {
		t.Errorf("칸이 %d개다 — 머리글이 말한 11칸이어야 한다:\n%s", got+1, line)
	}
	if !strings.Contains(line, "| - |") {
		t.Errorf("빈 신호가 자리를 안 지킨다:\n%s", line)
	}
}

// 머리글이 실제 칸과 맞아야 한다. 어긋나면 모델이 값을 한 칸씩 밀려 읽는다.
func TestHeaderMatchesTheColumns(t *testing.T) {
	out := renderLibrary([]api.Track{signalTrack("x", 1, 1, true)}, nil, time.Now()).text
	head := strings.SplitN(out, "\n", 2)[0]
	if !strings.Contains(head, "signals") {
		t.Errorf("머리글에 signals 가 없다:\n%s", head)
	}
	if got, want := strings.Count(head, "|"), strings.Count(lastLine(out), "|"); got != want {
		t.Errorf("머리글은 %d칸, 곡 줄은 %d칸", got+1, want+1)
	}
}

// **별점은 넣지 않는다.**
//
// 실측에서 202곡 전부 0 이었다 — Music.app 이 값을 돌려주지 않는다. 없는 값을
// 칸으로 만들면 모델이 "아무도 별점을 안 줬다"는 사실이 아닌 것을 읽는다.
// api.Track 에 필드가 있어서 넣고 싶어지는 자리라 못 박아 둔다.
func TestRatingIsNotShown(t *testing.T) {
	five := 5
	tr := signalTrack("rated", 3, 0, false)
	tr.Rating = &five

	out := renderLibrary([]api.Track{tr}, nil, time.Now()).text
	for _, bad := range []string{"rating", "star", "★"} {
		if strings.Contains(strings.ToLower(out), bad) {
			t.Errorf("별점을 내보낸다 (%q) — 실측에서 언제나 0인 값이다", bad)
		}
	}
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	return lines[len(lines)-1]
}

// 우리가 본 것을 머리글이 설명해야 한다.
//
// 값만 주고 뜻을 안 알려주면 "3 dropped early" 가 무슨 뜻인지 모델이
// 지어낸다. 칸 이름과 뜻은 늘 같이 간다.
func TestHeaderExplainsWhatWeObserved(t *testing.T) {
	tr := signalTrack("x", 1, 0, false)
	out := renderLibrary([]api.Track{tr}, map[int64]data.Reaction{tr.Id: {Early: 2}}, time.Now()).text
	head := strings.SplitN(out, "\n\n", 2)[0]
	for _, want := range []string{"dropped early", "finished", "removed"} {
		if !strings.Contains(head, want) {
			t.Errorf("머리글이 %q 를 설명하지 않는다:\n%s", want, head)
		}
	}
}

// **증거이지 판정이 아니다.**
//
// 이 문장이 빠지면 모델은 한 번 넘긴 곡을 영영 안 튼다. 사람은 좋아하는
// 곡도 그 순간에 안 맞으면 넘긴다 — 암묵적 피드백에 진짜 "싫다" 는 없다.
func TestGuidanceSaysItIsEvidenceNotAVerdict(t *testing.T) {
	tr := signalTrack("x", 1, 0, false)
	out := renderLibrary([]api.Track{tr}, map[int64]data.Reaction{tr.Id: {Early: 2}}, time.Now()).text
	head := strings.SplitN(out, "\n\n", 2)[0]
	if !strings.Contains(head, "evidence, not a verdict") {
		t.Error("증거일 뿐이라는 말이 없다 — 한 번 넘긴 곡이 영영 사라진다")
	}
	if !strings.Contains(head, "never play it again") {
		t.Error("금지가 아니라는 말이 없다")
	}
}

// 기록이 없어도 선곡은 돈다. 첫날에는 아무 기록도 없다.
func TestNoRecordIsNotAnError(t *testing.T) {
	out := renderLibrary([]api.Track{signalTrack("x", 1, 0, false)}, nil, time.Now()).text
	if !strings.Contains(lastLine(out), "| - |") {
		t.Errorf("기록이 없는데 칸이 안 지켜졌다:\n%s", lastLine(out))
	}
}
