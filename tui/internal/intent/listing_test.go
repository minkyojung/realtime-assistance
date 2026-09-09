package intent

import (
	"testing"
	"time"

	"amcli/tui/internal/api"
)

func track(id int64, title string, excluded bool) api.Track {
	return api.Track{Id: id, Title: title, Artist: api.Artist{Name: "A"}, Excluded: excluded}
}

// 줄번호는 1부터 빈틈없이 이어져야 한다.
// 제외된 곡을 건너뛰므로 진짜 id 를 그대로 쓰면 번호에 구멍이 난다.
func TestListingNumbersAreContiguous(t *testing.T) {
	tracks := []api.Track{
		track(-2064959302851484466, "first", false),
		track(999, "excluded", true), // 건너뛴다
		track(8673039688227705491, "second", false),
	}
	lst := renderLibrary(tracks, nil, time.Now())

	if len(lst.ids) != 2 {
		t.Fatalf("목록 길이: got %d, want 2", len(lst.ids))
	}
	if lst.ids[0] != -2064959302851484466 || lst.ids[1] != 8673039688227705491 {
		t.Errorf("되돌리는 표가 틀렸다: %v", lst.ids)
	}
	// 19자리 id 가 본문에 새면 안 된다 — 그것을 감추는 것이 이 층의 목적이다.
	if want := "8673039688227705491"; contains(lst.text, want) {
		t.Errorf("진짜 id 가 프롬프트에 샜다")
	}
	if !contains(lst.text, "1 | first") || !contains(lst.text, "2 | second") {
		t.Errorf("줄번호가 안 매겨졌다:\n%s", lst.text)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
