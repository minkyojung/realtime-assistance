package musicapp

import (
	"time"

	"amcli/tui/internal/data"
	"amcli/tui/internal/music"
	tea "charm.land/bubbletea/v2"
)

// 곡이 끝나는 순간을 잡아 한 줄 적는다.
//
// 폴링이 이미 1초마다 지금 곡을 보고 있다. 곡이 바뀌면 **직전 곡이 끝난
// 것**이고, 그때 우리는 그 곡을 얼마나 들었는지(마지막으로 본 위치)와
// 얼마짜리였는지를 알고 있다. 새 인프라가 필요 없는 이유다.
//
// 놓치는 것도 있다. 우리 앱이 꺼져 있으면 폴링이 없고, 1초 안에 두 곡이
// 지나가면 가운데를 못 본다. 그래서 이 기록은 **누적 횟수의 출처가 아니다** —
// 그것은 Music.app 이 구멍 없이 세고 있다(intent 의 signals 칸).
// 여기 적는 것은 Music.app 이 모르는 것뿐이다. 왜 끝났는지, 어느 요청에서
// 나온 곡인지(turnID — 0 이면 사람이 직접 고른 것이다).

// 끝난 이유를 우리가 만든 경우, 다음 폴링이 그것을 집어 가도록 남겨 둔다.
//
// 폴링은 1초 주기이고 AppleScript 는 그보다 빨리 돌아온다. 그래도 오래 남은
// 표시는 엉뚱한 곡에 붙으므로 시효를 둔다.
const endHintTTL = 5 * time.Second

// hintEnd 는 "다음에 곡이 바뀌면 그 이유는 이것이다" 라고 적어 둔다.
func (m Model) hintEnd(why data.EndedBy) Model {
	m.endHint, m.endHintAt = why, time.Now()
	return m
}

// notePlayback 은 폴링 결과를 받아, 곡이 바뀌었으면 직전 곡을 기록한다.
//
// m.live 는 아직 **이전** 상태다. 부르는 쪽이 이것을 먼저 부르고 나서
// m.live 를 갈아끼운다.
func (m Model) notePlayback(next music.PlayerState) (Model, tea.Cmd) {
	prev := m.live
	if prev.PersistentID == "" {
		return m, nil // 처음 본 것이다. 끝난 곡이 없다
	}
	// 같은 곡이 계속 흐르는 중이면 아직 끝나지 않았다.
	//
	// 다만 위치가 크게 뒤로 갔으면 한 판이 끝나고 다시 시작한 것이다 —
	// `/repeat one` 에서는 이것이 곡이 끝나는 유일한 모습이다.
	if next.PersistentID == prev.PersistentID && !next.Stopped {
		if next.PositionMs >= prev.PositionMs-2000 {
			return m, nil
		}
	}

	why := m.whyEnded(prev, next)
	m.endHint = ""

	t, ok := data.Lib().ByPersistentID(prev.PersistentID)
	if !ok {
		return m, nil // 라이브러리 밖의 곡. 되돌릴 id 가 없다
	}
	return m, cmdAppendPlay(data.Play{
		At:       time.Now(),
		TrackID:  t.Id,
		PlayedMs: prev.PositionMs,
		DurMs:    prev.DurationMs,
		EndedBy:  why,
		Shuffle:  prev.Shuffle,
		TurnID:   m.turnID,
	})
}

// whyEnded 는 무엇이 곡을 끝냈는지 정한다.
//
// 우리가 끊었으면 그 표시를 쓴다. 아니면 위치로 판단한다 — 끝에 닿았으면
// 다 흐른 것이고, 아니면 누군가 넘긴 것이다. 그 누군가가 터미널인지
// Music.app 인지 에어팟인지는 구별하지 않는다. **셋 다 사용자이므로 같은
// 신호다.** 구별해야 하는 것은 사용자와 우리뿐이다.
func (m Model) whyEnded(prev, next music.PlayerState) data.EndedBy {
	if m.endHint != "" && time.Since(m.endHintAt) < endHintTTL {
		return m.endHint
	}
	if next.Stopped {
		return data.EndedStopped
	}
	// 폴링이 1초 주기라 마지막으로 본 위치는 실제 끝보다 최대 그만큼 이르다.
	if prev.DurationMs > 0 && prev.PositionMs >= prev.DurationMs-2000 {
		return data.EndedDone
	}
	return data.EndedSkipped
}

// cmdAppendPlay 는 파일에 한 줄 덧붙인다.
//
// 실패해도 아무 말 하지 않는다. 못 적은 사건 하나를 알리자고 화면에 오류를
// 띄우면, 듣는 내내 그 말이 뜬다. 재생을 막지 않는 것이 먼저다.
func cmdAppendPlay(p data.Play) tea.Cmd {
	return func() tea.Msg {
		_ = data.AppendPlay(p)
		return nil
	}
}
