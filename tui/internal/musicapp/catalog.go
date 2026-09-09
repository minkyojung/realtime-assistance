package musicapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"amcli/tui/internal/api"
	"amcli/tui/internal/app"
	"amcli/tui/internal/applemusic"
	"amcli/tui/internal/data"
	"amcli/tui/internal/music"
	tea "charm.land/bubbletea/v2"
)

// 카탈로그 — 라이브러리 바깥이다.
//
// 이 앱의 후보 집합은 언제나 사용자의 라이브러리다 (internal/intent 참조).
// 그래서 바깥으로 나가는 것은 **사용자가 의도한 행동**이어야 하고,
// 검색하다 미끄러져 나가면 안 된다. `/catalog` 라는 명령이 그 경계다.

// 카탈로그 검색은 몇백 ms 다. 8초는 "이쯤이면 네트워크가 죽은 것"의 경계다.
const catalogTimeout = 8 * time.Second

// 동기화는 보통 몇 초다. 20초까지 기다린다 —
// 그 안에 안 나타나면 대개 영원히 안 나타난다 (Sync Library 가 꺼져 있다).
const (
	addPlayInterval = 1500 * time.Millisecond
	addPlayTries    = 13
)

var (
	errCatalogNotConfigured = errors.New(
		"Apple Music is not connected  ·  /setup")
	errLoginRequired = errors.New(
		"Sign in to add this to your library  ·  /login")
)

// addJob — 카탈로그 곡 하나를 담고, 나타나면 트는 일.
// Music.app 동기화는 우리가 제어할 수 없으므로 시도 횟수를 들고 있는다.
type addJob struct {
	track   api.CatalogTrack
	attempt int
}

type (
	catalogReadyMsg struct {
		client *applemusic.Client
		err    error
	}
	catalogLoginMsg struct {
		client *applemusic.Client
		err    error
	}
	catalogMsg struct {
		seq   int
		term  string
		items []api.CatalogTrack
		err   error
	}
	catalogAddedMsg struct {
		track api.CatalogTrack
		err   error
	}
	catalogTryPlayMsg struct {
		track   api.CatalogTrack
		attempt int
	}
	storefrontMsg     struct{ id string }
	loginStartedMsg   struct{}
	catalogStartedMsg struct{}
)

// cmdCatalogInit — 설정을 읽고 개발자 토큰을 만든다. 네트워크는 안 탄다.
//
// 그래도 Cmd 로 미루는 이유는 New() 를 순수하게 두기 위해서다.
// New() 에서 환경변수를 읽으면 골든 화면이 개발자 기계에 종속된다.
func cmdCatalogInit() tea.Msg {
	cfg, err := applemusic.LoadConfig()
	if err != nil {
		return catalogReadyMsg{err: err}
	}
	c, err := applemusic.NewClient(cfg)
	return catalogReadyMsg{client: c, err: err}
}

func cmdCatalogSearch(c *applemusic.Client, term string, seq int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), catalogTimeout)
		defer cancel()
		items, err := c.Search(ctx, term, 25)
		return catalogMsg{seq: seq, term: term, items: items, err: err}
	}
}

func cmdStorefront(c *applemusic.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), catalogTimeout)
		defer cancel()
		id, err := c.LookupStorefront(ctx)
		if err != nil {
			return storefrontMsg{}
		}
		return storefrontMsg{id: id}
	}
}

func cmdAddToLibrary(c *applemusic.Client, ct api.CatalogTrack) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), catalogTimeout)
		defer cancel()
		err := c.AddToLibrary(ctx, []string{ct.AppleMusicId})
		return catalogAddedMsg{track: ct, err: err}
	}
}

// cmdCatalogLogin — 브라우저를 열고 최대 3분을 기다린다.
//
// 모델이 든 클라이언트를 그대로 넘기면 안 된다. Login 이 UserToken 에
// 쓰는 동안 View 가 그것을 읽는다. 복사본에 로그인하고 통째로 갈아끼운다 —
// 상태는 언제나 Update 에서만 바뀐다.
func cmdCatalogLogin(c *applemusic.Client) tea.Cmd {
	cp := *c
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		if err := cp.Login(ctx); err != nil {
			return catalogLoginMsg{err: err}
		}
		return catalogLoginMsg{client: &cp}
	}
}

// catalogError — 스펙의 어휘를 로그 문장에 남긴다.
// 나중에 서버로 갈아타도 사용자가 보는 말이 안 바뀐다.
func catalogError(err error) error {
	switch {
	case errors.Is(err, applemusic.ErrUserTokenRejected):
		return errors.New("Your Apple Music session expired  ·  /login to reconnect")
	case errors.Is(err, applemusic.ErrCatalogUnavailable):
		return fmt.Errorf("CATALOG_UNAVAILABLE · Could not reach Apple Music. "+
			"Your library still works (%w)", err)
	}
	return err
}

