package host

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"amcli/tui/internal/cameraapp"
	"amcli/tui/internal/musicapp"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// 카메라 앱을 호스트 안에 넣고 실제로 한 프레임을 흘려본다.
//
// 단위 테스트는 ascii() 가 폭을 안 넘는 것까지만 안다. 그 문자열이 호스트의
// 패딩과 룰 사이에 끼워졌을 때도 안 넘는지는 여기서만 확인할 수 있다 —
// 트루컬러 이스케이프가 폭 계산을 어긋나게 하기 딱 좋은 자리다.
func TestCameraFrameFitsInsideHost(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "camerad")
	body := "#!/bin/sh\n" +
		"printf 'CAM2'\n" +
		"while :; do\n" +
		"  printf 'P\\000\\240\\000\\170'\n" +
		"  head -c 57600 /dev/urandom\n" +
		"  sleep 0.05\n" +
		"done\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CAMERAD", script)

	msgs := make(chan tea.Msg, 64)
	hm := New(musicapp.New(), cameraapp.New())
	hm.LeaveHome()
	hm.SetSend(func(msg tea.Msg) { msgs <- msg })
	hm.Init()

	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 96, Height: 32})
	m, _ = m.Update(switchAppMsg{index: 1}) // 카메라로 간다 → 여기서 켜진다

	var frame tea.Msg
	deadline := time.After(5 * time.Second)
	for frame == nil {
		select {
		case msg := <-msgs:
			// 헬퍼가 보내는 메시지는 카메라 앱의 것이라 여기서는 불투명하다.
			// 그대로 호스트에 넣으면 지금 앱에게 흘러간다.
			frame = msg
		case <-deadline:
			t.Fatal("프레임이 오지 않았다")
		}
	}
	m, _ = m.Update(frame)

	out := m.View().Content
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		if got := lipgloss.Width(line); got > 96 {
			t.Errorf("%d번째 줄이 %d칸으로 넘침", i+1, got)
		}
	}
	if len(lines) != 32 {
		t.Errorf("화면이 %d줄 — 원한 것 32", len(lines))
	}
	// 프레임이 실제로 그려졌는지. 비어 있으면 위의 검사는 다 통과한다.
	if !strings.Contains(out, "\x1b[38;2;") {
		t.Error("본문에 카메라 프레임이 없다")
	}

	// 켜져 있는 동안에는 상태줄이 그것을 말한다. 배경 앱은 이름만 모이므로
	// 이 표시가 존재할 수 있는 자리는 여기뿐이고, 카메라는 보일 때만 켜지니
	// 그것으로 충분하다. 실제로 꺼지는지는 cameraapp 쪽에서 확인한다.
	if !strings.Contains(plain(out), "● 켜짐") {
		t.Error("보고 있는데 상태줄이 켜졌다고 말하지 않는다")
	}
}
