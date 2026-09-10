package musicapp

import (
	"context"
	"errors"
	"fmt"

	"amcli/tui/internal/api"
	"amcli/tui/internal/app"
	"amcli/tui/internal/applemusic"
	"amcli/tui/internal/data"
	"amcli/tui/internal/shazam"
	tea "charm.land/bubbletea/v2"
)

// 인식 — 방 안의 소리가 들어오는 유일한 문이다 (F10 · F11).
//
// 알아맞힌 곡을 api.CatalogTrack 으로 옮긴다. 인식 결과 전용 자료형을
// 따로 만들지 않는 이유는, 그 곡의 처지가 카탈로그 곡과 똑같기 때문이다 —
// **아직 내 것이 아닐 수 있는 곡**. 그래서 담기·틀기(playCatalog)와 줄
// 그리기(catalogCells)가 그대로 산다.
//
// 이 서비스가 하려는 말이 나오는 자리이기도 하다. 알아맞힌 곡이 이미
// 라이브러리에 있는데 한 번도 안 들은 곡이면, 그 사실을 로그에 적는다.

type (
	shazamReadyMsg   struct{ ok bool }
	shazamStartedMsg struct{}
	shazamMsg        struct {
		res shazam.Result
		err error
	}

	// shazamMarkedMsg — 알아맞힌 곡이 담긴 곡인지 Apple 이 답했다.
	// 못 물었으면 짐작한 채로 온다.
	shazamMarkedMsg struct{ track api.CatalogTrack }
)

// cmdShazamInit — 헬퍼가 옆에 있는지 한 번만 본다.
//
// Cmd 로 미루는 이유는 cmdCatalogInit 과 같다 — New() 가 파일 시스템을
// 읽으면 골든 화면이 개발자 기계에 종속된다.
//
// 헬퍼는 배포물에 안 들어간다. ShazamKit 은 제한 엔타이틀먼트라
// 프로비저닝 프로파일이 있어야 서명되는데, 그 관문을 배포 경로에
// 끼워 넣지 않기로 했다(helper/README.md). 그래서 배포판에서는 이 답이
// 언제나 false 이고, `/shazam` 은 아예 나타나지 않는다.
func cmdShazamInit() tea.Msg {
	_, err := shazam.HelperPath()
	return shazamReadyMsg{ok: err == nil}
}

func cmdShazam() tea.Msg {
	res, err := shazam.Listen(context.Background())
	return shazamMsg{res: res, err: err}
}

// shazamCmd — `/shazam`. 듣는 데 몇 초 걸리므로 먼저 왜 기다리는지 말한다.
func (m Model) shazamCmd(string) tea.Cmd {
	if m.shzBusy {
		return send(errMsg{errListening})
	}
	// 헬퍼가 없으면 마이크를 켜기도 전에 끝난다. 그 사실을 지금 말한다.
	if _, err := shazam.HelperPath(); err != nil {
		return send(errMsg{err})
	}
	return tea.Batch(
		send(shazamStartedMsg{}),
		app.Say(m.Name(), "Listening…"),
		cmdShazam,
	)
}

var errListening = errors.New("Already listening")

// applyShazam — 인식 관련 메시지를 한자리에서 처리한다 (applyCatalog 와 같은 꼴).
func (m Model) applyShazam(msg tea.Msg) (app.App, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case shazamReadyMsg:
		m.shzOK = msg.ok
		return m, nil, true

	case shazamStartedMsg:
		m.shzBusy = true
		return m, nil, true

	case shazamMsg:
		m.shzBusy = false
		if msg.err != nil {
			// 못 알아들은 것은 실패가 아니다. 조용했거나 모르는 곡이다.
			// 빨간 줄로 말하면 사용자가 고장난 줄 안다.
			if errors.Is(msg.err, shazam.ErrNoMatch) {
				return m, app.Say(m.Name(),
					"Did not catch that — play it out loud and try again"), true
			}
			return m, app.SayErr(m.Name(), msg.err), true
		}
		return m.applyRecognition(msg.res)

	case shazamMarkedMsg:
		if id := msg.track.AppleMusicId; id != "" {
			for i := range m.shzHits {
				if m.shzHits[i].AppleMusicId == id {
					m.shzHits[i] = msg.track
					break
				}
			}
		}
		return m, app.Say(m.Name(), recognizedLine(msg.track)), true
	}
	return m, nil, false
}

