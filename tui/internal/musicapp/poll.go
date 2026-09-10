package musicapp

import (
	"context"
	"time"

	"amcli/tui/internal/api"
	"amcli/tui/internal/app"
	"amcli/tui/internal/data"
	"amcli/tui/internal/intent"
	"amcli/tui/internal/music"
	tea "charm.land/bubbletea/v2"
)

// Music.app 은 우리에게 알림을 주지 않으므로 폴링한다.
// 1초는 3초 이내 이탈(skip_early)을 잡기에 충분한 주기다. docs/03 7절.
const pollInterval = time.Second

type tickMsg struct{}

// queueMsg 는 의도 층이 만들어낸 큐다. 실패도 여기로 온다.
//
// seq 는 "몇 번째 물음의 답인가"다. esc 로 그만두면 번호가 올라가므로,
// 이미 날아간 요청의 답이 뒤늦게 와도 화면을 건드리지 못한다.
type queueMsg struct {
	seq int
	res intent.Result
	err error

	// add 가 켜져 있으면 **갈아끼우지 않고 붙인다.** 선곡은 같은 길을
	// 쓰지만 결과를 앉히는 법이 다르다 — 듣던 곡을 끊느냐 마느냐가 갈린다.
	add   bool
	atEnd bool // 붙일 자리. 꺼져 있으면 지금 곡 바로 뒤다
}

// 선곡은 실측 7초다. 상한을 크게 잡아도 되는 이유는 이제 사용자가
// esc 로 언제든 그만둘 수 있기 때문이다.
const askTimeout = 90 * time.Second

// cmdBuildQueue — 자연어 한 줄을 큐로 바꾼다. 입력창을 막지 않도록 Cmd 로 돈다.
//
// ctx 를 바깥에서 받는다. 안에서 만들면 취소 손잡이가 이 함수와 함께
// 사라져서, 도는 동안 끊을 방법이 없다.
func cmdBuildQueue(ctx context.Context, seq int, prompt string, library []api.Track, extras []api.CatalogTrack, cur intent.Current) tea.Cmd {
	return func() tea.Msg {
		res, err := intent.Build(ctx, prompt, library, extras, reactions(), cur, time.Now())
		return queueMsg{seq: seq, res: res, err: err}
	}
}

// cmdAddTracks — 같은 선곡을 돌리되 결과를 큐에 **붙인다.**
//
// 고르는 일은 한 곳뿐이다. 붙이기 위해 선곡을 따로 만들면 두 벌이 되고,
// 하나를 고칠 때마다 다른 하나가 뒤처진다.
func cmdAddTracks(ctx context.Context, seq int, prompt string, library []api.Track, extras []api.CatalogTrack, cur intent.Current, atEnd bool) tea.Cmd {
	return func() tea.Msg {
		res, err := intent.Build(ctx, prompt, library, extras, reactions(), cur, time.Now())
		return queueMsg{seq: seq, res: res, err: err, add: true, atEnd: atEnd}
	}
}

// reactions 는 쌓아 둔 재생 기록을 곡별로 접어 온다.
//
// 부를 때마다 파일을 읽는다. 한 줄이 200바이트라 1년치가 11MB 남짓이고,
// 선곡은 실측 7초짜리 일이라 이 읽기는 티가 안 난다. 캐시를 두면 방금
// 넘긴 곡이 이번 선곡에 안 잡히는데, 그것이 제일 알고 싶은 정보다.
//
// 못 읽어도 선곡은 돈다. 반응은 있으면 더 좋은 것이지 없으면 못 하는
// 것이 아니다 — 첫날에는 아무 기록도 없다.
func reactions() map[int64]data.Reaction {
	ps, err := data.LoadPlays()
	if err != nil {
		return nil
	}
	return data.Reactions(ps)
}

type statusMsg struct {
	state music.PlayerState
	err   error
}

