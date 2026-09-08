// Package music 는 Music.app 을 AppleScript 로 조종한다.
//
// 로그인은 없다. Music.app 이 이미 사용자 Apple ID 로 로그인되어 있으므로
// 우리는 그 앱에 말을 걸 뿐이다. 대신 **macOS 자동화 권한**이 첫 실행에
// 한 번 필요하고, 그것이 이 서비스의 유일한 관문이다.
package music

import (
	"errors"
	"os/exec"
	"strconv"
	"strings"
)

var (
	// ErrNotRunning — Music.app 이 실행돼 있지 않다 (E2).
	ErrNotRunning = errors.New("music app is not running")
	// ErrPermissionDenied — 자동화 권한이 거부됐다 (E1).
	ErrPermissionDenied = errors.New("automation permission denied")
)

// PlayerState 는 Music.app 을 폴링해 얻는 값이다. DB 에 저장하지 않는다.
type PlayerState struct {
	Playing      bool
	Stopped      bool
	PersistentID string
	Title        string
	Artist       string
	PositionMs   int
	DurationMs   int
}

// Running 은 Music.app 이 떠 있는지 본다.
//
// AppleScript 로 확인하면 앱이 없을 때 앱을 실행시켜 버리므로,
// 프로세스 목록을 직접 본다. 권한도 필요 없다.
func Running() bool {
	return exec.Command("pgrep", "-x", "Music").Run() == nil
}

// tell 블록 안에서는 `as text` 를 쓴다. `as string` 은 구문 오류가 난다.
const statusScript = `tell application "Music"
	set ps to (player state as text)
	if ps is "stopped" then return "stopped|||||"
	set t to current track
	set pos to 0
	try
		set pos to player position
	end try
	return ps & "|" & (persistent ID of t) & "|" & (name of t) & "|" & (artist of t) & "|" & (pos as text) & "|" & ((duration of t) as text)
end tell`

// Status 는 현재 재생 상태를 읽는다. 1초 주기 폴링에 쓰인다.
func Status() (PlayerState, error) {
	if !Running() {
		return PlayerState{}, ErrNotRunning
	}
	out, err := run(statusScript)
	if err != nil {
		return PlayerState{}, err
	}
	f := strings.Split(out, "|")
	if len(f) < 6 {
		return PlayerState{}, nil
	}
	if f[0] == "stopped" {
		return PlayerState{Stopped: true}, nil
	}
	return PlayerState{
		Playing:      f[0] == "playing",
		PersistentID: f[1],
		Title:        f[2],
		Artist:       f[3],
		PositionMs:   seconds(f[4]),
		DurationMs:   seconds(f[5]),
	}, nil
}

// PlayPersistentID 는 라이브러리에서 곡을 찾아 튼다.
// persistent ID 는 Music.app 재시작에도 유지되는 유일한 식별자다.
func PlayPersistentID(id string) error {
	if !Running() {
		return ErrNotRunning
	}
	_, err := run(`tell application "Music"
	set t to (first track of library playlist 1 whose persistent ID is "` + id + `")
	play t
end tell`)
	return err
}

func PlayPause() error { return simple(`playpause`) }
func Next() error      { return simple(`next track`) }
func Previous() error  { return simple(`previous track`) }

func simple(cmd string) error {
	if !Running() {
		return ErrNotRunning
	}
	_, err := run(`tell application "Music" to ` + cmd)
	return err
}

func run(script string) (string, error) {
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		// -1743 은 TCC 가 자동화를 막았을 때의 코드다.
		if strings.Contains(s, "-1743") || strings.Contains(s, "Not authorized") {
			return "", ErrPermissionDenied
		}
		return "", errors.New(s)
	}
	return s, nil
}

func seconds(s string) int {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return int(f * 1000)
}

// SettingsURL 은 자동화 권한 화면을 여는 주소다. E1 안내에서 쓴다.
const SettingsURL = "x-apple.systempreferences:com.apple.preference.security?Privacy_Automation"

// OpenSettings 는 시스템 설정의 자동화 항목을 연다.
func OpenSettings() error { return exec.Command("open", SettingsURL).Run() }
