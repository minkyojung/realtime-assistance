package cameraapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"amcli/tui/internal/app"
	tea "charm.land/bubbletea/v2"
)

// 진짜 카메라 대신 프레임을 뿜는 스크립트를 쓴다. 형식만 같으면
// 이쪽에서는 구별할 이유가 없다 — 그러라고 프로세스를 나눴다.
//
// 프레임마다 tick 파일에 한 바이트를 남긴다. 그 파일이 자라기를 멈추는 것이
// "카메라가 실제로 꺼졌다"의 유일하게 믿을 만한 증거다.
func fakeHelper(t *testing.T) (tick string) {
	t.Helper()
	dir := t.TempDir()
	tick = filepath.Join(dir, "tick")
	script := filepath.Join(dir, "camerad")

	body := "#!/bin/sh\n" +
		"printf 'CAM2'\n" +
		"while :; do\n" +
		"  printf 'P\\000\\240\\000\\170'\n" +
		"  head -c 57600 /dev/urandom\n" +
		"  printf . >> " + tick + "\n" +
		"  sleep 0.05\n" +
		"done\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CAMERAD", script)
	return tick
}

func ticks(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return len(b)
}

// 포커스를 받으면 켜지고, 벗어나면 꺼진다. 이 앱의 존재 이유에 붙은
// 유일한 안전 장치다 — 안 보는 동안 카메라가 살아 있으면 안 된다.
func TestFocusStartsBlurStops(t *testing.T) {
	tick := fakeHelper(t)

	msgs := make(chan tea.Msg, 64)
	m := New()
	m.Init(func(msg tea.Msg) { msgs <- msg })

	next, _ := m.Update(app.FocusMsg{})
	m = next.(Model)
	if err := m.Ready(); err != nil {
		t.Fatalf("헬퍼가 있는데 관문이 닫혔다: %v", err)
	}

	var frame frameMsg
	select {
	case msg := <-msgs:
		f, ok := msg.(frameMsg)
		if !ok {
			t.Fatalf("프레임이 올 자리에 %T 가 왔다: %v", msg, msg)
		}
		frame = f
	case <-time.After(5 * time.Second):
		t.Fatal("프레임이 오지 않았다")
	}
	if frame.w != 160 || frame.h != 120 {
		t.Fatalf("프레임 크기 %dx%d", frame.w, frame.h)
	}
	if len(frame.px) != frame.w*frame.h*3 {
		t.Fatalf("프레임 길이 %d, 원한 것 %d", len(frame.px), frame.w*frame.h*3)
	}

	next, _ = m.Update(frame)
	m = next.(Model)
	body := m.View(80, 24)
	if strings.Count(body, "\n")+1 != 24 {
		t.Errorf("본문이 24줄이 아니다: %d", strings.Count(body, "\n")+1)
	}
	if strings.TrimSpace(strip(body)) == "" {
		t.Error("프레임을 받았는데 화면이 비어 있다")
	}

	// 이제 나간다.
	next, _ = m.Update(app.BlurMsg{})
	m = next.(Model)
	if m.cam.running() {
		t.Fatal("나갔는데 헬퍼가 살아 있다")
	}
	if m.px != nil {
		t.Error("나갔는데 마지막 프레임이 남아 있다")
	}

	// 프로세스가 정말 죽었는가. 살아 있으면 tick 파일이 계속 자란다.
	before := ticks(t, tick)
	time.Sleep(400 * time.Millisecond)
	if after := ticks(t, tick); after != before {
		t.Errorf("헬퍼가 아직 프레임을 만들고 있다: %d → %d", before, after)
	}
}

// 두 번 포커스를 받아도 카메라를 두 개 열지 않는다.
func TestFocusTwiceOpensOneCamera(t *testing.T) {
	fakeHelper(t)
	msgs := make(chan tea.Msg, 64)
	m := New()
	m.Init(func(msg tea.Msg) { msgs <- msg })

	next, _ := m.Update(app.FocusMsg{})
	m = next.(Model)
	gen := m.cam.generation()
	next, _ = m.Update(app.FocusMsg{})
	m = next.(Model)
	if m.cam.generation() != gen {
		t.Errorf("헬퍼가 다시 떴다: 세대 %d → %d", gen, m.cam.generation())
	}
	m.cam.stopHelper()
}

// 껐다 켜기 전에 떠난 프레임은 버린다. 안 그러면 켜자마자
// 아까 화면이 한 장 스쳐 지나간다.
func TestStaleFrameIgnored(t *testing.T) {
	fakeHelper(t)
	m := New()
	m.Init(func(tea.Msg) {})
	next, _ := m.Update(app.FocusMsg{})
	m = next.(Model)
	defer m.cam.stopHelper()

	stale := frameMsg{gen: m.cam.generation() - 1, px: make([]byte, 160*120*3), w: 160, h: 120}
	next, _ = m.Update(stale)
	if next.(Model).px != nil {
		t.Error("이전 세대의 프레임을 받아들였다")
	}
}

// 헬퍼가 없으면 관문 화면이 뜬다. 죽거나 빈 화면이 되면 안 된다.
func TestMissingHelperShowsGate(t *testing.T) {
	t.Setenv("CAMERAD", filepath.Join(t.TempDir(), "없는파일"))
	m := New()
	if m.Ready() == nil {
		t.Fatal("헬퍼가 없는데 관문이 열렸다")
	}
	body := m.View(80, 24)
	if n := strings.Count(body, "\n") + 1; n != 24 {
		t.Errorf("관문 화면이 24줄이 아니다: %d", n)
	}
	if !strings.Contains(strip(body), "CAMERA") {
		t.Error("관문 화면이 이유를 말하지 않는다")
	}
}

// 어떤 상태에서든 본문은 주어진 높이를 정확히 채운다.
func TestViewAlwaysFillsHeight(t *testing.T) {
	t.Setenv("CAMERAD", filepath.Join(t.TempDir(), "없음"))
	broken := New()

	ok := New()
	ok.px, ok.fw, ok.fh, ok.on = gradient(), srcW, srcH, true

	waiting := New()
	waiting.on = true

	for name, m := range map[string]Model{"관문": broken, "대기": waiting, "재생": ok} {
		for _, size := range [][2]int{{80, 24}, {200, 50}, {20, 3}} {
			if name != "관문" {
				t.Setenv("CAMERAD", "/bin/echo") // 관문을 연다
			}
			body := m.View(size[0], size[1])
			if n := strings.Count(body, "\n") + 1; n != size[1] {
				t.Errorf("%s %dx%d: %d줄", name, size[0], size[1], n)
			}
		}
	}
}
