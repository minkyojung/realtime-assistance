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

// 짝짓기는 제목으로 한다. 아티스트로는 안 한다 —
// 실측에서 카탈로그(kr)가 "로제"라고 준 곡을 Music.app 은 "ROSÉ" 로 적었다.
func TestAttachAddedIgnoresLocalisedArtistNames(t *testing.T) {
	picks := []intent.Pick{
		{TrackID: 7, Reason: "이미 가진 곡"},
		{CatalogID: "c1", Reason: "밖에서 온 곡"},
	}
	extras := []api.CatalogTrack{
		{AppleMusicId: "c1", Title: "Messy (From F1 The Movie)", ArtistName: "로제"},
	}
	got, dropped := attachAdded(picks, extras, []api.Track{added(99, "Messy (From F1 The Movie)")})

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
	got, dropped := attachAdded(picks, extras, []api.Track{added(1, "arrived")})

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
	got, dropped := attachAdded(picks, extras, []api.Track{added(1, "same name"), added(2, "same name")})

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
