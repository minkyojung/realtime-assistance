package musicapp

import (
	"testing"

	"amcli/tui/internal/api"
	"amcli/tui/internal/intent"
)

func pid(s string) *string { return &s }

func added(id int64, title string) api.Track {
	return api.Track{Id: id, Title: title, PersistentId: pid("P"), Artist: api.Artist{Name: "ROSÉ"}}
}

// 길이를 아는 채로 담긴 곡. 짝짓기의 정상 경로다.
func addedMs(id int64, title string, ms int) api.Track {
	t := added(id, title)
	t.DurationMs = ms
	return t
}

// 길이를 모르면 제목으로 짝짓는다. 아티스트로는 안 한다 —
// 실측에서 카탈로그(kr)가 "로제"라고 준 곡을 Music.app 은 "ROSÉ" 로 적었다.
func TestAttachAddedIgnoresLocalisedArtistNames(t *testing.T) {
	picks := []intent.Pick{
		{TrackID: 7, Reason: "이미 가진 곡"},
		{CatalogID: "c1", Reason: "밖에서 온 곡"},
	}
	extras := []api.CatalogTrack{
		{AppleMusicId: "c1", Title: "Messy (From F1 The Movie)", ArtistName: "로제"},
	}
	got, dropped := attachAdded(picks, extras, []api.Track{added(99, "Messy (From F1 The Movie)")}, nil)

	if dropped != 0 {
		t.Fatalf("아티스트 이름이 달라서 버렸다: dropped=%d", dropped)
	}
	if len(got) != 2 || got[1].TrackID != 99 {
		t.Errorf("담긴 곡에 번호가 안 붙었다: %+v", got)
	}
	if got[0].TrackID != 7 {
		t.Error("라이브러리 픽을 건드렸다")
	}
}

// 끝내 안 나타난 곡은 버린다. 버리되 **몇 곡인지 센다** —
// 조용히 사라지면 무엇이 빠졌는지 알 길이 없다.
func TestAttachAddedCountsWhatItDropped(t *testing.T) {
	picks := []intent.Pick{
		{CatalogID: "c1"},
		{CatalogID: "c2"},
	}
	extras := []api.CatalogTrack{
		{AppleMusicId: "c1", Title: "arrived"},
		{AppleMusicId: "c2", Title: "never showed up"},
	}
	got, dropped := attachAdded(picks, extras, []api.Track{added(1, "arrived")}, nil)

	if len(got) != 1 || got[0].TrackID != 1 {
		t.Errorf("도착한 곡을 못 앉혔다: %+v", got)
	}
	if dropped != 1 {
		t.Errorf("dropped = %d, want 1", dropped)
	}
}

// 같은 제목이 두 번 나와도 두 픽이 같은 곡을 가리키면 안 된다.
// 큐에 같은 곡이 두 번 들어가고, 근거는 서로 다른 말을 한다.
func TestAttachAddedNeverUsesOneTrackTwice(t *testing.T) {
	picks := []intent.Pick{{CatalogID: "c1"}, {CatalogID: "c2"}}
	extras := []api.CatalogTrack{
		{AppleMusicId: "c1", Title: "same name"},
		{AppleMusicId: "c2", Title: "same name"},
	}
	got, dropped := attachAdded(picks, extras, []api.Track{added(1, "same name"), added(2, "same name")}, nil)

	if dropped != 0 {
		t.Fatalf("dropped = %d", dropped)
	}
	if got[0].TrackID == got[1].TrackID {
		t.Errorf("두 픽이 같은 곡을 가리킨다: %+v", got)
	}
}

// 담을 수 없을 때(로그인 전 등) 라이브러리 픽은 살고 나머지는 세어서 버린다.
func TestDropCatalogPicksKeepsWhatWeAlreadyHave(t *testing.T) {
	got, dropped := dropCatalogPicks([]intent.Pick{
		{TrackID: 1}, {CatalogID: "c1"}, {TrackID: 2}, {CatalogID: "c2"},
	})
	if len(got) != 2 || got[0].TrackID != 1 || got[1].TrackID != 2 {
		t.Errorf("가진 곡까지 버렸다: %+v", got)
	}
	if dropped != 2 {
		t.Errorf("dropped = %d, want 2", dropped)
	}
}

