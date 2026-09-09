package music

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"os/exec"
	"strings"
	"time"
)

//go:embed dump.js
var dumpScript string

// ErrDumpTimeout — 덤프가 제 시간에 안 끝났다.
var ErrDumpTimeout = errors.New("library dump timed out")

// 덤프는 곡 수에 비례한다. 실측 157곡 0.16초였으니 넉넉히 잡되 무한은 아니다.
const dumpTimeout = 60 * time.Second

// DumpLibrary 는 Music.app 라이브러리 전체를 JSON 으로 받아온다.
//
// CombinedOutput 을 쓰지 않는다. osascript 는 경고를 stderr 로 뱉는데
// 그것이 섞이면 JSON 이 통째로 깨진다.
func DumpLibrary(ctx context.Context) ([]byte, error) {
	if !Running() {
		return nil, ErrNotRunning // AppleScript 로 물으면 Music.app 을 켜 버린다
	}
	ctx, cancel := context.WithTimeout(ctx, dumpTimeout)
	defer cancel()

	// 스크립트를 stdin 으로 넘긴다. -e 로 넘기면 인자 길이 제한에 걸린다.
	cmd := exec.CommandContext(ctx, "osascript", "-l", "JavaScript", "-")
	cmd.Stdin = strings.NewReader(dumpScript)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ErrDumpTimeout
		}
		return nil, classify(stderr.String())
	}
	return stdout.Bytes(), nil
}
