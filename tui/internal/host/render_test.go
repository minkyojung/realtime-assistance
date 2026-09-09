package host

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"amcli/tui/internal/api"
	"amcli/tui/internal/app"
	"amcli/tui/internal/data"
	"amcli/tui/internal/intent"
	"amcli/tui/internal/music"
	"amcli/tui/internal/musicapp"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// 한글은 표시폭이 2다. 폭 계산이 어긋나면 테두리와 정렬이 전부 깨지므로
// 어느 줄도 화면 폭을 넘지 않는지 자동으로 확인한다.
func TestViewFitsWidth(t *testing.T) {
	for _, w := range []int{70, 80, 96, 120, 200} {
		hm := New(musicapp.New())
		hm.LeaveHome()
		var m tea.Model = &hm
		m, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: 32})

		for i, line := range strings.Split(m.View().Content, "\n") {
			if got := lipgloss.Width(line); got > w {
				t.Errorf("터미널 폭 %d: %d번째 줄이 %d칸으로 넘침\n%q", w, i+1, got, line)
			}
		}
	}
}

// 기본 화면에 라이브러리와 재생 바가 실제로 그려지는지.
func TestViewShowsLibraryAndPlayer(t *testing.T) {
	hm := New(musicapp.New())
	hm.LeaveHome()
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 32})
	out := m.View().Content

	for _, want := range []string{
		"Recently Added", // 상태줄이 지금 어디인지 말한다
		"Please Wait for Me",
		"Busker Busker",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("화면에 %q 가 없다", want)
		}
	}
}

// tab 으로 섹션을 옮기면 목록이 실제로 바뀌는지.
func TestTabSwitchesSection(t *testing.T) {
	hm := New(musicapp.New())
	hm.LeaveHome()
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 32})
	first := m.View().Content

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.View().Content == first {
		t.Error("tab 을 눌렀는데 목록이 그대로다")
	}
}

// plain 은 색 코드를 벗긴다. 스타일이 낀 문자열은 그대로 비교할 수 없다.
var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plain(s string) string { return ansi.ReplaceAllString(s, "") }

func typeText(m tea.Model, s string) tea.Model {
	for _, r := range s {
		m, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	return m
}

// 입력창은 늘 활성이어야 한다. 이게 깨지면 아무것도 칠 수 없다.
// 그리고 프롬프트 모드에서는 목록을 건드리지 않아야 한다.
func TestPromptDoesNotFilterList(t *testing.T) {
	hm := New(musicapp.New())
	hm.LeaveHome()
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 32})
	m = typeText(m, "something quiet")

	out := m.View().Content
	if !strings.Contains(out, "something quiet") {
		t.Fatal("친 글자가 입력창에 없다 — 입력창이 활성이 아니다")
	}
	if !strings.Contains(out, "Peanut butter Sandwich") {
		t.Error("프롬프트를 치는 중인데 목록이 걸러졌다")
	}
}

// ctrl+f 로 들어간 검색 모드에서만 목록이 걸러져야 한다.
func TestSearchModeFiltersList(t *testing.T) {
	hm := New(musicapp.New())
	hm.LeaveHome()
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 32})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	m = typeText(m, "oasis")

	out := m.View().Content
	if !strings.Contains(out, "Search") {
		t.Error("검색 중이라는 표시가 없다")
	}
	// 재생 바에 뜨는 곡은 목록과 무관하게 남으므로, 재생 중이 아닌 곡으로 확인한다.
	if strings.Contains(out, "Peanut butter Sandwich") {
		t.Error("검색어를 쳤는데 목록이 걸러지지 않았다")
	}
	if !strings.Contains(out, "Oasis") {
		t.Error("검색 결과에 Oasis 곡이 없다")
	}
}

// Music.app 폴링 결과가 화면에 반영되는지.
func TestStatusUpdatesPlayer(t *testing.T) {
	hm := New(musicapp.New())
	hm.LeaveHome()
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 32})

	// library.json 에 실재하는 곡의 persistent ID 로 상태를 흘려보낸다.
	track := data.Lib().Tracks[0]
	m, _ = m.Update(musicapp.StatusMsgFor(music.PlayerState{
		Playing:      true,
		PersistentID: *track.PersistentId,
		PositionMs:   30_000,
		DurationMs:   track.DurationMs,
	}, nil))

	out := m.View().Content
	if !strings.Contains(out, track.Title) {
		t.Errorf("재생 바에 %q 가 없다", track.Title)
	}
	if !strings.Contains(out, "0:30") {
		t.Error("재생 위치가 반영되지 않았다")
	}
}