// normalize — 곱슬 어포스트로피와 대소문자를 지운다.
// Apple 카탈로그와 Music.app 이 같은 곡을 다르게 적는 일이 흔하다.
func normalize(s string) string {
	s = strings.ReplaceAll(s, "’", "'")
	return strings.ToLower(strings.TrimSpace(s))
}

// localMatch — 카탈로그 곡이 이미 내 라이브러리에 있는지 이름으로 본다.
//
// Music.app 도 Apple Music API 도 카탈로그 id 를 라이브러리 곡에 붙여주지
// 않는다. 이름으로 맞추는 수밖에 없고, 그래서 이 판정은 참고값이다.
func localMatch(ct api.CatalogTrack) (api.Track, bool) {
	title, artist := normalize(ct.Title), normalize(ct.ArtistName)
	for _, t := range data.Lib().Tracks {
		if normalize(t.Title) == title && normalize(t.Artist.Name) == artist {
			return t, true
		}
	}
	return api.Track{}, false
}

func markInLibrary(items []api.CatalogTrack) []api.CatalogTrack {
	out := make([]api.CatalogTrack, len(items))
	copy(out, items)
	for i := range out {
		_, ok := localMatch(out[i])
		out[i].InLibrary = &ok
	}
	return out
}

func setInLibrary(items []api.CatalogTrack, appleMusicID string) []api.CatalogTrack {
	out := make([]api.CatalogTrack, len(items))
	copy(out, items)
	yes := true
	for i := range out {
		if out[i].AppleMusicId == appleMusicID {
			out[i].InLibrary = &yes
		}
	}
	return out
}

func inLibrary(ct api.CatalogTrack) bool { return ct.InLibrary != nil && *ct.InLibrary }

func catalogRows(items []api.CatalogTrack) []listRow {
	out := make([]listRow, 0, len(items))
	for i := range items {
		ct := items[i]
		out = append(out, listRow{catalog: &ct})
	}
	return out
}

// playCatalog — 카탈로그 곡을 튼다.
//
// Apple Music API 는 재생을 시키지 못한다. 재생은 Music.app 만 하고,
// Music.app 은 제 라이브러리에 있는 곡만 안다. 그래서 담아야 틀 수 있다.
func (m Model) playCatalog(ct api.CatalogTrack) (app.App, tea.Cmd) {
	// 이미 내 것이면 여기가 제일 튼튼하다. 담을 것도 기다릴 것도 없다.
	//
	// 그 곡 하나로 큐를 만든다. 곡 객체를 그냥 틀면 한 곡만 나오고 멈춘다 —
	// 이어서 나올 것을 Music.app 에게 줘야 한다(play.go).
	if t, ok := localMatch(ct); ok && t.PersistentId != nil {
		return m.playOne(t)
	}
	if m.cat == nil || m.cat.UserToken == "" {
		return m, app.SayErr(m.Name(), errLoginRequired)
	}
	// **이미 담은 곡을 또 담지 않는다.**
	//
	// 담기는 성공했는데 Music.app 에 아직 안 나타난 상태가 있다(iCloud
	// 동기화). 그때 우리 라이브러리 스냅샷에도 없으므로 위의 localMatch 가
	// 실패하고, 누를 때마다 같은 곡을 다시 담게 된다. 화면에는 ✓ 가 떠
	// 있는데 "담는 중" 이 또 뜨는 것이 그 모습이다.
	//
	// 담을 것이 없으니 나타나기를 기다리기만 한다.
	if inLibrary(ct) {
		m.adding = &addJob{track: ct}
		return m, tea.Batch(
			tea.Tick(addPlayInterval, func(time.Time) tea.Msg {
				return catalogTryPlayMsg{track: ct, attempt: 1}
			}),
			app.Say(m.Name(), fmt.Sprintf("Waiting for %q to show up in Music…", ct.Title)),
		)
	}
	m.adding = &addJob{track: ct}
	return m, tea.Batch(
		cmdAddToLibrary(m.cat, ct),
		app.Say(m.Name(), fmt.Sprintf("Adding %q to your library…", ct.Title)),
	)
}

