package music

import (
	"strings"
	"testing"
)

// 스크립트만 본다. 실제로 돌리면 남의 라이브러리를 고친다.

// 지우는 것은 언제나 **우리 플레이리스트의 곡**이다.
//
// 라이브러리 쪽으로 새면 파일이 사라지고 되돌릴 수 없다. 이 파일에서 가장
// 중요한 한 줄이라 따로 지킨다.
func TestTailRewriteNeverTouchesTheLibrary(t *testing.T) {
	s := rewriteQueueTailScript("PID", 2, []string{"A", "B"})
	for _, bad := range []string{"delete track i of library", "delete (first track of library"} {
		if strings.Contains(s, bad) {
			t.Fatalf("라이브러리를 지우는 말이 들어 있다: %s", bad)
		}
	}
	if !strings.Contains(s, "delete track i of pl") {
		t.Error("우리 플레이리스트를 거쳐서 지우지 않는다")
	}
}

// 앞은 건드리지 않는다. 지금 나오는 곡이 살아남아야 음악이 안 끊긴다.
func TestTailRewriteStartsWhereItIsTold(t *testing.T) {
	if s := rewriteQueueTailScript("PID", 3, nil); !strings.Contains(s, "to 3 by -1") {
		t.Errorf("3번째부터 지워야 하는데:\n%s", s)
	}
}

// 뒤에서부터 지운다. 앞에서 지우면 자리번호가 밀려 다음 바퀴가 엉뚱한 줄을 본다.
func TestTailRewriteDeletesBackwards(t *testing.T) {
	s := rewriteQueueTailScript("PID", 2, nil)
	if !strings.Contains(s, "from (count of tracks of pl) to 2 by -1") {
		t.Errorf("뒤에서부터 지우지 않는다:\n%s", s)
	}
}

// 지우기가 붙이기보다 먼저다. 순서가 뒤집히면 방금 붙인 것을 도로 지운다.
func TestTailRewriteDeletesBeforeItAppends(t *testing.T) {
	s := rewriteQueueTailScript("PID", 2, []string{"A"})
	del, add := strings.Index(s, "delete track i of pl"), strings.Index(s, "duplicate")
	if del < 0 || add < 0 {
		t.Fatalf("지우기나 붙이기가 없다:\n%s", s)
	}
	if del > add {
		t.Error("붙인 뒤에 지운다 — 방금 붙인 것이 사라진다")
	}
}

// 빈 목록이면 지우기만 한다. 붙일 것이 없는데 repeat 를 열면 문법이 깨진다.
func TestTailRewriteWithNothingToAddOnlyDeletes(t *testing.T) {
	if s := rewriteQueueTailScript("PID", 2, nil); strings.Contains(s, "duplicate") {
		t.Errorf("붙일 것이 없는데 붙이는 말이 있다:\n%s", s)
	}
}

// 플레이리스트를 지우지 않는다. 그것이 ReplaceQueue 와 갈리는 지점이다.
func TestTailRewriteKeepsThePlaylist(t *testing.T) {
	s := rewriteQueueTailScript("PID", 2, []string{"A"})
	if strings.Contains(s, "delete (first user playlist") {
		t.Error("플레이리스트를 통째로 지운다 — 그러면 음악이 끊긴다")
	}
	if strings.Contains(s, "make new user playlist") {
		t.Error("플레이리스트를 새로 만든다 — 그러면 음악이 끊긴다")
	}
}

// 못 믿을 인자는 Music.app 까지 가지 않는다.
func TestTailRewriteRefusesBadArguments(t *testing.T) {
	if err := RewriteQueueTail("", 1, []string{"A"}); err == nil {
		t.Error("플레이리스트를 모르는데 받아들였다")
	}
	if err := RewriteQueueTail("PID", 0, []string{"A"}); err == nil {
		t.Error("0번째부터라는 말을 받아들였다 — 자리번호는 1부터다")
	}
}
