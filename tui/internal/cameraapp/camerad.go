package cameraapp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
)

// 헬퍼 프로세스를 잡고 있는 유일한 곳.
//
// Model 은 값 타입이라 복사되며 돌아다닌다. 프로세스 핸들은 복사되면
// 안 되므로 여기 포인터 하나에 모아 두고, Model 은 그것을 가리키기만 한다.

type camera struct {
	mu sync.Mutex

	// 이벤트 루프 밖에서 메시지를 넣는 통로. Init 에서 받는다.
	// Model 은 값 수신자라 거기 담아 두면 호출자에게 돌아가지 않는다.
	send func(tea.Msg)

	cmd  *exec.Cmd
	in   io.WriteCloser // 헬퍼에게 요청을 보내는 통로. 지금은 촬영뿐이다
	stop chan struct{}  // 닫히면 "우리가 껐다" — 그때 나는 오류는 오류가 아니다
	gen  int            // 뒤늦게 도착한 이전 세대의 프레임을 버리는 데 쓴다
}

// frameMsg 는 프리뷰 프레임 하나다. 화면에 그려지고 버려진다.
type frameMsg struct {
	gen  int
	px   []byte
	w, h int
}

// stillMsg 는 촬영된 원본 한 장이다. 프리뷰와 달리 줄이지 않은 픽셀이라
// 파일로 남을 수 있다.
type stillMsg struct {
	px   []byte
	w, h int
}

// camErrMsg 는 헬퍼가 죽었다는 뜻이다. 이유는 대개 stderr 에 있다.
type camErrMsg struct {
	gen int
	err error
}

var errNoHelper = errors.New(
	"camerad 가 없습니다. tui 디렉터리에서 `make` 를 실행하면 함께 빌드됩니다")

// helperPath 는 헬퍼를 찾는다.
//
// 실행 파일 옆을 먼저 본다. Makefile 이 둘을 같은 bin/ 에 넣기 때문이다.
// go run 으로 띄우면 실행 파일이 임시 디렉터리에 있어 못 찾는데,
// 그때는 Ready() 가 무엇을 해야 하는지 말해 준다.
func helperPath() (string, error) {
	if p := strings.TrimSpace(os.Getenv("CAMERAD")); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("CAMERAD=%s 를 찾을 수 없습니다", p)
		}
		return p, nil
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "camerad")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if p, err := exec.LookPath("camerad"); err == nil {
		return p, nil
	}
	return "", errNoHelper
}

// start 는 헬퍼를 띄우고 프레임을 읽는 고루틴을 건다.
//
// 이미 켜져 있으면 아무것도 하지 않는다 — 포커스가 두 번 오더라도 카메라를
// 두 개 열지 않는다.
func (c *camera) start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cmd != nil {
		return nil
	}
	if c.send == nil {
		return errors.New("헬퍼에 메시지를 넣을 통로가 없습니다 — Init 이 불리지 않았습니다")
	}
	send := c.send

	path, err := helperPath()
	if err != nil {
		return err
	}

	cmd := exec.Command(path)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	var errBuf lockedBuffer
	cmd.Stderr = &errBuf
	if err := cmd.Start(); err != nil {
		return err
	}

	c.gen++
	c.cmd = cmd
	c.in = in
	c.stop = make(chan struct{})
	gen, stop := c.gen, c.stop

	go func() {
		err := pump(out, gen, send)
		// 우리가 끈 것이면 조용히 끝난다. 끄는 것과 죽는 것은 다르다.
		select {
		case <-stop:
			cmd.Wait()
			return
		default:
		}
		cmd.Wait()
		if msg := strings.TrimSpace(errBuf.String()); msg != "" {
			err = errors.New(msg)
		} else if err == nil || errors.Is(err, io.EOF) {
			err = errors.New("camerad 가 조용히 끝났습니다")
		}
		send(camErrMsg{gen: gen, err: err})
	}()
	return nil
}

// stopHelper 는 헬퍼를 죽인다. 카메라 불이 꺼지는 유일한 경로다.
func (c *camera) stopHelper() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cmd == nil {
		return
	}
	close(c.stop)
	c.in.Close()
	c.cmd.Process.Kill()
	c.cmd, c.in, c.stop = nil, nil, nil
}

// requestStill 은 셔터를 누른다. 원본 한 장이 곧 stillMsg 로 돌아온다.
func (c *camera) requestStill() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cmd == nil {
		return errors.New("카메라가 꺼져 있습니다 — /camera 로 열면 켜집니다")
	}
	_, err := io.WriteString(c.in, "S\n")
	return err
}

func (c *camera) running() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cmd != nil
}

func (c *camera) generation() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.gen
}

// pump 는 머리를 한 번 읽고, 그 뒤로는 메시지를 끝없이 읽어 밀어 넣는다.
// 형식은 internal/cameraapp/camerad/main.swift 에 적혀 있다.
func pump(r io.Reader, gen int, send func(tea.Msg)) error {
	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return err
	}
	if string(magic[:]) != "CAM2" {
		return fmt.Errorf("알 수 없는 프레임 형식: %q", magic[:])
	}

	var head [5]byte
	for {
		if _, err := io.ReadFull(r, head[:]); err != nil {
			return err
		}
		kind := head[0]
		w := int(binary.BigEndian.Uint16(head[1:3]))
		h := int(binary.BigEndian.Uint16(head[3:5]))
		// 파이프가 어긋나면 여기서 터무니없는 크기가 나온다. 그때 수 기가를
		// 잡으려 들지 않도록 막는다.
		if w <= 0 || h <= 0 || w*h > 1<<23 {
			return fmt.Errorf("프레임 크기가 이상합니다: %dx%d", w, h)
		}

		// 메시지마다 새로 잡는다. 재사용하면 화면이 읽는 중에 덮어쓰게 된다.
		px := make([]byte, w*h*3)
		if _, err := io.ReadFull(r, px); err != nil {
			return err
		}

		switch kind {
		case 'P':
			send(frameMsg{gen: gen, px: px, w: w, h: h})
		case 'S':
			// 세대를 달지 않는다. 사진은 화면과 달리 늦게 도착해도
			// 버릴 이유가 없다 — 사용자가 요청한 그 순간의 픽셀이다.
			send(stillMsg{px: px, w: w, h: h})
		default:
			return fmt.Errorf("알 수 없는 메시지 종류: %q", kind)
		}
	}
}

// 고루틴이 쓰고 이벤트 루프가 읽으므로 잠금이 필요하다.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}
