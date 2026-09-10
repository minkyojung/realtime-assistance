// Package music 는 Music.app 을 AppleScript 로 조종한다.
//
// 로그인은 없다. Music.app 이 이미 사용자 Apple ID 로 로그인되어 있으므로
// 우리는 그 앱에 말을 걸 뿐이다. 대신 **macOS 자동화 권한**이 첫 실행에
// 한 번 필요하고, 그것이 이 서비스의 유일한 관문이다.
package music

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	// ErrNotRunning — Music.app 이 실행돼 있지 않다 (E2).
	ErrNotRunning = errors.New("music app is not running")
	// ErrPermissionDenied — 자동화 권한이 거부됐다 (E1).
	ErrPermissionDenied = errors.New("automation permission denied")
	// ErrTimeout — osascript 가 제 시간에 안 돌아왔다.
	ErrTimeout = errors.New("osascript timed out")
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

	// 켜져 있는 것들. 같은 왕복에 실어 온다 — 따로 물으면 1초마다 두 번
	// 묻게 되고, 그 값이 서로 다른 순간의 것이라 화면이 어긋난다.
	//
	// 우리가 켠 것을 기억하지 않고 매번 읽는 이유는, 사용자가 Music.app
	// 에서 직접 켤 수도 있기 때문이다. 그때 화면이 옛말을 하면 거짓말이
	// 된다 — "Music.app 이 말해주는 것을 그대로 그린다"(player.go).
	Shuffle   bool
	Repeat    Repeat
	Favorited bool
}

// Running 은 Music.app 이 떠 있는지 본다.
//
// AppleScript 로 확인하면 앱이 없을 때 앱을 실행시켜 버리므로,
// 프로세스 목록을 직접 본다. 권한도 필요 없다.
//
// 한때 부를 때마다 pgrep 을 띄웠다. Music.app 에 말을 거는 스무 자리가
// 전부 이것을 먼저 부르니, 폴링만으로 1초에 프로세스 둘이 생겼다. 이제
// 찾은 PID 를 기억해 두고 **살아 있는지만** 본다(kill 0) — 프로세스 생성
// 없이 마이크로초다. 죽었으면 그 자리에서 알고, 그때만 다시 찾는다.
//
// 시간으로 캐시하지 않는 이유: 사용자가 Music.app 을 끈 직후 1초 폴링이
// 묵은 "켜져 있음"을 믿고 스크립트를 보내면, 그 스크립트가 Music.app 을
// 도로 켠다. 이 함수가 있는 이유 자체를 잃는다.
func Running() bool {
	musicPID.Lock()
	defer musicPID.Unlock()
	if musicPID.pid > 0 && alive(musicPID.pid) {
		return true
	}
	musicPID.pid = findMusic()
	return musicPID.pid > 0
}

var musicPID struct {
	sync.Mutex
	pid int
}

// findMusic 은 Music.app 의 PID 를 찾는다. 없으면 0.
func findMusic() int {
	out, err := exec.Command("pgrep", "-x", "Music").Output()
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0]))
	if err != nil {
		return 0
	}
	return pid
}

// alive 는 그 PID 가 살아 있는지만 본다. 신호 0 은 아무것도 보내지 않는다.
func alive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

// tell 블록 안에서는 `as text` 를 쓴다. `as string` 은 구문 오류가 난다.
const statusScript = `tell application "Music"
	set ps to (player state as text)
	set sh to (shuffle enabled as text)
	set rp to (song repeat as text)
	if ps is "stopped" then return "stopped|||||" & "|" & sh & "|" & rp & "|false"
	set t to current track
	set pos to 0
	try
		set pos to player position
	end try
	set fav to false
	try
		set fav to favorited of t
	end try
	return ps & "|" & (persistent ID of t) & "|" & (name of t) & "|" & (artist of t) & "|" & (pos as text) & "|" & ((duration of t) as text) & "|" & sh & "|" & rp & "|" & (fav as text)
end tell`

// Status 는 현재 재생 상태를 읽는다. 1초 주기 폴링에 쓰인다.
//
// 통로가 차 있으면 **기다리지 않고 ErrBusy 로 돌아온다.** 폴링은 버려도
// 되는 유일한 요청이다 — 1초 뒤에 또 물을 것이고, 쌓아 두면 그 더미가
// Music.app 을 굳힌다(lane.go). 통로를 먼저 보고 프로세스 목록을 본다.
// 반대로 하면 버릴 폴링을 위해 pgrep 을 하나 더 띄운다.
func Status() (PlayerState, error) {
	release, ok := tryHold()
	if !ok {
		return PlayerState{}, ErrBusy
	}
	defer release()
	if !Running() {
		return PlayerState{}, ErrNotRunning
	}
	out, err := exec1(cmdTimeout, statusScript)
	if err != nil {
		return PlayerState{}, err
	}
	return parseStatus(out), nil
}

