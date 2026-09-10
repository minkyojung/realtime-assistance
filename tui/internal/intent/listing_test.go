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

	if len(lst.rows) != 2 {
		t.Fatalf("목록 길이: got %d, want 2", len(lst.rows))
	}
	if lst.rows[0].trackID != -2064959302851484466 || lst.rows[1].trackID != 8673039688227705491 {
		t.Errorf("되돌리는 표가 틀렸다: %v", lst.rows)
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

// 카탈로그 후보는 라이브러리 번호에서 **이어진다.** 번호를 따로 매기면
// 모델이 돌려준 7 이 어느 세계의 7 인지 알 수 없다.
func TestCatalogCandidatesContinueTheNumbering(t *testing.T) {
	lib := []api.Track{track(11, "mine", false), track(22, "also mine", false)}
	lst := renderLibrary(lib, nil, time.Now()).withCatalog([]api.CatalogTrack{
		{AppleMusicId: "1791161222", Title: "stranger", ArtistName: "Bon Iver"},
	})

	if len(lst.rows) != 3 {
		t.Fatalf("줄 수: got %d, want 3", len(lst.rows))
	}
	if lst.rows[2].catalogID != "1791161222" || lst.rows[2].trackID != 0 {
		t.Errorf("셋째 줄이 카탈로그를 안 가리킨다: %+v", lst.rows[2])
	}
	if !contains(lst.catalog, "3 | stranger") {
		t.Errorf("번호가 안 이어졌다:\n%s", lst.catalog)
	}
	// 라이브러리 표는 그대로여야 한다 — 그래야 프롬프트 캐시가 걸린다.
	if contains(lst.text, "stranger") {
		t.Error("카탈로그 후보가 라이브러리 표에 섞였다 — 캐시가 매번 깨진다")
	}
}

// 후보가 없으면 블록도 없다. 빈 표를 붙이면 모델이 "밖에 아무것도 없다"는
// 사실이 아닌 것을 읽는다.
func TestNoCatalogBlockWhenThereAreNoCandidates(t *testing.T) {
	lst := renderLibrary([]api.Track{track(11, "mine", false)}, nil, time.Now()).withCatalog(nil)
	if lst.catalog != "" {
		t.Errorf("빈 카탈로그 블록이 붙었다:\n%s", lst.catalog)
	}
	if len(lst.rows) != 1 {
		t.Errorf("줄 수: got %d, want 1", len(lst.rows))
	}
}
