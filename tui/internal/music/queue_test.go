package music

import (
	"strings"
	"testing"
)

// 이 테스트는 스크립트 **문자열**만 본다. 실제로 돌리는 테스트는 쓸 수 없다 —
// 돌리는 순간 이 기계의 진짜 음악 라이브러리를 고치기 때문이다.
//
// 그래서 여기서 지키는 것은 하나다. **되돌릴 수 없는 문장이 섞이지 않았는가.**

// 첫 실행에는 지울 것이 없다. 아무것도 지우지 않아야 한다.
func TestFirstQueueDeletesNothing(t *testing.T) {
	s := replaceQueueScript("", []string{"AAAA1111"})
	if strings.Contains(s, "delete") {
		t.Errorf("지울 것이 없는데 delete 가 들어 있다:\n%s", s)
	}
}

// 우리가 만든 것만 지운다.
//
// 이름으로 지우면 사용자가 우연히 같은 이름으로 만든 플레이리스트를 날린다.
// persistent ID 를 기억해 두는 이유가 이것뿐이다.
func TestQueueDeletesOnlyByPersistentID(t *testing.T) {
	s := replaceQueueScript("BBBB2222", []string{"AAAA1111"})

	if !strings.Contains(s, `delete (first user playlist whose persistent ID is "BBBB2222")`) {
		t.Errorf("우리 플레이리스트를 ID 로 지우지 않는다:\n%s", s)
	}
	// 지우는 문장에 이름이 등장하면 안 된다.
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, "delete") && strings.Contains(line, QueuePlaylistName) {
			t.Errorf("이름으로 지우는 문장이 있다: %q", strings.TrimSpace(line))
		}
	}
}

// 지우는 것은 플레이리스트이지 곡이 아니다.
//
// Music.app 에서 라이브러리의 곡을 지우면 파일까지 사라진다. 그 문장은
// 이 패키지 어디에도 있으면 안 된다.
func TestQueueNeverDeletesTracks(t *testing.T) {
	s := replaceQueueScript("BBBB2222", []string{"AAAA1111", "CCCC3333"})

	for _, forbidden := range []string{
		"delete track",
		"delete every track",
		"delete (every track",
		"delete (first track",
		"library playlist 1)",
	} {
		if strings.Contains(s, forbidden) {
			t.Errorf("곡을 지우는 문장이 있다: %q\n%s", forbidden, s)
		}
	}
}

// 담은 곡이 순서대로 다 들어가야 한다. 하나라도 빠지면 화면의 큐와
// Music.app 의 큐가 어긋나고, 그때부터 번호를 믿을 수 없게 된다.
func TestQueueScriptCarriesEveryTrackInOrder(t *testing.T) {
	ids := []string{"AAAA1111", "BBBB2222", "CCCC3333"}
	s := replaceQueueScript("", ids)

	at := -1
	for _, id := range ids {
		i := strings.Index(s, `"`+id+`"`)
		if i < 0 {
			t.Fatalf("%s 가 스크립트에 없다", id)
		}
		if i < at {
			t.Errorf("%s 가 순서에서 벗어났다", id)
		}
		at = i
	}
	if !strings.Contains(s, "return persistent ID of pl") {
		t.Error("새 플레이리스트의 ID 를 돌려주지 않는다 — 다음번에 지울 대상을 잃는다")
	}
}

// 곡 빼기는 통째로 다시 쓰지 않는다. 다시 쓰면 듣던 곡까지 사라져 음악이 끊긴다.
func TestRemoveTouchesOneTrackOnly(t *testing.T) {
	s := removeQueueTrackScript("BBBB2222", 3, false)

	if strings.Contains(s, "make new user playlist") {
		t.Errorf("한 곡 빼자고 플레이리스트를 새로 만든다:\n%s", s)
	}
	if !strings.Contains(s, "delete track 3 of pl") {
		t.Errorf("3번 곡을 지우지 않는다:\n%s", s)
	}
}

// 지우는 대상은 언제나 우리 플레이리스트를 거쳐서 가리켜야 한다.
//
// 라이브러리 쪽으로 새면 파일이 사라진다. 이 테스트가 그 한 줄을 지킨다.
func TestRemoveNeverReachesTheLibrary(t *testing.T) {
	s := removeQueueTrackScript("BBBB2222", 3, true)

	if !strings.Contains(s, `set pl to (first user playlist whose persistent ID is "BBBB2222")`) {
		t.Fatalf("우리 플레이리스트를 ID 로 잡지 않는다:\n%s", s)
	}
	for _, line := range strings.Split(s, "\n") {
		if !strings.Contains(line, "delete") {
			continue
		}
		if !strings.HasSuffix(strings.TrimSpace(line), "of pl") {
			t.Errorf("우리 플레이리스트를 거치지 않는 delete 다: %q", strings.TrimSpace(line))
		}
		if strings.Contains(line, "library") {
			t.Errorf("라이브러리를 가리키는 delete 다: %q", strings.TrimSpace(line))
		}
	}
}

// 지금 나오는 곡을 빼면 다음 곡으로 넘어간다. 조용해지는 것이 아니다.
func TestRemovingTheCurrentTrackAdvancesFirst(t *testing.T) {
	with := removeQueueTrackScript("BBBB2222", 1, true)
	without := removeQueueTrackScript("BBBB2222", 1, false)

	if !strings.Contains(with, "next track") {
		t.Error("지금 나오는 곡을 빼는데 다음 곡으로 안 넘긴다")
	}
	if strings.Contains(without, "next track") {
		t.Error("안 나오는 곡을 빼는데 재생을 건드린다")
	}
	// 넘긴 다음에 지워야 한다. 지우고 넘기면 그 사이에 무엇이 나올지 모른다.
	if strings.Index(with, "next track") > strings.Index(with, "delete") {
		t.Error("지우고 나서 넘긴다 — 순서가 반대다")
	}
}
