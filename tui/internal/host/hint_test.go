package host

import (
	"strings"
	"testing"

	"amcli/tui/internal/app"
	"amcli/tui/internal/secrets"

	tea "charm.land/bubbletea/v2"
)

// 입력창은 지금 커서가 놓인 것으로 무엇을 할 수 있는지 말한다.
//
// enter 의 뜻은 하나지만 그 하나가 자리마다 다르게 보인다 — 큐의 곡에서는
// "여기부터 튼다", 카탈로그의 곡에서는 "담고 튼다". 눌러 보기 전에 알 수
// 있어야 한다는 것이 이 자리의 존재 이유다.
// 안내문은 Ask 모드의 것이다. 키가 없으면 Search 로 시작하므로 옮겨 준다.
func askHost(t *testing.T, hint string) tea.Model {
	t.Helper()
	m, _ := twoAppHost()
	if state(t, m).mode != modePrompt {
		m, _ = m.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	}
	return setHint(t, m, hint)
}

func setHint(t *testing.T, m tea.Model, hint string) tea.Model {
	t.Helper()
	st := state(t, m)
	sa, ok := st.apps[st.current].(stubApp)
	if !ok {
		t.Fatalf("앞에 나온 앱이 stubApp 이 아니다: %T", st.apps[st.current])
	}
	sa.hint = hint
	st.apps[st.current] = sa
	return st
}

func TestInputSaysWhatEnterDoes(t *testing.T) {
	m := askHost(t, "enter  play from here")

	out := plain(m.View().Content)
	if !strings.Contains(out, "enter  play from here") {
		t.Error("커서가 놓인 줄에서 무엇을 할 수 있는지 화면이 말하지 않는다")
	}
	if strings.Contains(out, "Ask for anything") {
		t.Error("앱이 할 말이 있는데도 아무것도 안 알려주는 기본 문구가 남아 있다")
	}
}

// 안내문은 **그릴 때** 정해진다.
//
// applyMode 는 모드가 바뀔 때만 불린다. 앱이 말을 바꿨는데 모드는 그대로인
// 것이 흔한 경우이고(화살표 한 번), 미리 적어 두면 그때 갱신을 빠뜨린다.
func TestHintFollowsAppWithoutModeChange(t *testing.T) {
	m := askHost(t, "enter  open")
	if out := plain(m.View().Content); !strings.Contains(out, "enter  open") {
		t.Fatal("첫 말이 안 나온다")
	}

	// 모드는 그대로 두고 앱의 말만 바꾼다.
	m = setHint(t, m, "enter  add it and play")
	out := plain(m.View().Content)
	if !strings.Contains(out, "enter  add it and play") {
		t.Error("앱이 말을 바꿨는데 입력창이 옛 말을 붙들고 있다")
	}
	if strings.Contains(out, "enter  open") {
		t.Error("옛 말이 남아 있다")
	}
}

// 앱이 할 말이 없으면 그 자리는 원래대로 돌아간다.
func TestHintFallsBackWhenAppIsSilent(t *testing.T) {
	m := askHost(t, "")
	out := plain(m.View().Content)
	if !strings.Contains(out, "Ask for anything") && !strings.Contains(out, "AI is off") {
		t.Error("앱이 말이 없는데 자리가 비었다")
	}
}

// AI 가 꺼져 있어도 enter 가 하는 일은 계속 말한다.
//
// enter 는 키 없이도 멀쩡히 동작한다. 켜는 법이 그 자리를 통째로 먹으면
// 키를 넣기 전까지는 목록을 어떻게 쓰는지 알 길이 없어진다.
func TestOfflineNoticeDoesNotEatTheHint(t *testing.T) {
	if secrets.HasOpenAIKey() {
		t.Skip("키가 있으면 꺼진 화면을 볼 수 없다")
	}
	m := askHost(t, "enter  play from here")
	out := plain(m.View().Content)
	if !strings.Contains(out, "enter  play from here") {
		t.Error("AI 가 꺼졌다고 enter 안내까지 사라졌다")
	}
	if !strings.Contains(out, "/ai") {
		t.Error("AI 를 켜는 법이 어디에도 없다")
	}
}

// 글자를 치면 안내문은 비켜난다. 자리를 새로 안 먹는 것이 이 설계의 값이다.
func TestHintYieldsToTyping(t *testing.T) {
	m := askHost(t, "enter  play from here")
	for _, r := range "play" {
		m, _ = key(m, r)
	}
	if out := plain(m.View().Content); strings.Contains(out, "enter  play from here") {
		t.Error("치는 중에도 안내문이 남아 글자와 겹친다")
	}
}

var _ app.App = stubApp{}
