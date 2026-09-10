package musicapp

import (
	"errors"
	"strconv"
	"strings"

	"amcli/tui/internal/app"
	"amcli/tui/internal/music"
	"amcli/tui/internal/style"
	tea "charm.land/bubbletea/v2"
)

var (
	errBadRepeat = errors.New("Needs off, one or all: /repeat <off|one|all>")
	errBadVolume = errors.New("Needs a number 0-100: /volume <0-100>")
	errBadRating = errors.New("Needs a number 0-5: /rate <0-5>")
)

// Music.app 의 버튼들을 명령으로 연다.
//
// 이 제품이 파는 것은 리모컨 위의 층이지만(docs/01 §2), 그렇다고 리모컨이
// 없어도 된다는 뜻은 아니다. 섞기 하나 켜자고 Music.app 을 열어야 하면
// "터미널 안에서 끝난다"가 거기서 깨진다.
//
// 결과를 SayMsg 로 그대로 돌려준다. 앱에 새 상태를 두지 않으므로 Update 에
// 손댈 것이 없고, 진짜 상태는 다음 폴링이 알려준다 — 우리가 켰다고 기억하는
// 대신 매번 읽는다(music/music.go PlayerState).

// modeCommands 는 이 앱이 여는 Music.app 버튼들이다.
//
// 지금 상태를 아는 채로 만들어진다. 그래서 /shuffle 이 토글이 될 수 있다 —
// 켤지 끌지를 부르는 시점에 이미 안다.
func (m Model) modeCommands() []app.Command {
	shuffle := "on"
	if m.live.Shuffle {
		shuffle = "off"
	}
	love := "add"
	if m.live.Favorited {
		love = "remove"
	}
	return []app.Command{
		{Name: "/shuffle", Help: "turn shuffle " + shuffle,
			Run: func(string) tea.Cmd { return cmdShuffle(m.player, !m.live.Shuffle) }},
		{Name: "/repeat", Arg: "<off|one|all>", Help: "repeat nothing, this track, or the queue",
			Run: func(arg string) tea.Cmd { return cmdRepeat(m.player, arg) }},
		{Name: "/volume", Arg: "<0-100>", Help: "set the volume",
			Run: func(arg string) tea.Cmd { return cmdVolume(m.player, arg) }},
		{Name: "/love", Help: love + " a heart on the track playing now",
			Run: func(string) tea.Cmd { return cmdFavorite(m.player, !m.live.Favorited) }},
		{Name: "/rate", Arg: "<0-5>", Help: "give the track playing now a star rating",
			Run: func(arg string) tea.Cmd { return cmdRate(m.player, arg) }},
	}
}

// marks 는 상태줄에 붙일 "지금 켜져 있는 것"이다.
//
// **기본값이면 아무것도 안 그린다.** 섞기가 꺼져 있고 반복이 없으면 평소
// 화면에서 한 칸도 늘어나지 않는다. "화면에 상시로 자리를 주지 않는다"
// (docs/07 2절) — 사이드바를 없앤 것과 같은 규칙이다.
//
// 켜 두고 잊으면 고장으로 읽히는 것들이라 표시가 필요하다. 순서대로 안
// 나오는데 왜 그런지 화면 어디에도 없으면, 사용자가 의심할 것은 앱뿐이다.
func (m Model) marks() string {
	var out []string
	if m.live.Shuffle {
		out = append(out, "⇄")
	}
	switch m.live.Repeat {
	case music.RepeatOne:
		out = append(out, "↻1")
	case music.RepeatAll:
		out = append(out, "↻")
	}
	if m.live.Favorited {
		out = append(out, "♥")
	}
	if len(out) == 0 {
		return ""
	}
	return style.BrandSoft.Render(strings.Join(out, " "))
}

// 아래 명령들은 결과를 기다리지 않는다. 다음 폴링이 진짜 상태를 알려준다.

func cmdShuffle(p music.Player, on bool) tea.Cmd {
	return modeCmd(func() error { return p.SetShuffle(on) }, "Shuffle "+onOff(on))
}

func cmdRepeat(p music.Player, arg string) tea.Cmd {
	r := music.Repeat(strings.ToLower(strings.TrimSpace(arg)))
	if !r.Valid() {
		return sayErr(errBadRepeat)
	}
	return modeCmd(func() error { return p.SetRepeat(r) }, "Repeat "+string(r))
}

func cmdVolume(p music.Player, arg string) tea.Cmd {
	n, err := strconv.Atoi(strings.TrimSpace(arg))
	if err != nil {
		return sayErr(errBadVolume)
	}
	return modeCmd(func() error { return p.SetVolume(n) }, "Volume "+strconv.Itoa(n))
}

// cmdFavorite 은 지금 나오는 곡에 하트를 켜고 끈다.
//
// 커서가 아니라 재생 중인 곡이 대상이다. /remove 는 목록을 손보는 일이고
// 이것은 소리에 반응하는 일이라, 듣다가 "이거 좋네" 하는 순간에 커서가
// 어디 있는지는 상관이 없다.
func cmdFavorite(p music.Player, on bool) tea.Cmd {
	word := "Loved"
	if !on {
		word = "Unloved"
	}
	return modeCmd(func() error { return p.SetFavorite(on) }, word+" this one")
}

func cmdRate(p music.Player, arg string) tea.Cmd {
	n, err := strconv.Atoi(strings.TrimSpace(arg))
	if err != nil {
		return sayErr(errBadRating)
	}
	return modeCmd(func() error { return p.SetRating(n) }, "Rated "+strings.Repeat("★", n)+strings.Repeat("☆", 5-n))
}

// modeCmd 는 한 일을 로그에 남긴다. 실패도 남긴다 — 조용히 실패하면
// 사용자는 켜졌다고 믿고 다음 행동을 한다.
//
// 함수로 받는다. 값으로 받으면 Cmd 를 만드는 순간, 즉 Update 안에서
// AppleScript 가 돌아 입력창이 멎는다.
func modeCmd(do func() error, said string) tea.Cmd {
	return func() tea.Msg {
		if err := do(); err != nil {
			return app.SayMsg{App: "music", Text: err.Error(), Err: true}
		}
		return app.SayMsg{App: "music", Text: said}
	}
}

func sayErr(err error) tea.Cmd {
	return func() tea.Msg {
		return app.SayMsg{App: "music", Text: err.Error(), Err: true}
	}
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}