func tick() tea.Cmd {
	return tea.Tick(pollInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

func fetchStatus() tea.Msg {
	st, err := music.Status()
	return statusMsg{state: st, err: err}
}

// 아래 조작은 결과를 기다리지 않는다. 다음 폴링이 진짜 상태를 알려준다.
func cmdPlayPause() tea.Cmd {
	return func() tea.Msg { music.PlayPause(); return fetchStatus() }
}

func cmdNext() tea.Cmd {
	return func() tea.Msg { music.Next(); return fetchStatus() }
}

func cmdPrevious() tea.Cmd {
	return func() tea.Msg { music.Previous(); return fetchStatus() }
}

func cmdOpenSettings() tea.Cmd {
	return func() tea.Msg { music.OpenSettings(); return nil }
}

// ProbeStatus 는 헤드리스 확인용이다. 실제 Music.app 상태를 한 번 읽는다.
func ProbeStatus() tea.Msg { return fetchStatus() }

// DrainForQueue 는 헤드리스 확인용이다. Batch 안에서 큐 생성 Cmd 를 찾아 실행한다.
func DrainForQueue(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if b, ok := msg.(tea.BatchMsg); ok {
		for _, c := range b {
			if m := DrainForQueue(c); m != nil {
				return m
			}
		}
		return nil
	}
	if _, ok := msg.(queueMsg); ok {
		return msg
	}
	return nil
}

// savedMsg — 큐를 Apple Music 플레이리스트로 저장한 결과 (F9).
type savedMsg struct {
	name string
	err  error
}

func cmdSavePlaylist(name string, tracks []api.Track) tea.Cmd {
	return func() tea.Msg {
		ids := make([]string, 0, len(tracks))
		for _, t := range tracks {
			if t.PersistentId != nil {
				ids = append(ids, *t.PersistentId)
			}
		}
		return savedMsg{name: name, err: music.CreatePlaylist(name, ids)}
	}
}

// 아래 둘은 테스트에서 앱 안쪽 상태를 흘려보내기 위한 것이다.
// 실제 API 나 Music.app 을 부르지 않고 화면을 확인할 수 있게 한다.

// StatusMsgFor 는 Music.app 폴링 결과를 흉내 낸다.
func StatusMsgFor(state music.PlayerState, err error) tea.Msg {
	return statusMsg{state: state, err: err}
}

// QueueMsgFor 는 의도 층의 결과를 흉내 낸다.
//
// 앱을 받는 이유는 답에 번호가 붙기 때문이다. 그만둔 요청의 답은 버려지므로
// (model.go), 지금 기다리는 물음의 번호를 그대로 달아 줘야 화면에 앉는다.
func QueueMsgFor(a app.App, res intent.Result, err error) tea.Msg {
	seq := 0
	if m, ok := a.(Model); ok {
		seq = m.ask.seq
	}
	return queueMsg{seq: seq, res: res, err: err}
}

// libraryMsg — 라이브러리 스냅샷이 도착했다. 캐시와 Music.app 두 경로로 온다.
type libraryMsg struct {
	lib      *data.Library
	live     bool // Music.app 에서 직접 읽은 것인지
	announce bool // /reload 로 부른 것인지 — 시작할 때마다 로그를 더럽히지 않는다
	err      error
}

// cmdLoadCache 는 지난번에 읽어둔 스냅샷을 꺼낸다. 몇 ms 다.
func cmdLoadCache() tea.Msg {
	l, err := data.LoadCache()
	return libraryMsg{lib: l, err: err}
}

// cmdDumpLibrary 는 Music.app 라이브러리를 통째로 읽는다. 몇 초다.
//
// 읽어낸 것은 캐시에 남긴다. 다음 시작이 즉시 그려지도록.
func cmdDumpLibrary(announce bool) tea.Cmd {
	return func() tea.Msg {
		b, err := music.DumpLibrary(context.Background())
		if err != nil {
			return libraryMsg{live: true, announce: announce, err: err}
		}
		l, err := data.FromDump(b)
		if err != nil {
			return libraryMsg{live: true, announce: announce, err: err}
		}
		_ = data.SaveCache(l) // 실패해도 다음 시작이 조금 느릴 뿐이다
		return libraryMsg{lib: l, live: true, announce: announce}
	}
}

// queueWrittenMsg — 큐 플레이리스트를 Music.app 에 만들고 재생까지 시켰다.
type queueWrittenMsg struct {
	pid string
	err error
}

// cmdWriteQueue 는 큐를 Music.app 안에 실제로 만든다.
//
// AppleScript 로 곡마다 duplicate 를 돌리므로 몇 초가 걸린다. 그래서 Cmd 다 —
// 그동안 입력창은 계속 살아 있어야 한다.
//
// 지난번 플레이리스트의 ID 를 넘기는 이유는 그것만 지우기 위해서다.
// 이름으로 지우면 사용자가 우연히 같은 이름으로 만든 것을 날린다(music/queue.go).
func cmdWriteQueue(persistentIDs []string, start, positionSec int) tea.Cmd {
	return func() tea.Msg {
		pid, err := music.ReplaceQueue(data.LoadQueuePID(), persistentIDs)
		if err != nil {
			return queueWrittenMsg{err: err}
		}
		// 적어 두지 못해도 재생은 시킨다. 다음번에 옛 플레이리스트가
		// 하나 남을 뿐이고, 그것이 남의 것을 지우는 것보다 낫다.
		_ = data.SaveQueuePID(pid)
		if err := music.PlayQueueAt(pid, start, positionSec); err != nil {
			return queueWrittenMsg{err: err}
		}
		return queueWrittenMsg{pid: pid}
	}
}

// cmdRemoveQueueTrack 은 큐에서 곡 하나를 뺀다.
//
// 통째로 다시 쓰지 않으므로 음악이 끊기지 않는다. advance 는 지금 나오는
// 곡을 빼는 중이라는 뜻이고, 그때는 지우기 전에 다음 곡으로 넘어간다.
func cmdRemoveQueueTrack(pid string, n int, advance bool) tea.Cmd {
	return func() tea.Msg {
		if err := music.RemoveQueueTrack(pid, n, advance); err != nil {
			return queueWrittenMsg{err: err}
		}
		// 뺀 자리만큼 화면과 플레이리스트가 같이 밀렸다. 짝은 그대로다.
		return queueWrittenMsg{pid: pid}
	}
}

// stepMsg — 모델을 한 번 부른 결과다. 도구를 부르겠다고 했거나, 그냥 말했거나.
type stepMsg struct {
	seq  int
	chat intent.Chat
	step intent.Step
	err  error
}

// cmdStep 은 모델에게 한 걸음을 묻는다.
//
// 라이브러리를 넘기지 않으므로 실측 1초다. 무거운 일은 도구 안에서 벌어진다.
func cmdStep(ctx context.Context, seq int, chat intent.Chat, tools []intent.Tool) tea.Cmd {
	return func() tea.Msg {
		next, s, err := chat.Step(ctx, tools)
		return stepMsg{seq: seq, chat: next, step: s, err: err}
	}
}
