package musicapp

import (
	"fmt"
	"strings"

	"amcli/tui/internal/app"
	"amcli/tui/internal/data"
	"amcli/tui/internal/intent"
)

// 첫 화면 박스에 적는 다섯 줄 — 앞의 넷은 우리가 붙어 있는 바깥이고,
// 마지막 하나는 그래서 손에 쥔 것이다.
//
// 이것을 적는 이유는 자랑이 아니다. 이 제품의 주장은 "네 것만 튼다"인데,
// 그 말은 진짜 네 Music.app 에 붙어 있을 때만 사실이다. 붙어 있음을
// 보이는 가장 짧은 방법이 붙은 곳을 이름으로 적는 것이다.
//
// 그릴 때마다 불린다. 디스크도 망도 건드리지 않는다 — 전부 이미 들고
// 있는 값이거나 상수다. 그래서 가사 캐시 개수 같은 것은 여기 없다.
func (m Model) Facts() []app.Fact {
	facts := []app.Fact{
		{Group: sources, Name: "Music.app", Detail: m.factPlayer()},
		{Group: sources, Name: "Apple Music", Detail: m.factCatalog()},
		{Group: sources, Name: "LRCLIB", Detail: "synced lyrics"},
		{Group: sources, Name: "OpenAI", Detail: intent.Model() + " · " + intent.EditModel()},
		{Group: library, Detail: factLibrary(data.Lib())},
	}
	// 명령은 이름만 늘어놓는다. 무엇을 하는지는 팔레트가 말하고(`/`),
	// 여기가 답하는 질문은 "무엇을 칠 수 있는가"뿐이다.
	cmds := m.Commands()
	names := make([]string, 0, len(cmds))
	for _, c := range cmds {
		names = append(names, c.Name)
	}
	return append(facts,
		app.Fact{Group: commands, Detail: strings.Join(names, " ")},
		app.Fact{Group: commands, Detail: fmt.Sprintf("press / for all %d", len(cmds))})
}

// 묶음 제목. 넷째 줄이 다시 첫째 묶음으로 돌아가는 일이 없도록 상수로 둔다.
const (
	sources  = "Sources"
	library  = "Library"
	commands = "Commands"
)

// 막혀 있으면 끊겼다고만 적는다. 왜인지는 홈이 아래에 따로 띄운다
// (host/home.go 의 관문 사유). 여기까지 사유를 적으면 같은 문장이 한
// 화면에 두 번 뜬다 — 이 줄이 답하는 질문은 "붙어 있는가"뿐이다.
func (m Model) factPlayer() string {
	if m.playerErr != nil {
		return "not connected"
	}
	return "play · queue · playlists"
}

// 카탈로그는 설정이 없으면 기능만 없다. 앱은 그대로 도므로 실패가 아니라
// 상태로 적는다.
func (m Model) factCatalog() string {
	switch {
	case m.cat == nil:
		return "not configured"
	case m.catLogin:
		return "waiting for the browser"
	case m.cat.UserToken == "":
		return "catalog search · /login to add"
	}
	return "catalog search · signed in"
}

func factLibrary(l *data.Library) string {
	if len(l.Tracks) == 0 {
		return "reading Music.app…"
	}
	// 아티스트·앨범 수는 세지 않는다. data.Artists 는 부를 때마다 맵을 짓고
	// 정렬하는데(data/library.go), 이 줄은 스피너가 도는 매 프레임 그려진다.
	// 여기서 셀 수 있는 것은 len() 으로 끝나는 것뿐이다.
	s := comma(len(l.Tracks)) + " tracks"
	if n := len(l.Playlists); n > 0 {
		s += fmt.Sprintf(" · %d playlists", n)
	}
	return s
}

// 12481 을 12,481 로. 세 자리마다 끊어야 자릿수가 한눈에 읽힌다 —
// style.Tokens 의 12.4k 는 상태줄처럼 좁은 자리를 위한 것이고,
// 여기는 "얼마나 갖고 있는가"를 말하는 자리라 정확한 수가 맞다.
func comma(n int) string {
	s := fmt.Sprint(n)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}