// applyCatalog — 카탈로그 관련 메시지를 한자리에서 처리한다.
func (m Model) applyCatalog(msg tea.Msg) (app.App, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case catalogStartedMsg:
		m.catSeq++
		m.catBusy = true
		return m, nil, true

	case loginStartedMsg:
		m.catLogin = true
		return m, nil, true

	case catalogReadyMsg:
		m.cat = msg.client
		// 왜 안 되는지 기억해 둔다. "안 된다"만 알면 화면이 다음 행동을
		// 말할 수 없다 — 키를 놓으라는 것과 Team ID 를 적으라는 것은
		// 사람이 할 일이 다르다.
		m.catErr = msg.err
		if m.cat == nil {
			// 설정이 없는 것은 정상 상태다. 첫 화면에서 안 쓸 기능의
			// 실패를 읽게 할 이유가 없다.
			return m, nil, true
		}
		m.resync(data.Lib()) // 카탈로그 섹션이 생긴다
		if m.cat.UserToken != "" {
			return m, cmdStorefront(m.cat), true
		}
		return m, nil, true

	case storefrontMsg:
		if m.cat != nil && msg.id != "" {
			m.cat.Storefront = msg.id
		}
		return m, nil, true

	case catalogMsg:
		if msg.seq != m.catSeq {
			return m, nil, true // 지나간 요청의 답. 버린다
		}
		m.catBusy = false
		if msg.err != nil {
			return m, app.SayErr(m.Name(), catalogError(msg.err)), true
		}
		m.catTerm = msg.term
		m.catHits = markInLibrary(msg.items)
		// 검색 중이면 결과가 같은 화면 아래에 붙는다. 자리를 옮기지 않는다.
		if m.searching() {
			m.clampList()
			return m, nil, true
		}
		m.listIdx, m.listTop = 0, 0
		m.jumpTo(secCatalog, "")
		if len(m.catHits) == 0 {
			return m, app.Say(m.Name(), "Nothing on Apple Music for "+msg.term), true
		}
		return m, nil, true

	case catalogLoginMsg:
		m.catLogin = false
		if msg.err != nil {
			return m, app.SayErr(m.Name(), msg.err), true
		}
		m.cat = msg.client
		return m, tea.Batch(
			cmdStorefront(m.cat),
			app.Say(m.Name(), "Connected to Apple Music"),
		), true

	case catalogAddedMsg:
		if msg.err != nil {
			m.adding = nil
			return m, app.SayErr(m.Name(), catalogError(msg.err)), true
		}
		m.catHits = setInLibrary(m.catHits, msg.track.AppleMusicId)
		return m, tea.Tick(addPlayInterval, func(time.Time) tea.Msg {
			return catalogTryPlayMsg{track: msg.track, attempt: 1}
		}), true

	case catalogTryPlayMsg:
		return m.tryPlayAdded(msg)
	}
	return m, nil, false
}

// tryPlayAdded — 담은 곡이 Music.app 에 나타났는지 보고, 나타났으면 튼다.
//
// 고루틴 안 for{sleep} 이 아니라 메시지 사다리인 이유: 매 시도가 Update 를
// 거쳐야 그새 다른 것을 담기 시작했는지 판정할 수 있다.
func (m Model) tryPlayAdded(msg catalogTryPlayMsg) (app.App, tea.Cmd, bool) {
	if m.adding == nil || m.adding.track.AppleMusicId != msg.track.AppleMusicId {
		return m, nil, true // 그새 다른 것을 담기 시작했다
	}

	err := music.PlayByTitleArtist(msg.track.Title, msg.track.ArtistName)
	if err == nil {
		m.adding = nil
		// 라이브러리를 다시 읽는다. 담긴 곡이 우리 스냅샷에는 아직 없어서,
		// 갱신하지 않으면 다음에 같은 곡을 골랐을 때 또 담으려 든다.
		//
		// persistent ID·길이는 다음 폴링이 알려준다. 우리가 찾을 필요가 없다.
		return m, tea.Batch(fetchStatus, cmdDumpLibrary(false),
			app.Say(m.Name(), fmt.Sprintf("Added %q and started playing", msg.track.Title))), true
	}
	// 관문은 재시도 대상이 아니다. 기다린다고 권한이 생기지 않는다.
	if errors.Is(err, music.ErrNotRunning) || errors.Is(err, music.ErrPermissionDenied) {
		m.adding = nil
		return m, nil, true
	}
	if msg.attempt >= addPlayTries {
		m.adding = nil
		// 담긴 것은 사실이다. 못 한 것은 재생뿐이므로 실패로 말하지 않는다 —
		// 실패로 말하면 사용자가 또 담으려 든다.
		return m, app.Say(m.Name(), fmt.Sprintf(
			"Added %q to your library. If it has not shown up in Music, "+
				"check that Sync Library is on", msg.track.Title)), true
	}
	m.adding.attempt = msg.attempt + 1
	next := msg.attempt + 1
	return m, tea.Tick(addPlayInterval, func(time.Time) tea.Msg {
		return catalogTryPlayMsg{track: msg.track, attempt: next}
	}), true
}
