package musicapp

import (
	"context"
	"fmt"
	"time"

	"amcli/tui/internal/api"
	"amcli/tui/internal/applemusic"
	"amcli/tui/internal/data"
	"amcli/tui/internal/intent"
	"amcli/tui/internal/music"
	tea "charm.land/bubbletea/v2"
)

// 라이브러리 밖에서 고른 곡은 **담아야 튼다.**
//
// Apple Music API 는 재생을 시키지 못하고, Music.app 은 제 라이브러리의 곡만
// 안다. 그래서 고른 것과 트는 것 사이에 한 단계가 더 있다 —
// 담고, 나타나기를 기다리고, 진짜 persistent ID 를 받아 온다.
//
// 실측: 3곡을 POST 한 번에 담는 데 0.7초, Music.app 에 나타나기까지 4.1초.
// 그래서 기다리는 것이 성립한다. 분 단위였으면 다른 모양이어야 했다.

// catalogPicks 는 아직 내 것이 아닌 픽만 골라낸다.
func catalogPicks(res intent.Result) []string {
	var out []string
	for _, p := range res.Picks {
		if p.CatalogID != "" {
			out = append(out, p.CatalogID)
		}
	}
	return out
}

// cmdResolvePicks 는 라이브러리 밖의 픽을 담고, 큐를 앉힐 수 있는 꼴로 바꾼다.
//
// 같은 queueMsg 를 돌려준다. 새 메시지 종류를 만들지 않는 이유는 이 단계가
// **답을 바꾸는 것이 아니라 준비를 마치는 것**이기 때문이다. 큐를 앉히는
// 코드는 이 단계가 있었는지 몰라도 된다.
func cmdResolvePicks(ctx context.Context, cat *applemusic.Client, msg queueMsg) tea.Cmd {
	return func() tea.Msg {
		out := msg
		out.resolved = true

		ids := catalogPicks(msg.res)
		if cat == nil || cat.UserToken == "" {
			// 담을 수 없으면 버린다. 버리는 것은 조용히 하지 않는다 —
			// 세어서 위층이 말하게 한다.
			out.res.Picks, out.dropped = dropCatalogPicks(msg.res.Picks)
			return out
		}

		before, err := music.LibraryIDs()
		if err != nil {
			out.res.Picks, out.dropped = dropCatalogPicks(msg.res.Picks)
			return out
		}
		if err := cat.AddToLibrary(ctx, ids); err != nil {
			out.res.Picks, out.dropped = dropCatalogPicks(msg.res.Picks)
			return out
		}

		// 나타나기를 기다린다. 다 오지 않아도 온 만큼은 쓴다 —
		// 하나가 늦는다고 큐 전체를 버릴 이유가 없다.
		arrived := waitForArrivals(ctx, before, len(ids))

		b, err := music.DumpLibrary(ctx)
		if err != nil {
			out.res.Picks, out.dropped = dropCatalogPicks(msg.res.Picks)
			return out
		}
		lib, err := data.FromDump(b)
		if err != nil {
			out.res.Picks, out.dropped = dropCatalogPicks(msg.res.Picks)
			return out
		}
		_ = data.SaveCache(lib) // 실패해도 다음 시작이 조금 느릴 뿐이다

		out.lib = lib
		out.res.Picks, out.dropped = attachAdded(msg.res.Picks, msg.extras, newTracks(lib, arrived))
		return out
	}
}

// waitForArrivals 는 라이브러리가 want 만큼 늘어나기를 기다린다.
//
// ctx 를 보는 이유는 esc 다. 사용자가 그만뒀는데 20초를 마저 자고 있으면
// 그 20초 동안 앱이 이미 버린 일을 하고 있는 것이다.
func waitForArrivals(ctx context.Context, before map[string]bool, want int) map[string]bool {
	got := map[string]bool{}
	for i := 0; i < addPlayTries; i++ {
		select {
		case <-ctx.Done():
			return got
		case <-time.After(addPlayInterval):
		}
		now, err := music.LibraryIDs()
		if err != nil {
			continue
		}
		got = map[string]bool{}
		for id := range now {
			if !before[id] {
				got[id] = true
			}
		}
		if len(got) >= want {
			return got
		}
	}
	return got
}

// newTracks 는 방금 늘어난 번호에 해당하는 곡들을 스냅샷에서 꺼낸다.
func newTracks(lib *data.Library, ids map[string]bool) []api.Track {
	if len(ids) == 0 {
		return nil
	}
	out := make([]api.Track, 0, len(ids))
	for _, t := range lib.Tracks {
		if t.PersistentId != nil && ids[*t.PersistentId] {
			out = append(out, t)
		}
	}
	return out
}

// attachAdded 는 카탈로그 픽에 진짜 Track.Id 를 붙인다. 못 붙인 것은 버린다.
//
// 짝짓기는 **제목**으로 한다. 아티스트로는 안 한다 — 실측에서 카탈로그(kr)가
// "로제"라고 준 곡을 Music.app 은 "ROSÉ" 로 적었다. 스토어프론트가 이름을
// 현지화하기 때문이고, 제목은 그러지 않았다.
//
// 그리고 짝지을 판이 아주 작다. 방금 담은 몇 곡뿐이라, 제목 하나로도
// 헷갈릴 것이 사실상 없다. 한 번 쓴 곡은 빼서 두 픽이 같은 곡을 가리키지
// 않게 한다.
func attachAdded(picks []intent.Pick, extras []api.CatalogTrack, added []api.Track) ([]intent.Pick, int) {
	title := make(map[string]string, len(extras))
	for _, ct := range extras {
		title[ct.AppleMusicId] = normalize(ct.Title)
	}
	used := make(map[int64]bool, len(added))

	out := make([]intent.Pick, 0, len(picks))
	dropped := 0
	for _, p := range picks {
		if p.CatalogID == "" {
			out = append(out, p)
			continue
		}
		want := title[p.CatalogID]
		id := int64(0)
		for _, t := range added {
			if used[t.Id] || normalize(t.Title) != want {
				continue
			}
			id = t.Id
			break
		}
		if id == 0 {
			dropped++
			continue // 끝내 안 나타났거나 이름이 어긋났다
		}
		used[id] = true
		p.TrackID = id
		out = append(out, p)
	}
	return out, dropped
}

// dropCatalogPicks 는 담지 못한 픽을 빼고 몇 곡을 뺐는지 돌려준다.
func dropCatalogPicks(picks []intent.Pick) ([]intent.Pick, int) {
	out := make([]intent.Pick, 0, len(picks))
	dropped := 0
	for _, p := range picks {
		if p.CatalogID != "" {
			dropped++
			continue
		}
		out = append(out, p)
	}
	return out, dropped
}

// plural·wasWere — 세는 수에 말을 맞춘다. "1 tracks were" 는 사람이 쓴
// 문장으로 안 읽히고, 그 순간 나머지 문장도 기계가 뱉은 것으로 읽힌다.
func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func wasWere(n int) string {
	if n == 1 {
		return "it was"
	}
	return "they were"
}
