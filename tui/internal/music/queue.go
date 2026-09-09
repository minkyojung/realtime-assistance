package music

import (
	"errors"
	"fmt"
	"strings"
)

// 큐는 Music.app 안에 **실제로** 있어야 한다.
//
// 예전에는 고른 8곡 중 첫 곡만 play 시켰다. 나머지 일곱은 Music.app 이
// 존재조차 몰랐고, 첫 곡이 끝나면 그 곡이 속한 앨범의 다음 트랙이 흘렀다.
// 화면의 큐는 재생 대기열이 아니라 "이 곡들을 골랐습니다"라는 영수증이었다.
//
// 그래서 플레이리스트 하나를 정해 두고 큐가 바뀔 때마다 갈아끼운다.
// Music.app 이 큐를 알게 되면 따라오는 것이 많다 — 다음/이전 곡이 우리 큐를
// 따르고, 에어팟과 잠금화면 조작도 맞고, 터미널을 꺼도 재생이 이어진다.
//
// AppleScript 로 Up Next 를 직접 건드릴 수는 없다. Music.app 이 그 표면을
// 열어주지 않는다. 플레이리스트가 유일한 길이다.

// QueuePlaylistName 은 사용자의 사이드바에 보일 이름이다.
// 제품 이름 그대로 쓴다 — 무엇이 만들었는지 한눈에 읽혀야 지울지 말지 정한다.
const QueuePlaylistName = "Apple Music CLI"

var (
	errNoTracks   = errors.New("no tracks to queue")
	errNoPlaylist = errors.New("queue playlist is missing")
)

// ReplaceQueue 는 큐 플레이리스트를 새 목록으로 갈아끼우고 persistent ID 를 돌려준다.
//
// prevPID 는 **지난번에 우리가 만든** 플레이리스트의 persistent ID 다.
// 이름으로 찾지 않는 이유는, 사용자가 우연히 같은 이름의 플레이리스트를
// 갖고 있을 때 그것을 지워 버리기 때문이다. 우리가 만든 것만 지운다.
// prevPID 가 비었거나 그런 플레이리스트가 없으면 아무것도 지우지 않는다.
//
// 지우는 것은 **플레이리스트이지 곡이 아니다.** 곡을 지우는 스크립트는 이
// 파일에 없다 — 라이브러리에서 음악이 사라지는 사고는 되돌릴 수 없다.
func ReplaceQueue(prevPID string, persistentIDs []string) (string, error) {
	if !Running() {
		return "", ErrNotRunning
	}
	if len(persistentIDs) == 0 {
		return "", errNoTracks
	}
	out, err := run(replaceQueueScript(prevPID, persistentIDs))
	if err != nil {
		return "", err
	}
	pid := strings.TrimSpace(out)
	if pid == "" {
		return "", errNoPlaylist
	}
	return pid, nil
}

// replaceQueueScript 는 스크립트를 조립한다.
//
// 실행과 나누어 둔 이유는 이것만 테스트할 수 있게 하기 위해서다.
// 실제로 돌리는 테스트는 남의 라이브러리를 고치므로 쓸 수 없다.
func replaceQueueScript(prevPID string, persistentIDs []string) string {
	var b strings.Builder
	b.WriteString("tell application \"Music\"\n")

	// 우리가 만든 것만 지운다. 없으면 조용히 넘어간다.
	if prevPID != "" {
		fmt.Fprintf(&b, "\ttry\n\t\tdelete (first user playlist whose persistent ID is %q)\n\tend try\n", prevPID)
	}

	fmt.Fprintf(&b, "\tset pl to (make new user playlist with properties {name:%q})\n", QueuePlaylistName)

	// 담는 방식은 CreatePlaylist 와 같다 — 라이브러리에 없고 플레이리스트에만
	// 있는 곡이 실제로 있어서, 라이브러리를 먼저 보고 없으면 나머지를 뒤진다.
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
	return persistent ID of pl
end tell`)
	return b.String()
}

// PlayQueueAt 는 큐 플레이리스트를 n번째 곡부터 재생한다 (1부터 센다).
//
// positionSec 이 0보다 크면 그 지점부터 잇는다. 큐를 재편성했는데 듣던 곡이
// 살아남았을 때 쓴다 — 듣던 곡이 처음으로 되감기는 것만큼 짜증나는 것이 없다.
// 갈아끼우는 동안 소리가 한 번 끊기는 것까지는 어쩔 수 없다.
func PlayQueueAt(pid string, n int, positionSec int) error {
	if !Running() {
		return ErrNotRunning
	}
	if pid == "" || n < 1 {
		return errNoPlaylist
	}
	var b strings.Builder
	b.WriteString("tell application \"Music\"\n")
	fmt.Fprintf(&b, "\tset pl to (first user playlist whose persistent ID is %q)\n", pid)
	fmt.Fprintf(&b, "\tplay (track %d of pl)\n", n)
	if positionSec > 0 {
		fmt.Fprintf(&b, "\tset player position to %d\n", positionSec)
	}
	b.WriteString("end tell")
	_, err := run(b.String())
	return err
}
