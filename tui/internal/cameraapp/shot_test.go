package cameraapp

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"amcli/tui/internal/app"
	tea "charm.land/bubbletea/v2"
)

// 세로 줄무늬. 좌우가 뒤집혔는지 한 픽셀만 봐도 알 수 있다.
func stripes(w, h int) []byte {
	px := make([]byte, w*h*3)
	for y := range h {
		for x := range w {
			p := (y*w + x) * 3
			px[p] = byte(x) // R 에 x 좌표를 심는다
			px[p+1] = 0x40
			px[p+2] = 0x80
		}
	}
	return px
}

func downloads(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return filepath.Join(home, "Downloads")
}

func TestSavePhotoWritesMirroredPNG(t *testing.T) {
	dir := downloads(t)
	const w, h = 64, 48

	path, err := savePhoto(stripes(w, h), w, h, time.Date(2026, 9, 9, 11, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != dir {
		t.Errorf("%s 에 저장됐다 — 원한 곳 %s", path, dir)
	}
	if base := filepath.Base(path); base != "camera-20260909-113000.png" {
		t.Errorf("파일 이름이 %q", base)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("PNG 로 읽히지 않는다: %v", err)
	}
	if b := img.Bounds(); b.Dx() != w || b.Dy() != h {
		t.Fatalf("크기가 %dx%d — 원한 것 %dx%d", b.Dx(), b.Dy(), w, h)
	}

	// 화면에서 본 그대로여야 한다. 원본의 x=0 은 사진의 오른쪽 끝에 온다.
	r, _, _, a := img.At(0, 0).RGBA()
	if got := r >> 8; got != uint32(w-1) {
		t.Errorf("왼쪽 끝 픽셀의 R 이 %d — 반전됐다면 %d", got, w-1)
	}
	if a>>8 != 0xff {
		t.Errorf("불투명해야 한다, 얻은 알파 %d", a>>8)
	}
	if r, _, _, _ := img.At(w-1, 0).RGBA(); r>>8 != 0 {
		t.Errorf("오른쪽 끝 픽셀의 R 이 %d — 원한 것 0", r>>8)
	}
}

// 같은 초에 두 번 찍어도 먼저 찍은 것을 덮지 않는다.
func TestSavePhotoDoesNotOverwrite(t *testing.T) {
	downloads(t)
	at := time.Date(2026, 9, 9, 11, 30, 0, 0, time.UTC)

	first, err := savePhoto(stripes(8, 8), 8, 8, at)
	if err != nil {
		t.Fatal(err)
	}
	second, err := savePhoto(stripes(8, 8), 8, 8, at)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("같은 파일에 두 번 썼다: %s", first)
	}
	for _, p := range []string{first, second} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s 가 없다: %v", p, err)
		}
	}
}

// 프레임이 온전하지 않으면 파일을 만들지 않는다. 반쪽짜리 PNG 가
// 다운로드 폴더에 남는 것이 제일 나쁘다.
func TestSavePhotoRejectsBrokenFrame(t *testing.T) {
	dir := downloads(t)
	if _, err := savePhoto(make([]byte, 10), 64, 48, time.Now()); err == nil {
		t.Fatal("모자란 프레임을 받아들였다")
	}
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		t.Errorf("실패했는데 파일이 %d개 남았다", len(entries))
	}
}

// 셔터 — /shot 부터 파일까지 전 구간.
//
// 헬퍼는 stdin 으로 "S" 를 받으면 원본 한 장을 얹어 보낸다. 그 왕복이
// 실제로 도는지는 여기서만 확인할 수 있다.
func TestShotSavesPhoto(t *testing.T) {
	dir := downloads(t)
	script := filepath.Join(t.TempDir(), "camerad")
	body := "#!/bin/sh\n" +
		"printf 'CAM2'\n" +
		"while read line; do\n" +
		"  if [ \"$line\" = S ]; then\n" +
		"    printf 'S\\000\\100\\000\\060'\n" + // 64x48
		"    head -c 9216 /dev/urandom\n" +
		"  fi\n" +
		"done\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CAMERAD", script)

	msgs := make(chan tea.Msg, 8)
	m := New()
	m.Init(func(msg tea.Msg) { msgs <- msg })
	next, _ := m.Update(app.FocusMsg{})
	m = next.(Model)
	defer m.cam.stopHelper()

	if _, cmd := m.Update(shotMsg{}); cmd != nil {
		if say, ok := cmd().(app.SayMsg); ok && say.Err {
			t.Fatalf("셔터가 실패했다: %s", say.Text)
		}
	}

	var still tea.Msg
	select {
	case still = <-msgs:
	case <-time.After(5 * time.Second):
		t.Fatal("사진이 오지 않았다")
	}
	if _, ok := still.(stillMsg); !ok {
		t.Fatalf("사진이 올 자리에 %T 가 왔다", still)
	}

	next, cmd := m.Update(still)
	if cmd == nil {
		t.Fatal("사진을 받고도 저장하지 않았다")
	}
	say, ok := cmd().(app.SayMsg)
	if !ok {
		t.Fatalf("저장 결과가 SayMsg 가 아니다: %T", cmd())
	}
	if say.Err {
		t.Fatalf("저장 실패: %s", say.Text)
	}
	if !strings.Contains(say.Text, "~/Downloads/camera-") || !strings.Contains(say.Text, "64×48") {
		t.Errorf("로그가 어디에 무엇을 저장했는지 말하지 않는다: %q", say.Text)
	}

	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("~/Downloads 에 파일이 %d개 (%v)", len(entries), err)
	}
	f, err := os.Open(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := png.Decode(f); err != nil {
		t.Errorf("저장된 것이 PNG 가 아니다: %v", err)
	}
	_ = next
}

// 카메라가 꺼져 있으면 셔터는 조용히 실패하지 않고 이유를 말한다.
func TestShotWhileOffSaysWhy(t *testing.T) {
	downloads(t)
	t.Setenv("CAMERAD", "/bin/echo")
	m := New()
	_, cmd := m.Update(shotMsg{})
	if cmd == nil {
		t.Fatal("꺼진 카메라로 찍었는데 아무 말이 없다")
	}
	say, ok := cmd().(app.SayMsg)
	if !ok || !say.Err {
		t.Fatalf("실패를 알리지 않는다: %#v", cmd())
	}
}

// 문장으로도 찍을 수 있어야 한다. Description 이 촬영을 광고하므로
// 라우터가 이 앱을 지목하는 유일한 이유가 그것이다.
func TestAskTakesAShot(t *testing.T) {
	m := New()
	cmd := m.Ask("사진 한 장 찍어줘")
	if cmd == nil {
		t.Fatal("문장을 받고 아무것도 하지 않았다")
	}
	if _, ok := cmd().(shotMsg); !ok {
		t.Fatalf("셔터가 아니라 %T 가 나왔다", cmd())
	}
}
