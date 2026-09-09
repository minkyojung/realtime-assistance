package intent

import (
	"strings"
	"testing"
	"time"

	"amcli/tui/internal/api"
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
		want  string
	}{
		{"아무것도 없으면", signalTrack("a", 3, 0, false), "-"},
		{"스킵만", signalTrack("b", 3, 2, false), "2 skips"},
		{"좋아요만", signalTrack("c", 3, 0, true), "favorite"},
		{"둘 다", signalTrack("d", 3, 4, true), "4 skips, favorite"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := signals(c.track); got != c.want {
				t.Errorf("%q, 원한 것 %q", got, c.want)
			}
		})
	}
}

// 값이 없을 때 빈 칸으로 두면 칸이 밀려 다음 값이 이 자리로 읽힌다.
func TestEmptySignalsStillTakesItsColumn(t *testing.T) {
	out := renderLibrary([]api.Track{signalTrack("quiet", 3, 0, false)}, time.Now()).text
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
	out := renderLibrary([]api.Track{signalTrack("x", 1, 1, true)}, time.Now()).text
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

	out := renderLibrary([]api.Track{tr}, time.Now()).text
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