// parseStatus 는 스크립트가 뱉은 한 줄을 읽는다.
//
// 읽기와 나누어 둔 이유는 이것만 테스트할 수 있게 하기 위해서다.
// 실제로 돌리는 테스트는 이 기계의 Music.app 상태에 기댄다.
func parseStatus(out string) PlayerState {
	f := strings.Split(strings.TrimRight(out, "\n"), "|")
	if len(f) < 6 {
		return PlayerState{}
	}
	modes := parseModes(f)
	if f[0] == "stopped" {
		modes.Stopped = true
		return modes
	}
	modes.Playing = f[0] == "playing"
	modes.PersistentID = f[1]
	modes.Title = f[2]
	modes.Artist = f[3]
	modes.PositionMs = seconds(f[4])
	modes.DurationMs = seconds(f[5])
	return modes
}

// PlayPersistentID 는 곡을 찾아 튼다.
//
// **이어 재생되지 않는다.** Music.app 에 곡 객체 하나를 주면 그것만 틀고
// 멈춘다 — 뒤에 나올 것이 없기 때문이다. 이어 들으려면 담을 것을 줘야 한다
// (ReplaceQueue + PlayQueueAt).
//
// 그래서 재생 경로에서는 쓰지 않는다. 이름만 보면 "곡을 튼다"라서 손이 가는
// 자리인데, 그 길로 가면 한 곡 뒤에 정적이 생긴다.
// persistent ID 는 Music.app 재시작에도 유지되는 유일한 식별자다.
//
// 라이브러리를 먼저 보고, 없으면 플레이리스트를 뒤진다.
// 플레이리스트에만 담고 라이브러리에는 추가하지 않은 곡이 실제로 있다.
func PlayPersistentID(id string) error {
	if !Running() {
		return ErrNotRunning
	}
	_, err := run(`tell application "Music"
	try
		play (first track of library playlist 1 whose persistent ID is "` + id + `")
		return "ok"
	end try
	repeat with p in user playlists
		try
			play (first track of p whose persistent ID is "` + id + `")
			return "ok"
		end try
	end repeat
	error "track not found"
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

// osascript 는 붙잡히면 돌아오지 않는다. 권한 대화상자가 떠 있는 동안이 그렇고,
// 1초 폴링이 그대로 프로세스 더미가 된다.
const cmdTimeout = 5 * time.Second

// 큐를 트는 일은 n번째까지 건너뛰고 재생 위치를 되짚느라 몇 초를 쓴다.
// 그 몇 초를 폴링의 상한과 같이 둘 수는 없다.
const queueTimeout = 45 * time.Second

func run(script string) (string, error) { return runFor(cmdTimeout, script) }

// runFor 는 통로를 잡고 스크립트를 돌린다. 명령은 기다린다 — 사람이 시킨
// 일은 버릴 수 없다(lane.go). 기다림도 timeout 안에 들어간다.
func runFor(timeout time.Duration, script string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	release, err := hold(ctx)
	if err != nil {
		return "", ErrTimeout
	}
	defer release()
	return exec1(timeout, script)
}

// exec1 은 통로를 **이미 잡은 채로** 스크립트 하나를 돌린다.
// 통로를 잡는 것은 부르는 쪽의 일이다 — 잡는 법이 둘(기다림·버림)이라서다.
func exec1(timeout time.Duration, script string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "osascript", "-e", script).CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		if ctx.Err() != nil {
			return "", ErrTimeout
		}
		return "", classify(s)
	}
	return s, nil
}

// classify 는 osascript 가 뱉은 말을 우리가 아는 오류로 바꾼다.
func classify(out string) error {
	s := strings.TrimSpace(out)
	// -1743 은 TCC 가 자동화를 막았을 때의 코드다.
	if strings.Contains(s, "-1743") || strings.Contains(s, "Not authorized") {
		return ErrPermissionDenied
	}
	if s == "" {
		return errors.New("osascript failed without a message")
	}
	return errors.New(s)
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

// CreatePlaylist 는 큐를 Apple Music 플레이리스트로 저장한다 (F9).
//
// 라이브러리에 없는 곡도 담을 수 있어야 하므로 플레이리스트까지 뒤진다.
func CreatePlaylist(name string, persistentIDs []string) error {
	if !Running() {
		return ErrNotRunning
	}
	if len(persistentIDs) == 0 {
		return errors.New("no tracks to add")
	}

	var b strings.Builder
	b.WriteString("tell application \"Music\"\n")
	fmt.Fprintf(&b, "\tset pl to (make new user playlist with properties {name:%q})\n", name)
	b.WriteString("\trepeat with pid in {")
	for i, id := range persistentIDs {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%q", id)
	}
	b.WriteString("}\n")
	b.WriteString(`		set found to false
		try
			duplicate (first track of library playlist 1 whose persistent ID is pid) to pl
			set found to true
		end try
		if not found then
			repeat with p in user playlists
				try
					duplicate (first track of p whose persistent ID is pid) to pl
					exit repeat
				end try
			end repeat
		end if
	end repeat
	return name of pl
end tell`)
	_, err := run(b.String())
	return err
}

// runArgs 는 값을 스크립트에 이어 붙이지 않고 argv 로 넘긴다.
//
// 제목과 아티스트는 사용자 데이터고 따옴표가 들어간다. 이어 붙이면
// 따옴표 하나로 스크립트가 깨진다. Go 의 %q 도 AppleScript 인용과
// 규칙이 달라 안전하지 않다.
func runArgs(script string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	argv := []string{"-e", "on run argv", "-e", script, "-e", "end run"}
	argv = append(argv, args...)
	out, err := exec.CommandContext(ctx, "osascript", argv...).CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		if ctx.Err() != nil {
			return "", ErrTimeout
		}
		return "", classify(s)
	}
	return s, nil
}

// LibraryIDs 는 라이브러리에 있는 모든 곡의 persistent ID 를 돌려준다.
//
// 이름을 안 받고 이름을 안 준다. 이 함수가 답하는 질문은 "무엇이 있나"가
// 아니라 **"무엇이 늘었나"**이고, 그 답에 이름은 필요 없다 — 오히려 해롭다.
//
// 이벤트 한 번이다. 곡마다 물으면 곡 수만큼 늘어난다(dump.js 머리말).
// LibraryCount 는 라이브러리 곡 수만 묻는다.
//
// LibraryIDs 가 전곡을 열거해 수천 글자를 실어 오는 것과 달리 숫자 하나다.
// 담은 곡이 나타나기를 기다리는 동안 1.5초마다 묻는 것이 이것이어야 한다 —
// Music.app 이 가장 바쁜 순간(방금 담은 곡을 받아오는 중)에 가장 무거운
// 질문을 반복하고 있었다. 수가 늘었을 때만 한 번 열거하면 된다.
func LibraryCount() (int, error) {
	if !Running() {
		return 0, ErrNotRunning
	}
	out, err := run(`tell application "Music" to return count of tracks of library playlist 1`)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(out))
}

func LibraryIDs() (map[string]bool, error) {
	if !Running() {
		return nil, ErrNotRunning
	}
	// 구분자는 tell 블록 **밖에서** 세운다. 안에서 세우면 Music.app 의
	// 속성으로 읽혀 -1731 (Unknown object type) 로 죽는다.
	out, err := run(`set AppleScript's text item delimiters to ","
tell application "Music"
	set ids to persistent ID of every track of library playlist 1
end tell
return ids as text`)
	if err != nil {
		return nil, err
	}
	ids := make(map[string]bool, 256)
	for _, id := range strings.Split(out, ",") {
		if id = strings.TrimSpace(id); id != "" {
			ids[id] = true
		}
	}
	return ids, nil
}

// PlayByTitleArtist 는 방금 담긴 곡을 이름으로 찾아 튼다.
//
// **더 이상 쓰지 않는다.** 담긴 곡을 찾는 일은 LibraryIDs 의 차이가 한다.
// 왜 이름이 안 되는지는 실측이 말한다 — 카탈로그(kr)가 "로제"라고 준 곡을
// Music.app 은 "ROSÉ" 로 적었고, 제목도 "Messy (From F1(R) The Movie)" 와
// "Messy" 로 갈렸다. 그때 이 경로는 담기까지 해놓고 "안 나타났다"고 말했다.
//
// 남겨 두는 이유는 이 주석 때문이다. 지우면 다음 사람이 같은 길을 다시 판다.
//
// Music.app 은 Apple Music 카탈로그 id 를 AppleScript 로 내주지 않는다.
// persistent ID 는 곡이 라이브러리에 나타난 뒤에야 생기므로, 담자마자
// 틀려면 이름으로 찾는 수밖에 없다. **이 경로가 제일 무른 곳이다.**
//
// 튼 뒤의 진짜 persistent ID 는 다음 폴링이 알려준다. 우리가 찾을 필요가 없다.
func PlayByTitleArtist(title, artist string) error {
	if !Running() {
		return ErrNotRunning
	}
	// 정확히 같은 이름을 먼저 보고, 없으면 부분 일치로 한 번 더 본다.
	// 카탈로그와 Music.app 이 같은 곡을 다르게 적는 일이 흔하다
	// (feat. 표기, explicit/clean 판본).
	_, err := runArgs(`tell application "Music"
	set t to item 1 of argv
	set a to item 2 of argv
	try
		play (first track of library playlist 1 whose name is t and artist is a)
		return "ok"
	end try
	try
		play (first track of library playlist 1 whose name contains t and artist is a)
		return "ok"
	end try
	error "track not found"
end tell`, title, artist)
	return err
}
