package data

import (
	"os"
	"testing"
)

func loadSample(t *testing.T) *Library {
	t.Helper()
	b, err := os.ReadFile("testdata/dump-sample.json")
	if err != nil {
		t.Fatal(err)
	}
	l, err := FromDump(b)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// persistent ID 가 곧 id 다. 다시 읽어도 같은 수여야 스냅샷을 갈아끼울 수 있다.
func TestTrackIDIsStable(t *testing.T) {
	const pid = "81F599359B2E449B"
	got := TrackID(pid)
	if got != TrackID(pid) {
		t.Fatal("같은 pid 가 다른 id 를 냈다")
	}
	if got == 0 {
		t.Fatal("정상 pid 가 0 이 됐다")
	}
	// 8~F 로 시작하면 음수다. 그것 자체는 문제가 아니다.
	if got >= 0 {
		t.Errorf("81F5... 는 음수여야 한다: %d", got)
	}
	if TrackID("not hex") != 0 {
		t.Error("망가진 pid 는 0 이어야 한다")
	}
}

// 제목에 | 와 줄바꿈과 따옴표가 들어가도 온전해야 한다.
// 구분자 대신 JSON 을 쓰는 이유가 이것이다.
func TestDumpKeepsAwkwardText(t *testing.T) {
	l := loadSample(t)
	tr, ok := l.Track(TrackID("81F599359B2E449B"))
	if !ok {
		t.Fatal("첫 곡을 못 찾았다")
	}
	if tr.Title != "Pipe | Newline\nTitle" {
		t.Errorf("제목이 깨졌다: %q", tr.Title)
	}
	if tr.Artist.Name != `Awkward "Quoted" Artist` {
		t.Errorf("아티스트가 깨졌다: %q", tr.Artist.Name)
	}
}

// 플레이리스트에만 담긴 곡은 담은 날짜가 없다. 지어내면 근거가 거짓이 된다.
func TestPlaylistOnlyTrackHasNoAddedAt(t *testing.T) {
	l := loadSample(t)
	tr, ok := l.Track(TrackID("FFFFFFFFFFFFFFFF"))
	if !ok {
		t.Fatal("플레이리스트 전용 곡이 없다")
	}
	if tr.InLibrary {
		t.Error("inLibrary 여서는 안 된다")
	}
	if tr.AddedAt != nil {
		t.Errorf("담은 적이 없는데 담은 날짜가 있다: %v", tr.AddedAt)
	}
	// 담아둔 곳이 어디든 검색에는 걸려야 한다.
	if len(l.Search("Playlist Only")) != 1 {
		t.Error("플레이리스트 전용 곡이 검색에 안 걸린다")
	}
	// 그러나 둘러보기에는 안 나온다.
	for _, s := range l.Songs() {
		if s.Id == tr.Id {
			t.Error("Songs() 에 라이브러리 밖 곡이 섞였다")
		}
	}
}

// persistent ID 가 없는 곡은 두 세계를 이을 수 없으므로 버린다.
func TestDumpDropsTrackWithoutPersistentID(t *testing.T) {
	l := loadSample(t)
	for _, tr := range l.Tracks {
		if tr.Title == "No Persistent ID" {
			t.Fatal("pid 없는 곡이 들어왔다")
		}
	}
	if len(l.Tracks) != 3 {
		t.Errorf("곡 수: got %d, want 3", len(l.Tracks))
	}
}

func TestDumpPlaylists(t *testing.T) {
	l := loadSample(t)
	if len(l.Playlists) != 2 {
		t.Fatalf("플레이리스트 수: got %d, want 2", len(l.Playlists))
	}
	// 곡이 많은 순으로 선다.
	if l.Playlists[0].Name != "Mixed" || l.Playlists[0].TrackCount != 2 {
		t.Errorf("첫 플레이리스트: %+v", l.Playlists[0])
	}
	// 순서가 보존돼야 한다 — 담은 순서가 곧 재생 순서다.
	tracks := l.PlaylistTracks(l.Playlists[0])
	if len(tracks) != 2 || tracks[0].Title != "Playlist Only" {
		t.Errorf("플레이리스트 순서가 어긋났다: %+v", tracks)
	}
}

// 앨범 제목이 없으면 앨범 자체가 없다. 빈 앨범을 만들지 않는다.
func TestDumpOmitsEmptyAlbum(t *testing.T) {
	l := loadSample(t)
	tr, _ := l.Track(TrackID("0000000000000001"))
	if tr.Album != nil {
		t.Errorf("앨범이 없는데 앨범이 생겼다: %+v", tr.Album)
	}
	if tr.Source != "file" {
		t.Errorf("source: got %q, want file", tr.Source)
	}
	if !tr.Favorited {
		t.Error("favorited 가 안 넘어왔다")
	}
}

func TestSetSwapsSnapshot(t *testing.T) {
	l := loadSample(t)
	Set(l)
	if len(Lib().Tracks) != 3 {
		t.Fatal("스냅샷이 안 바뀌었다")
	}
	// 갈아끼워도 pid → 곡이 그대로 이어져야 한다.
	if _, ok := Lib().ByPersistentID("81F599359B2E449B"); !ok {
		t.Error("갈아끼운 뒤 pid 로 못 찾는다")
	}
	Set(&Library{})
	if len(Lib().Tracks) != 0 {
		t.Error("빈 스냅샷으로 못 바꿨다")
	}
}