// applyRecognition — 알아맞힌 곡 하나를 화면에 앉힌다.
func (m Model) applyRecognition(res shazam.Result) (app.App, tea.Cmd, bool) {
	ct := recognized(res)
	// 새것이 위다. 인식은 "방금 무엇이 흐르는가"를 묻는 일이라
	// 마지막 답이 언제나 가장 쓸모 있다.
	m.shzHits = append([]api.CatalogTrack{ct}, m.shzHits...)
	m.resync(data.Lib())
	m.listIdx, m.listTop = 0, 0
	m.jumpTo(secShazam, "")
	// 줄은 지금 앉히고, 말은 답을 듣고 한다.
	return m, cmdMarkRecognized(m.cat, ct), true
}

// cmdMarkRecognized — 담긴 곡인지 Apple 에게 묻고, 그 답이 온 뒤에 말한다.
//
// 짐작한 문장을 먼저 띄우고 나중에 고치지 않는다. 사용자는 이미 "없다"를
// 읽은 뒤이고, 이 서비스가 하는 말 중 제일 중요한 문장이 그것이다.
// 인식은 몇 초짜리 일이고 이 물음은 몇백 ms 다 — 기다려도 티가 안 난다.
//
// 못 물으면(로그인 전·id 없음·망 실패) 짐작한 채로 간다. 말이 늦는 것보다
// 덜 정확한 편이 낫고, 그 경우는 지금까지와 똑같이 동작한다.
func cmdMarkRecognized(c *applemusic.Client, ct api.CatalogTrack) tea.Cmd {
	return func() tea.Msg {
		if c == nil || ct.AppleMusicId == "" {
			return shazamMarkedMsg{track: ct}
		}
		ctx, cancel := context.WithTimeout(context.Background(), catalogTimeout)
		defer cancel()
		out, err := c.MarkInLibrary(ctx, []api.CatalogTrack{ct})
		if err != nil || len(out) == 0 {
			return shazamMarkedMsg{track: ct}
		}
		return shazamMarkedMsg{track: out[0]}
	}
}

// recognized — SHMediaItem 을 카탈로그 곡으로 옮기고, 그 자리에서 짐작한다.
//
// 여기의 짐작은 **줄을 그리기 위한 임시값**이다. 진짜 답은 곧
// cmdMarkRecognized 가 Apple 에게 받아 와 덮어쓴다. 그래도 여기서 한 번
// 채우는 이유는, 못 물었을 때(로그인 전) 그대로 쓰이기 때문이다.
func recognized(res shazam.Result) api.CatalogTrack {
	ct := api.CatalogTrack{
		AppleMusicId: res.AppleMusicID,
		Title:        res.Title,
		ArtistName:   res.Artist,
	}
	if res.ISRC != "" {
		ct.Isrc = &res.ISRC
	}
	if res.Genre != "" {
		ct.Genre = &res.Genre
	}
	if res.ArtworkURL != "" {
		ct.ArtworkUrl = &res.ArtworkURL
	}
	if res.Year > 0 {
		ct.Year = &res.Year
	}
	return markInLibrary([]api.CatalogTrack{ct})[0]
}

// recognizedLine — 알아맞힌 뒤에 남기는 한 문장.
//
// 곡 이름만 적으면 Shazam 과 다를 것이 없다. 이 서비스가 더 말할 수 있는
// 것은 **그 곡과 나의 관계**다 — 갖고 있는지, 갖고도 안 들었는지.
func recognizedLine(ct api.CatalogTrack) string {
	head := fmt.Sprintf("%s — %s", ct.Title, ct.ArtistName)

	// 담겼는지는 Apple 이 답했으면 그 답이 먼저다. 로컬에서 못 찾는 것은
	// 이름이 어긋났다는 뜻이지 없다는 뜻이 아니다.
	t, found := localMatch(ct)
	held := found
	if ct.InLibrary != nil {
		held = *ct.InLibrary
	}

	if !held {
		return head + "  ·  not in Your Library"
	}
	// 담긴 것은 아는데 어느 줄인지 못 찾았다. 횟수는 말하지 않는다 —
	// 모르는 수를 지어내면 이 문장 전체가 못 믿을 것이 된다.
	if !found {
		return head + "  ·  in Your Library"
	}
	if t.PlayCount == 0 && t.LastPlayedAt == nil {
		return head + "  ·  in Your Library — and you have never played it"
	}
	return head + fmt.Sprintf("  ·  in Your Library — played %d times", t.PlayCount)
}
