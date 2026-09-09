package musicapp

import (
	"strings"
	"testing"
)

// enter 의 뜻은 하나지만("고른 곡부터 이 목록을 이어서 튼다"), 그 하나가
// 자리마다 다르게 **보인다**. 묶음에서는 여는 것이고, 아직 내 것이 아닌
// 곡에서는 담는 것이 먼저다. 화면이 그것을 말하지 않으면 눌러 보기 전에는
// 알 길이 없다 — 이 앱에서 가장 오래 헷갈렸던 자리다.
func TestHintNamesWhatEnterDoesHere(t *testing.T) {
	for _, c := range []struct {
		name string
		kind sectionKind
		want string
	}{
		{"묶음", secArtists, "open"},
		{"곡", secSongs, "play from here"},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := atSection(t, c.kind)
			if len(m.rows()) == 0 {
				t.Skip("픽스처에 줄이 없다")
			}
			if got := m.Hint(); !strings.Contains(got, c.want) {
				t.Errorf("%q 라고 말해야 하는데 %q 다", c.want, got)
			}
		})
	}
}

// 큐에서만 /remove 를 덧붙인다.
//
// 커서가 가리키는 것을 빼는 일이라 그 자리에서만 뜻이 있다. 다른 목록에서
// 내놓으면 무엇이 빠지는지가 모호해진다 — 라이브러리에서 지운다는 뜻으로도
// 읽힌다.
// atQueueOf 는 큐를 그대로 둔 채 화면만 Queue 로 옮긴다.
func atQueueOf(t *testing.T, m Model) Model {
	t.Helper()
	for i, s := range m.sections {
		if s.kind == secQueue {
			m.sectionIdx, m.listIdx, m.drill = i, 0, nil
			return m
		}
	}
	t.Fatal("Queue 섹션이 없다")
	return m
}

func TestRemoveIsOfferedOnlyInTheQueue(t *testing.T) {
	// 실제 경로로 큐를 채운다. playFrom 은 명령을 돌려줄 뿐이라 여기서는
	// Music.app 을 건드리지 않는다.
	src := atSection(t, secSongs)
	if len(src.rows()) == 0 {
		t.Skip("픽스처에 곡이 없다")
	}
	q, _ := src.playFrom(src.rows(), 0, "test")
	q = atQueueOf(t, q)
	if got := q.Hint(); !strings.Contains(got, "/remove") {
		t.Errorf("큐인데 빼는 법을 안 알려준다: %q", got)
	}

	s := atSection(t, secSongs)
	if len(s.rows()) == 0 {
		t.Skip("픽스처에 곡이 없다")
	}
	if got := s.Hint(); strings.Contains(got, "/remove") {
		t.Errorf("큐가 아닌데 빼는 법을 내놨다: %q", got)
	}
}

// 커서가 설 줄이 없으면 아무 말도 안 한다. 없는 일을 약속하지 않는다.
func TestHintIsSilentWithNothingUnderTheCursor(t *testing.T) {
	m := atSection(t, secQueue) // 큐는 시작할 때 비어 있다
	if len(m.rows()) != 0 {
		t.Skip("큐가 비어 있지 않다")
	}
	if got := m.Hint(); got != "" {
		t.Errorf("고른 것이 없는데 %q 라고 말한다", got)
	}
}
