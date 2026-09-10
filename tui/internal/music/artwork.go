package music

import (
	"context"
	_ "embed"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

// 앨범 커버를 Music.app 에서 꺼낸다.
//
// 카탈로그 API 로도 받을 수 있지만(applemusic/catalog.go 가 artwork.url 을
// 이미 읽는다) 그쪽은 카탈로그에 있는 곡만 된다. AppleScript 는 내려받은
// 곡이든 직접 넣은 파일이든 **지금 스피커에서 나오는 그 곡**의 커버를 준다.
//
// 파일을 거치는 이유는 osascript 가 바이너리를 stdout 으로 못 주기 때문이다.
// 표준 출력은 텍스트라 raw data 를 흘리면 깨진다.

//go:embed artwork.applescript
var artworkScript string

// ErrNoArtwork — 곡은 있는데 커버가 없다. 실패가 아니라 상태다.
var ErrNoArtwork = errors.New("no artwork")

// 커버는 곡이 바뀔 때만 부른다. 폴링 주기(1초)와 무관하므로 넉넉히 준다.
const artworkTimeout = 10 * time.Second

// Artwork 는 지금 재생 중인 곡의 커버를 원본 바이트로 돌려준다.
// JPEG 이거나 PNG 다 — 어느 쪽인지는 부르는 쪽이 디코더에게 맡긴다.
func Artwork(ctx context.Context) ([]byte, error) {
	if !Running() {
		return nil, ErrNotRunning
	}
	f, err := os.CreateTemp("", "amcli-art-*")
	if err != nil {
		return nil, err
	}
	path := f.Name()
	f.Close()
	defer os.Remove(path)

	ctx, cancel := context.WithTimeout(ctx, artworkTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "osascript", "-", path)
	cmd.Stdin = strings.NewReader(artworkScript)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// 스크립트가 직접 던지는 두 가지는 실패가 아니라 상태다.
		if s := stderr.String(); strings.Contains(s, "no artwork") || strings.Contains(s, "stopped") {
			return nil, ErrNoArtwork
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, ErrNoArtwork
	}
	return b, nil
}
