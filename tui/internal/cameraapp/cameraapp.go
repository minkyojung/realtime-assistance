// Package cameraapp 은 호스트가 담는 세 번째 앱이다. 맥 카메라를 ASCII 로 그린다.
//
// 앞의 둘과 다른 점이 둘 있다.
//
//  1. **바깥 프로세스를 쓴다.** macOS 카메라는 AVFoundation 이고 그것은 Go 가
//     아니다. cgo 로 끌어오는 대신 헬퍼(camerad)를 띄워 파이프로 받는다.
//     헬퍼는 화면을 모른다 — ASCII 로 바꾸는 일은 전부 이쪽에 있어서
//     카메라 권한 없이 테스트된다.
//
//  2. **안 보는 동안에는 꺼진다.** 다른 앱은 배경에서 계속 돌아야 하지만
//     (재생·메시지 수신), 카메라가 그러면 초록불이 세션 내내 켜져 있다.
//     app.FocusMsg / app.BlurMsg 가 그 신호다.
//
// 하는 일은 보여주는 것과 한 장 남기는 것뿐이다. 녹화하지도, 알아보지도 않는다.
//
//	cameraapp.go  계약 — 호스트가 보는 전부
//	camerad.go    헬퍼 프로세스와 파이프
//	frame.go      RGB 프레임 → ASCII. 카메라가 없는 순수 함수
//	shot.go       촬영 — 원본 한 장을 ~/Downloads 에 PNG 로
//	camerad/      헬퍼의 Swift 소스
package cameraapp

import (
	"fmt"
	"os"
	"strings"
	"time"

	"amcli/tui/internal/app"
	"amcli/tui/internal/style"
	tea "charm.land/bubbletea/v2"
)

// Model 은 이 앱의 상태 전부다.
//
// 크기를 담지 않는 것에 주의 — View 가 인자로 받는다. 그림은 언제나
// 그때의 인자를 따르고, 여기 있는 것은 마지막으로 받은 프레임뿐이다.
type Model struct {
	cam *camera // 프로세스 핸들. 값 복사되면 안 되는 유일한 것

	px     []byte // 마지막 프레임 (RGB24)
	fw, fh int    // 그 프레임의 크기
	gen    int    // 그 프레임을 보낸 헬퍼 세대

	on   bool  // 화면 앞에 있어서 켜져 있는가
	err  error // 헬퍼가 죽었거나 없으면 그 이유
	mono bool
}

var _ app.App = Model{}

func New() Model { return Model{cam: &camera{}} }

func (m Model) Name() string { return "camera" }

// Description 은 라우터가 보는 유일한 근거다.
//
// 못 하는 것까지 적는 이유는, 이 앱이 할 수 없는 요청("사진 찍어줘")이
// 여기로 오면 갈 곳 없는 문장이 되기 때문이다.
func (m Model) Description() string {
	return "Shows the Mac's built-in camera as live ASCII art in the terminal, " +
		"and saves a still photo to the Downloads folder on request. " +
		"It cannot record video or recognize anything it sees."
}

func (m Model) Tagline() string { return "보여주고, 한 장 남긴다" }

// Ready 는 관문이다. 음악은 자동화 권한, 채팅은 로그인, 여기는 헬퍼다.
//
// 카메라 권한은 여기서 볼 수 없다. TCC 는 이 프로세스가 아니라 이것을
// 실행한 터미널 앱에 붙고, 거부는 헬퍼를 띄워 봐야 알 수 있다.
func (m Model) Ready() error {
	if m.err != nil {
		return m.err
	}
	if _, err := helperPath(); err != nil {
		return err
	}
	return nil
}

// Init 은 아무것도 켜지 않는다.
//
// 다른 앱은 여기서 시작하지만 카메라는 보일 때 켜진다. send 를 헬퍼가
// 쓸 수 있도록 넘겨만 둔다 — 값 수신자라 Model 에 담아도 돌아가지 않으므로
// 복사되지 않는 곳(cam)에 둔다.
func (m Model) Init(send func(tea.Msg)) tea.Cmd {
	m.cam.mu.Lock()
	m.cam.send = send
	m.cam.mu.Unlock()
	return nil
}

func (m Model) Update(msg tea.Msg) (app.App, tea.Cmd) {
	switch msg := msg.(type) {
	case app.FocusMsg:
		m.on = true
		m.err = m.cam.start()
		if m.err != nil {
			return m, app.SayErr(m.Name(), m.err)
		}
		return m, nil

	case app.BlurMsg:
		// 화면을 벗어나면 놓는다. 마지막 프레임도 버린다 — 돌아왔을 때
		// 몇 분 전 얼굴이 잠깐 비치면 켜져 있었던 것처럼 보인다.
		m.cam.stopHelper()
		m.on, m.px = false, nil
		return m, nil

	case frameMsg:
		// 이전 세대의 프레임이 늦게 도착할 수 있다. 껐다 켠 뒤라면 버린다.
		if !m.on || msg.gen != m.cam.generation() {
			return m, nil
		}
		m.px, m.fw, m.fh, m.gen = msg.px, msg.w, msg.h, msg.gen
		m.err = nil
		return m, nil

	case camErrMsg:
		if msg.gen != m.cam.generation() {
			return m, nil
		}
		m.cam.stopHelper()
		m.px, m.err = nil, msg.err
		return m, app.SayErr(m.Name(), msg.err)

	case monoMsg:
		m.mono = !m.mono
		return m, nil

	case shotMsg:
		// 셔터를 누르기만 한다. 사진은 헬퍼가 다음 프레임에 얹어 보낸다.
		if err := m.cam.requestStill(); err != nil {
			return m, app.SayErr(m.Name(), err)
		}
		return m, nil

	case stillMsg:
		// 파일로 쓰는 일은 Cmd 로 미룬다. 720p PNG 인코딩은 이벤트 루프를
		// 세워 둘 만큼 걸리고, 그동안 화면이 멈춰 있으면 안 된다.
		return m, cmdSave(m.Name(), msg)

	case app.AskMsg:
		return m, m.Ask(msg.Prompt)
	}
	return m, nil
}