// 밖에서 고른 곡이 없으면 이 단계 자체가 없어야 한다.
func TestCatalogPicksIsEmptyForAnOrdinaryQueue(t *testing.T) {
	if n := catalogPicks(intent.Result{Picks: []intent.Pick{{TrackID: 1}, {TrackID: 2}}}); len(n) != 0 {
		t.Errorf("라이브러리 안에서만 고른 큐가 담기 단계를 부른다: %v", n)
	}
}

// 세는 수에 말을 맞춘다. "1 tracks were" 는 사람이 쓴 문장으로 안 읽힌다.
func TestPluralReadsLikeAPersonWroteIt(t *testing.T) {
	for _, c := range []struct {
		n          int
		want, were string
	}{
		{1, "1 track", "it was"},
		{3, "3 tracks", "they were"},
	} {
		if got := plural(c.n, "track"); got != c.want {
			t.Errorf("plural(%d) = %q, want %q", c.n, got, c.want)
		}
		if got := wasWere(c.n); got != c.were {
			t.Errorf("wasWere(%d) = %q, want %q", c.n, got, c.were)
		}
	}
}

// **제목도 현지화된다.** 이것이 진짜 함정이었다.
//
// 아티스트가 현지화되는 건 알고 피했는데("로제" ↔ "ROSÉ") 제목은 안 그런 줄
// 알았다. 실측: 카탈로그(kr) "야생화 — 박효신" 을 Music.app 은 "Wild Flower —
// Park Hyo Shin" 으로 적는다. 담기는 성공하고 곡은 라이브러리에 멀쩡히 들어와
// 있는데 화면은 "담지 못했다"고 말했다.
func TestAttachAddedSurvivesLocalisedTitles(t *testing.T) {
	picks := []intent.Pick{{CatalogID: "c1", Reason: "야생화"}}
	extras := []api.CatalogTrack{{AppleMusicId: "c1", Title: "야생화", ArtistName: "박효신"}}
	durMs := map[string]int{"c1": 312092}

	got, dropped := attachAdded(picks, extras,
		[]api.Track{addedMs(42, "Wild Flower", 312092)}, durMs)

	if dropped != 0 {
		t.Fatalf("이름이 영어로 바뀌었다고 버렸다: dropped=%d", dropped)
	}
	if len(got) != 1 || got[0].TrackID != 42 {
		t.Errorf("번호가 안 붙었다: %+v", got)
	}
}

// 밀리초까지 똑같지는 않다. 1초 안이면 같은 곡이다.
func TestAttachAddedAllowsASecondOfSlack(t *testing.T) {
	picks := []intent.Pick{{CatalogID: "c1"}}
	extras := []api.CatalogTrack{{AppleMusicId: "c1", Title: "야생화"}}

	got, dropped := attachAdded(picks, extras,
		[]api.Track{addedMs(42, "Wild Flower", 312092+700)}, map[string]int{"c1": 312092})
	if dropped != 0 || got[0].TrackID != 42 {
		t.Errorf("0.7초 차를 다른 곡으로 봤다: dropped=%d %+v", dropped, got)
	}

	// 한참 다르면 다른 곡이다. 길이를 안다면서 아무거나 앉히면 안 된다.
	_, dropped = attachAdded(picks, extras,
		[]api.Track{addedMs(42, "Wild Flower", 312092+60000)}, map[string]int{"c1": 312092})
	if dropped != 1 {
		t.Errorf("1분 차이 나는 곡을 같은 곡으로 봤다")
	}
}

// 길이가 같은 두 곡이 함께 담겨도 한 곡을 두 번 쓰지 않는다.
func TestAttachAddedNeverUsesOneTrackTwiceByDuration(t *testing.T) {
	picks := []intent.Pick{{CatalogID: "c1"}, {CatalogID: "c2"}}
	extras := []api.CatalogTrack{{AppleMusicId: "c1"}, {AppleMusicId: "c2"}}
	durMs := map[string]int{"c1": 200000, "c2": 200000}

	got, dropped := attachAdded(picks, extras,
		[]api.Track{addedMs(1, "A", 200000), addedMs(2, "B", 200000)}, durMs)
	if dropped != 0 {
		t.Fatalf("dropped = %d", dropped)
	}
	if got[0].TrackID == got[1].TrackID {
		t.Errorf("두 픽이 같은 곡을 가리킨다: %+v", got)
	}
}
