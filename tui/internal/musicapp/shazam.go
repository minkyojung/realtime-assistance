package musicapp

import (
	"context"
	"errors"
	"fmt"

	"amcli/tui/internal/api"
	"amcli/tui/internal/app"
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
	shazamStartedMsg struct{}
	shazamMsg        struct {
		res shazam.Result
		err error
	}
)

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
	return m, app.Say(m.Name(), recognizedLine(ct)), true
}

// recognized — SHMediaItem 을 카탈로그 곡으로 옮기고, 그 자리에서 대조한다.
//
// 대조는 markInLibrary 가 한다. 이름으로 맞추는 참고값이라는 한계도 함께
// 물려받는다 — localMatch 머리말. 스펙은 appleMusicId·isrc 정확 일치를
// 요구하지만(docs/04), Music.app 덤프에 그 두 키가 없어서 아직 못 쓴다.
// 인식 결과에는 둘 다 실어 두므로, 라이브러리에 키가 생기는 날 여기만 바꾸면 된다.
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
	t, ok := localMatch(ct)
	if !ok {
		return head + "  ·  not in Your Library"
	}
	if t.PlayCount == 0 && t.LastPlayedAt == nil {
		return head + "  ·  in Your Library — and you have never played it"
	}
	return head + fmt.Sprintf("  ·  in Your Library — played %d times", t.PlayCount)
}
