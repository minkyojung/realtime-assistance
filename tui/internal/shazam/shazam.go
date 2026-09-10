// Package shazam 은 마이크로 들리는 곡을 알아맞힌다.
//
// 직접 하지 않는다. ShazamKit 은 Swift/ObjC 프레임워크이고 Go 에서 부를 길이
// 없으므로, 옆에 세워 둔 .app 헬퍼를 한 번 실행하고 그가 뱉은 JSON 한 줄을
// 읽는다 — helper/README.md.
//
// music 패키지가 Music.app 에, applemusic 이 Apple 서버에 말을 건다면
// 여기는 **방 안의 소리**에 말을 건다. 셋 다 우리 밖에 있는 것들이라
// 실패가 정상 상태에 속한다. 못 알아듣는 것(ErrNoMatch)이 특히 그렇다.
package shazam

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

var (
	// ErrNoMatch — 조용했거나 모르는 곡이다. 고장이 아니다.
	ErrNoMatch = errors.New("nothing recognized")
	// ErrMicDenied — 터미널에 마이크 권한이 없다.
	ErrMicDenied = errors.New("microphone access denied")
	// ErrCatalogRefused — 카탈로그 조회가 거부됐다.
	// 망이 끊겼거나 헬퍼에 ShazamKit 엔타이틀먼트가 없다. 둘을 못 가른다.
	ErrCatalogRefused = errors.New("shazam catalog refused")
	// ErrNoHelper — 헬퍼가 없다. 아직 안 만들었다는 뜻이다.
	ErrNoHelper = errors.New("shazamd helper not built")
	// ErrTimeout — 헬퍼가 제 시간에 안 돌아왔다.
	ErrTimeout = errors.New("shazamd timed out")
)

// Result 는 SHMediaItem 에서 우리가 쓰는 것만 옮긴 것이다.
// 이름은 스펙의 recognition 필드를 따른다 — db/schema.dbml.
type Result struct {
	OK           bool   `json:"ok"`
	Error        string `json:"error"`
	Message      string `json:"message"`
	ShazamID     string `json:"shazamId"`
	AppleMusicID string `json:"appleMusicId"`
	ISRC         string `json:"isrc"`
	Title        string `json:"title"`
	Artist       string `json:"artist"`
	Genre        string `json:"genre"`
	ArtworkURL   string `json:"artworkUrl"`
	Year         int    `json:"year"`
}

// 듣는 시간. 스펙의 인식 목표가 5초다(docs/03 8절). 헬퍼가 그보다 조금 더
// 듣고, 우리는 그보다 조금 더 기다린다 — 경계에서 서로를 죽이지 않게.
const (
	listenSeconds = 8
	killAfter     = 15 * time.Second
)

// Listen 은 한 번 듣고 한 곡을 돌려준다.
func Listen(ctx context.Context) (Result, error) {
	bin, err := HelperPath()
	if err != nil {
		return Result{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, killAfter)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "-seconds", strconv.Itoa(listenSeconds))
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return Result{}, ErrTimeout
		}
		// 헬퍼는 실패도 JSON 으로 말한다. 종료 코드가 0 이 아니면
		// 그 약속이 깨진 것이므로 stderr 를 그대로 올린다.
		if msg := trimmed(stderr.String()); msg != "" {
			return Result{}, fmt.Errorf("shazamd: %s", msg)
		}
		return Result{}, fmt.Errorf("shazamd: %w", err)
	}
	return Decode(stdout.Bytes())
}

// Decode 는 헬퍼의 한 줄을 결과나 에러로 가른다.
//
// 실행과 나눠 둔 이유는 이 갈래마다 화면이 다르기 때문이다 —
// 마이크 권한과 "못 알아들었다" 는 사용자가 할 일이 완전히 다르다.
func Decode(line []byte) (Result, error) {
	var r Result
	if err := json.Unmarshal(bytes.TrimSpace(line), &r); err != nil {
		return Result{}, fmt.Errorf("shazamd said something we cannot read: %w", err)
	}
	if r.OK {
		if r.Title == "" {
			return Result{}, ErrNoMatch // 제목이 없으면 보여줄 것이 없다
		}
		return r, nil
	}

	msg := r.Message
	switch r.Error {
	case "no-match":
		return Result{}, ErrNoMatch
	case "microphone-denied":
		return Result{}, fmt.Errorf("%w · %s", ErrMicDenied, msg)
	case "catalog-refused":
		return Result{}, fmt.Errorf("%w · %s", ErrCatalogRefused, msg)
	}
	if msg == "" {
		msg = "shazamd failed"
	}
	return Result{}, errors.New(msg)
}

// HelperPath 는 헬퍼를 찾는다.
//
// 찾는 곳이 여럿인 이유는 실행되는 자리가 여럿이기 때문이다 —
// `cd tui && go run .` 일 때와 빌드된 바이너리를 옮겨 쓸 때가 다르다.
func HelperPath() (string, error) {
	const inApp = "Shazamd.app/Contents/MacOS/shazamd"

	if p := os.Getenv("SHAZAMD"); p != "" {
		if ok(p) {
			return p, nil
		}
		return "", fmt.Errorf("%w: SHAZAMD points at %s", ErrNoHelper, p)
	}

	var dirs []string
	if cwd, err := os.Getwd(); err == nil {
		// 저장소 안에서 돌 때. tui/ 에서 부르는 경우가 흔하다.
		dirs = append(dirs, filepath.Join(cwd, "helper", "build"),
			filepath.Join(cwd, "..", "helper", "build"))
	}
	if exe, err := os.Executable(); err == nil {
		// 빌드된 바이너리 옆. 배포할 때의 모양이다.
		dirs = append(dirs, filepath.Dir(exe))
	}
	for _, d := range dirs {
		if p := filepath.Join(d, inApp); ok(p) {
			return p, nil
		}
	}
	return "", fmt.Errorf("%w · build it with `make -C helper`", ErrNoHelper)
}

func ok(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir() && fi.Mode().Perm()&0o100 != 0
}

func trimmed(s string) string {
	if len(s) > 200 {
		s = s[:200]
	}
	return string(bytes.TrimSpace([]byte(s)))
}
