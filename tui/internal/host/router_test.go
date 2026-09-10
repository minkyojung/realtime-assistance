package host

import (
	"context"
	"os"
	"strings"
	"testing"
)

// 라우터는 사실상 순수 함수다 — (문장, 앱 설명들, 지금 앱) → 앱 이름 목록.
// 그래서 슬랙을 만들지 않고도 가짜 설명만으로 검증할 수 있다.
var fakeApps = []Spec{
	{"music", "Plays music from the user's own Apple Music library. " +
		"Handles what to listen to, building or adjusting a play queue, " +
		"finding tracks, and playback control."},
	{"chat", "Reads and sends messages in the user's workspace. " +
		"Handles who messaged, replying, and setting status or do-not-disturb."},
}

func TestRoute(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("OPENAI_API_KEY 가 없어 건너뜁니다")
	}

	for _, c := range []struct {
		prompt string
		want   []string
	}{
		{"조용한 거", []string{"music"}},
		{"누가 나 찾았어?", []string{"chat"}},
		{"something quiet, nothing I've heard too much", []string{"music"}},
		{"조용한 거 틀고 알림도 꺼줘", []string{"music", "chat"}},
		{"다음 곡", []string{"music"}},
		{"팀한테 늦는다고 보내줘", []string{"chat"}},
	} {
		got, err := Route(context.Background(), c.prompt, fakeApps, "music")
		if err != nil {
			t.Fatalf("%q: %v", c.prompt, err)
		}
		if !sameSet(got, c.want) {
			t.Errorf("%q → %v, 원하는 것 %v", c.prompt, got, c.want)
		}
	}
}

// 앱이 하나면 라우터를 부르지 않는다 — 답이 정해져 있다.
func TestRouteSkipsWhenSingleApp(t *testing.T) {
	got, err := Route(context.Background(), "아무 말이나", fakeApps[:1], "music")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "music" {
		t.Errorf("앱이 하나인데 %v 를 돌려줬다", got)
	}
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]bool{}
	for _, x := range a {
		seen[strings.ToLower(x)] = true
	}
	for _, x := range b {
		if !seen[strings.ToLower(x)] {
			return false
		}
	}
	return true
}