// View 는 본문을 그린다. 언제나 정확히 h 줄이다.
func (m Model) View(w, h int) string {
	if err := m.Ready(); err != nil {
		return m.viewGate(w, h, err)
	}
	if m.px == nil {
		return center(w, h, style.Faint.Render("카메라를 켜는 중…"))
	}
	return ascii(m.px, m.fw, m.fh, w, h, m.mono)
}

// 관문 화면. 다음에 무엇을 해야 하는지가 전부다.
func (m Model) viewGate(w, h int, err error) string {
	lines := []string{
		style.ErrorBadge.Render("CAMERA") + " " +
			style.Body.Render(style.Truncate(err.Error(), style.Max(w-10, 10))),
		"",
	}
	for _, s := range gateSteps {
		lines = append(lines, style.Faint.Render(style.Truncate(s, w)))
	}
	return fit(lines, h)
}

var gateSteps = []string{
	"tui 디렉터리에서 `make` 를 실행하면 camerad 가 bin/ 에 함께 빌드됩니다.",
	"",
	"카메라 권한은 이 프로그램이 아니라 이것을 실행한 터미널 앱에 붙습니다.",
	"거부했다면 시스템 설정 > 개인정보 보호 및 보안 > 카메라 에서 켜 주세요.",
}

func center(w, h int, s string) string {
	lines := make([]string, h)
	if h > 0 {
		pad := style.Max((w-len([]rune(style.Truncate(s, w))))/2, 0)
		lines[h/2] = strings.Repeat(" ", pad) + s
	}
	return strings.Join(lines, "\n")
}

// fit 은 줄들을 정확히 h 줄로 맞춘다. 넘치면 아래를 자른다.
func fit(lines []string, h int) string {
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines[:style.Max(h, 0)], "\n")
}

// Status 는 배경에 있을 때도 불리지만, 호스트는 배경 앱의 이름만 모은다.
// 그래도 괜찮다 — 카메라는 보일 때만 켜지므로, 켜져 있는 동안에는 언제나
// 이 줄이 상태줄의 왼쪽을 차지한다.
func (m Model) Status() string {
	switch {
	case m.err != nil:
		return style.Warn.Render("camera · " + style.Truncate(m.err.Error(), 48))
	case !m.on:
		return style.Faint.Render("camera · 꺼짐 · 볼 때만 켜집니다")
	case m.px == nil:
		return style.Faint.Render("camera · 켜는 중…")
	}
	mode := "color"
	if m.mono {
		mode = "mono"
	}
	return style.Dim.Render("camera · ● 켜짐 · " + mode)
}

// 안 읽은 것이라는 개념이 없다.
func (m Model) Badge() int { return 0 }

// Ask 는 셔터를 누른다.
//
// 문장을 해석하지 않는다. 해석할 것이 없기 때문이다 — 이 앱이 요청을 받아
// **할 수 있는** 일은 촬영 하나뿐이다. 보여주는 것은 화면을 여는 일이라
// 라우팅으로는 닿지 않는다(라우터는 문장을 앱에 넘길 뿐 화면을 바꾸지 않는다).
//
// 그래서 단어 매칭도, 모델 호출도 두지 않는다. 카메라가 꺼져 있으면
// 그 사실과 여는 방법이 로그로 돌아간다.
func (m Model) Ask(string) tea.Cmd {
	return func() tea.Msg { return shotMsg{} }
}

// 거를 목록이 없다.
func (m Model) Filter(string) app.App { return m }

func (m Model) Commands() []app.Command {
	return []app.Command{
		{
			Name: "/shot",
			Help: "save a photo to ~/Downloads",
			Run:  func(string) tea.Cmd { return func() tea.Msg { return shotMsg{} } },
		},
		{
			Name: "/mono",
			Help: "toggle colour",
			Run:  func(string) tea.Cmd { return func() tea.Msg { return monoMsg{} } },
		},
	}
}

type (
	monoMsg struct{}
	shotMsg struct{}
)

// cmdSave 는 사진을 쓰고 그 결과를 로그에 남긴다.
//
// 경로를 로그에 적는 이유는, 화면이 ASCII 라 무엇이 저장됐는지 눈으로
// 확인할 방법이 여기밖에 없기 때문이다.
func cmdSave(name string, still stillMsg) tea.Cmd {
	return func() tea.Msg {
		path, err := savePhoto(still.px, still.w, still.h, time.Now())
		if err != nil {
			return app.SayMsg{App: name, Text: "사진을 저장하지 못했습니다: " + err.Error(), Err: true}
		}
		return app.SayMsg{
			App:  name,
			Text: fmt.Sprintf("%s 에 저장했습니다 · %d×%d", prettyPath(path), still.w, still.h),
		}
	}
}

// 홈 디렉터리는 ~ 로 줄인다. 로그는 한 줄이라 경로가 길면 다른 것을 밀어낸다.
func prettyPath(path string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home+"/") {
		return "~" + path[len(home):]
	}
	return path
}
