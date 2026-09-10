package musicapp

import (
	"strings"
	"testing"

	"amcli/tui/internal/app"
	"amcli/tui/internal/intent"
	tea "charm.land/bubbletea/v2"
)

// 담긴 곡을 "담지 못했다"고 말하면 안 된다.
//
// 실측: "야생화 틀어줘" 가 "1 track could not be added" 로 끝났는데 곡은
// 라이브러리에 멀쩡히 들어가 있었다. 애플이 받아들이는 데는 0.5초,
// Music.app 이 받아오는 데는 수십 초에서 몇 분이 걸린다. 우리가 19.5초
// 만에 포기하고 실패라고 말한 것이다.
//
// 사용자가 할 일이 다르다 — 담지 못한 것은 다시 시켜야 하고,
// 동기화 중인 것은 기다리면 온다.
func TestSyncingIsNotFailure(t *testing.T) {
	said := saysOf(t, queueMsg{syncing: 1, res: intent.Result{Title: "야생화"}})

	if strings.Contains(said, "could not be added") {
		t.Errorf("담긴 곡을 못 담았다고 한다: %q", said)
	}
	if !strings.Contains(said, "Added") {
		t.Errorf("담겼다는 말이 없다: %q", said)
	}
	// 기다리면 온다는 것과, 안 오면 무엇을 볼지까지 말한다.
	if !strings.Contains(said, "synced") {
		t.Errorf("왜 큐에 없는지 설명이 없다: %q", said)
	}
	if !strings.Contains(said, "Sync Library") {
		t.Errorf("끝내 안 올 때 볼 곳을 안 알려준다: %q", said)
	}
}

// 진짜로 못 담은 것은 여전히 못 담았다고 말한다. 위 수정이 실패를
// 통째로 삼켜 버리면 안 된다.
func TestDroppedStillSaysItFailed(t *testing.T) {
	said := saysOf(t, queueMsg{dropped: 2, res: intent.Result{Title: "무엇"}})

	if !strings.Contains(said, "could not be added") {
		t.Errorf("못 담은 것을 안 알린다: %q", said)
	}
	if strings.Contains(said, "Added") {
		t.Errorf("못 담았는데 담았다고 한다: %q", said)
	}
}

// 한 곡이면 한 곡의 말로 쓴다. "1 tracks are" 는 사람이 쓴 문장이 아니다.
func TestOneTrackReadsLikeOneTrack(t *testing.T) {
	one := saysOf(t, queueMsg{syncing: 1})
	for _, bad := range []string{"1 tracks", "them yet", "they are not"} {
		if strings.Contains(one, bad) {
			t.Errorf("한 곡인데 %q 라고 쓴다: %q", bad, one)
		}
	}
	many := saysOf(t, queueMsg{syncing: 3})
	if !strings.Contains(many, "3 tracks") {
		t.Errorf("여러 곡인데 수를 안 맞춘다: %q", many)
	}
}

// saysOf 는 이 답이 화면에 남기는 말을 전부 모은다.
func saysOf(t *testing.T, msg queueMsg) string {
	t.Helper()
	m := askedSomething(t)
	msg.seq = m.ask.seq
	_, cmd := m.Update(msg)

	var b strings.Builder
	collect(cmd, &b)
	return b.String()
}

func collect(cmd tea.Cmd, b *strings.Builder) {
	if cmd == nil {
		return
	}
	switch v := cmd().(type) {
	case app.SayMsg:
		b.WriteString(v.Text + "\n")
	case tea.BatchMsg:
		for _, c := range v {
			collect(c, b)
		}
	}
}
