package musicapp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"amcli/tui/internal/music"
	tea "charm.land/bubbletea/v2"
)

// 가짜 Music.app.
//
// 무엇을 시켰는지 순서대로 적어 둔다. 이것으로 재생·큐 경로를 Music.app
// 없이 끝까지 돈다 — music.Player 경계가 값을 내는 자리다. 이전에는 이
// 경로에 테스트가 없었다. 실제 앱을 pty 로 띄워야만 확인이 됐다.
type fakePlayer struct {
	calls []string
	state music.PlayerState
	fail  error // 켜 두면 모든 쓰기가 이 오류로 실패한다
}

func (f *fakePlayer) note(s string) error { f.calls = append(f.calls, s); return f.fail }

func (f *fakePlayer) Status() (music.PlayerState, error) { f.note("status"); return f.state, nil }
func (f *fakePlayer) PlayPause() error                   { return f.note("playpause") }
func (f *fakePlayer) Next() error                        { return f.note("next") }
func (f *fakePlayer) Previous() error                    { return f.note("previous") }
func (f *fakePlayer) PlayPersistentID(id string) error   { return f.note("play " + id) }
func (f *fakePlayer) PlayByTitleArtist(t, a string) error {
	return f.note("play " + t + " — " + a)
}
func (f *fakePlayer) CreatePlaylist(name string, ids []string) error {
	return f.note("playlist " + name + " " + strings.Join(ids, ","))
}
func (f *fakePlayer) ReplaceQueue(prev string, ids []string) (string, error) {
	if err := f.note("queue " + strings.Join(ids, ",")); err != nil {
		return "", err
	}
	return "QUEUE-PID", nil
}
func (f *fakePlayer) PlayQueueAt(pid string, n, pos int) error {
	return f.note("play-queue " + pid)
}
func (f *fakePlayer) RemoveQueueTrack(pid string, n int, adv bool) error {
	return f.note("remove " + pid)
}
func (f *fakePlayer) RewriteQueueTail(pid string, from int, ids []string) error {
	return f.note("rewrite " + pid)
}
func (f *fakePlayer) LibraryCount() (int, error)                  { return 0, nil }
func (f *fakePlayer) LibraryIDs() (map[string]bool, error)        { return map[string]bool{}, nil }
func (f *fakePlayer) DumpLibrary(context.Context) ([]byte, error) { return nil, errors.New("no dump") }
func (f *fakePlayer) Artwork(context.Context) ([]byte, error)     { return nil, music.ErrNoArtwork }
func (f *fakePlayer) SetShuffle(on bool) error                    { return f.note("shuffle") }
func (f *fakePlayer) SetRepeat(r music.Repeat) error              { return f.note("repeat") }
func (f *fakePlayer) SetVolume(n int) error                       { return f.note("volume") }
func (f *fakePlayer) SetFavorite(on bool) error                   { return f.note("favorite") }
func (f *fakePlayer) SetRating(stars int) error                   { return f.note("rating") }

var _ music.Player = (*fakePlayer)(nil)

// sandboxHome 은 큐 PID 같은 파일이 진짜 설정 폴더에 안 가게 한다.
func sandboxHome(t *testing.T) { t.Helper(); t.Setenv("HOME", t.TempDir()) }

// drain 는 Cmd 를 실제로 돌려 나온 메시지를 전부 모은다. Batch 도 편다.
func drain(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	switch v := cmd().(type) {
	case tea.BatchMsg:
		var out []tea.Msg
		for _, c := range v {
			out = append(out, drain(c)...)
		}
		return out
	case nil:
		return nil
	default:
		return []tea.Msg{v}
	}
}

// 곡 위에서 enter — 큐가 Music.app 안에 실제로 만들어지고, 그 큐가 재생된다.
// 고른 곡부터, 목록 순서 그대로.
func TestEnterWritesTheQueueAndPlaysIt(t *testing.T) {
	sandboxHome(t)
	fp := &fakePlayer{}
	m := atRow(t, secSongs, 3).WithPlayer(fp)
	rows := m.rows()

	next, cmd := m.playSelected()
	msgs := drain(cmd)

	if len(fp.calls) < 2 {
		t.Fatalf("Music.app 에 시킨 것: %v — 큐 쓰기와 재생이어야 한다", fp.calls)
	}
	wantFirst := *rows[3].track.PersistentId
	if !strings.HasPrefix(fp.calls[0], "queue "+wantFirst) {
		t.Errorf("큐의 첫 곡이 고른 곡이 아니다: %q", fp.calls[0])
	}
	if fp.calls[1] != "play-queue QUEUE-PID" {
		t.Errorf("만든 큐를 트는 게 아니다: %q", fp.calls[1])
	}
	// 결과가 모델에 앉는다 — 이제 큐를 고칠 수 있는 상태다.
	mm := next.(Model)
	for _, msg := range msgs {
		n, _ := mm.Update(msg)
		mm = n.(Model)
	}
	if mm.queuePID != "QUEUE-PID" {
		t.Errorf("큐 PID 가 모델에 안 남았다: %q", mm.queuePID)
	}
}

// Music.app 이 거절하면 화면이 그것을 안다. 조용히 넘어가지 않는다.
func TestQueueWriteFailureIsReported(t *testing.T) {
	sandboxHome(t)
	fp := &fakePlayer{fail: errors.New("music app said no")}
	m := atRow(t, secSongs, 0).WithPlayer(fp)

	_, cmd := m.playSelected()
	msgs := drain(cmd)
	var got queueWrittenMsg
	for _, msg := range msgs {
		if q, ok := msg.(queueWrittenMsg); ok {
			got = q
		}
	}
	if got.err == nil {
		t.Fatal("실패가 메시지로 안 왔다")
	}
}

// shift+↓ 는 Music.app 의 재생/일시정지다. 딱 한 번.
func TestPlayPauseKeyReachesTheDevice(t *testing.T) {
	fp := &fakePlayer{}
	m := New().WithPlayer(fp)
	m.bodyH = 20

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModShift})
	drain(cmd)

	n := 0
	for _, c := range fp.calls {
		if c == "playpause" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("playpause 가 %d번 갔다: %v", n, fp.calls)
	}
}

// 폴링이 읽은 상태가 재생 바에 그려진다.
func TestPolledStateReachesTheScreen(t *testing.T) {
	fp := &fakePlayer{state: music.PlayerState{
		Playing: true, PersistentID: "5D6BA5B487F37235",
		Title: "Blood Bank", Artist: "Bon Iver", PositionMs: 1000, DurationMs: 284132,
	}}
	m := New().WithPlayer(fp)

	next, _ := m.Update(pollStatus(fp))
	if v := next.(Model).View(100, 24); !strings.Contains(v, "Blood Bank") {
		t.Errorf("폴링한 곡이 화면에 없다:\n%s", v)
	}
}