// 첫 실행 관문 — 로그인이 아니라 권한과 앱 실행 여부다.
func TestFirstRunGates(t *testing.T) {
	cases := []struct {
		err      error
		badge    string
		hintPart string
	}{
		{music.ErrPermissionDenied, "PERMISSION", "System Settings"},
		{music.ErrNotRunning, "MUSIC APP", "open -a Music"},
	}
	for _, c := range cases {
		hm := New(musicapp.New())
		hm.LeaveHome()
		var m tea.Model = &hm
		m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 32})
		m, _ = m.Update(musicapp.StatusMsgFor(music.PlayerState{}, c.err))

		out := m.View().Content
		if !strings.Contains(out, c.badge) {
			t.Errorf("%v: 배지 %q 가 없다", c.err, c.badge)
		}
		if !strings.Contains(out, c.hintPart) {
			t.Errorf("%v: 안내에 %q 가 없다", c.err, c.hintPart)
		}
	}
}

// 프롬프트를 보내면 Thinking 이 뜨고, 결과가 오면 큐로 바뀌는지.
// 실제 API 는 부르지 않는다 — queueMsg 를 직접 흘려보낸다.
func TestPromptToQueue(t *testing.T) {
	// 키가 없으면 요청이 아예 안 나간다(musicapp/startAsk). 여기서 보려는
	// 것은 그 뒤에 답이 화면에 앉는 경로다.
	t.Setenv("OPENAI_API_KEY", "sk-test")
	hm := New(musicapp.New())
	hm.LeaveHome()
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 32})
	m = typeText(m, "quiet")

	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter 를 눌렀는데 아무 Cmd 도 나오지 않았다")
	}
	if out := plain(m.View().Content); !strings.Contains(out, "묻는 중") {
		t.Error("요청 중이라는 표시가 없다")
	}
	if out := plain(m.View().Content); !strings.Contains(out, "quiet") {
		t.Error("내가 한 말이 로그에 없다")
	}

	l := data.Lib()
	m, _ = m.Update(musicapp.QueueMsgFor(state(t, m).app(), intent.Result{
		Title: "quiet set",
		Note:  "picked two you never played",
		Picks: []intent.Pick{
			{TrackID: l.Tracks[0].Id, Reason: "never played since you added it"},
			{TrackID: l.Tracks[1].Id, Reason: "same record"},
		},
		Usage: api.Usage{PromptTokens: 10000, CompletionTokens: 500, CostUsd: 0.0074},
	}, nil))

	// 앱이 로그에 남기는 말은 Cmd 로 오지만, 여기서 Cmd 를 실행하면 진짜
	// Music.app 을 건드린다. 그래서 로그 메시지만 직접 흘려보낸다.
	m, _ = m.Update(app.SayMsg{App: "music", Text: "picked two you never played"})

	out := plain(m.View().Content)
	for _, want := range []string{
		"picked two you never played", // 큐 전체 설명은 로그로 간다
		"quiet set",                   // 큐 제목이 상태줄에
		"Queue · 2",                   // 상태줄이 큐로 옮겨간 것을 보여준다
		"$0.0074",                     // 사용량
	} {
		if !strings.Contains(out, want) {
			t.Errorf("화면에 %q 가 없다", want)
		}
	}
	if strings.Contains(out, "묻는 중") {
		t.Error("결과가 왔는데 기다린다는 표시가 남아 있다")
	}
}

// 요청이 실패해도 화면이 살아 있어야 한다.
func TestQueueFailureShowsBadge(t *testing.T) {
	hm := New(musicapp.New())
	hm.LeaveHome()
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 32})
	m, _ = m.Update(musicapp.QueueMsgFor(state(t, m).app(), intent.Result{}, errors.New("model unavailable")))
	m, _ = m.Update(app.SayMsg{App: "music", Text: "model unavailable", Err: true})

	out := plain(m.View().Content)
	if !strings.Contains(out, "model unavailable") {
		t.Error("실패 사유가 로그에 없다")
	}
	if !strings.Contains(out, "Please Wait for Me") {
		t.Error("실패했다고 목록이 사라지면 안 된다")
	}
}
