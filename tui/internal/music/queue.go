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

	writeAppend(&b, persistentIDs)
	b.WriteString("\treturn persistent ID of pl\nend tell")
	return b.String()
}

// writeAppend 는 곡들을 pl 의 **맨 뒤에** 붙이는 블록을 적는다.
//
// duplicate 는 언제나 뒤에 붙는다. 가운데에 끼우는 명령이 없다 — Music.app
// 의 AppleScript 사전에 playlist 안의 곡을 옮기는 길이 아예 없다(move 는
// 플레이리스트를 폴더로 옮기는 것이고, track 에는 자리번호 속성이 없다).
// 순서를 바꾸려면 뒤를 지웠다가 다시 붙이는 수밖에 없는 이유다.
//
// 라이브러리를 먼저 보고 없으면 나머지를 뒤진다 — 라이브러리에 없고
// 플레이리스트에만 있는 곡이 실제로 있다.
func writeAppend(b *strings.Builder, persistentIDs []string) {
	b.WriteString("\trepeat with pid in {")
	for i, id := range persistentIDs {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(b, "%q", id)
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
`)
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
	_, err := runFor(queueTimeout, playQueueAtScript(pid, n, positionSec))
	return err
}

// playQueueAtScript 는 스크립트를 조립한다. 나눠 둔 이유는 replaceQueueScript 와 같다.
//
// **첫 곡부터일 때는 `play pl` 이어야 한다.** `play (track 1 of pl)` 은 그 곡
// 하나만 틀고 멈춘다 — Music.app 이 큐 맥락을 잡지 않는다. 실측으로 확인했다:
//
//	play (track 1 of pl) → 1번 재생 후 정지
//	play pl              → 1번 재생 후 2번으로 이어짐
//
// 큐를 새로 만들 때는 언제나 n=1 이므로, 이 한 줄이 "고른 곡들이 끝까지
// 이어진다"의 전부다.
//
// n > 1 은 아직 이어지지 않는다. `next track` 으로 건너뛰는 방법은 실측에서
// 못 쓸 것으로 판명됐다 — 곡마다 로딩을 기다려야 해서 두 칸을 요청하면 한 칸만
// 먹고, 지연을 늘리면 엉뚱한 곡으로 샜다. **틀린 곡을 트느니 그 곡만 트는 편이
// 낫다.** 되는 방법은 확인해 뒀다(플레이리스트를 회전시켜 그 곡을 1번으로
// 올린다). 다만 그러면 화면의 큐 순서도 같이 돌려야 해서 호출부가 바뀐다.
func playQueueAtScript(pid string, n, positionSec int) string {
	var b strings.Builder
	b.WriteString("tell application \"Music\"\n")
	// 순서가 이 큐의 전부다(intent 의 시스템 프롬프트).
	// 섞기가 켜져 있으면 우리가 정한 순서가 무의미해진다.
	b.WriteString("\tset shuffle enabled to false\n")
	fmt.Fprintf(&b, "\tset pl to (first user playlist whose persistent ID is %q)\n", pid)
	if n == 1 {
		b.WriteString("\tplay pl\n")
	} else {
		fmt.Fprintf(&b, "\tplay (track %d of pl)\n", n)
	}
	// 스트리밍 곡은 버퍼링 전에는 seek 가 먹지 않는다. 한 번 찔러보고 마는
	// 코드는 조용히 실패한다 — 실측으로 25.9초가 0.08초로 떨어졌다.
	// 자리를 잡을 때까지 되짚는다.
	//
	// 읽는 쪽은 get 으로 값을 끌어내고 try 로 감싼다. 곡이 아직 안 물렸을 때
	// player position 은 값이 아니라 참조로 평가되어(class 가 property 로 나온다)
	// 수와 비교하는 순간 -1700 으로 죽는다. 실측으로 봤다.
	if positionSec > 0 {
		fmt.Fprintf(&b, `	repeat 20 times
		try
			set player position to %d
		end try
		delay 0.3
		try
			set p to (get player position)
			if p > %d then exit repeat
		end try
	end repeat
`, positionSec, positionSec-3)
	}
	b.WriteString("end tell")
	return b.String()
}

// RemoveQueueTrack 은 큐에서 곡 하나를 뺀다 (n 은 1부터 센다).
//
// 통째로 다시 쓰지 않는 이유는 **음악이 끊기기 때문**이다. 플레이리스트를
// 지우고 새로 만들면 듣던 곡도 함께 사라진다. 한 줄만 지우면 나머지는
// 그대로 흐른다 — 앞쪽을 빼든 뒤쪽을 빼든 마찬가지다.
//
// advance 는 "지금 나오는 곡을 빼는 중"이라는 뜻이다. 그 곡은 지우기 전에
// 다음 곡으로 넘겨야 한다. 사용자가 기대하는 것도 그것이다 — "이거 별로야"는
// 조용해지라는 뜻이 아니라 다음 걸 틀라는 뜻이다.
//
// 지우는 대상은 언제나 **우리 플레이리스트를 거쳐서** 가리킨다. 라이브러리
// 쪽으로 새면 파일이 사라지고, 그것은 되돌릴 수 없다.
func RemoveQueueTrack(pid string, n int, advance bool) error {
	if !Running() {
		return ErrNotRunning
	}
	if pid == "" || n < 1 {
		return errNoPlaylist
	}
	_, err := run(removeQueueTrackScript(pid, n, advance))
	return err
}

func removeQueueTrackScript(pid string, n int, advance bool) string {
	var b strings.Builder
	b.WriteString("tell application \"Music\"\n")
	fmt.Fprintf(&b, "\tset pl to (first user playlist whose persistent ID is %q)\n", pid)
	if advance {
		b.WriteString("\tnext track\n")
	}
	fmt.Fprintf(&b, "\tdelete track %d of pl\n", n)
	b.WriteString("end tell")
	return b.String()
}

// RewriteQueueTail 은 큐의 from 번째부터 끝까지를 지우고 새 목록으로 다시 쓴다.
// (from 은 1부터 센다)
//
// **이것이 순서를 바꾸고 가운데에 끼우는 유일한 길이다.** AppleScript 에는
// 플레이리스트 안의 곡을 옮기는 명령이 없어서(writeAppend 의 주석), 바꿀
// 자리부터 뒤를 통째로 다시 쌓는 수밖에 없다.
//
// 앞은 건드리지 않는다. **지금 나오는 곡이 앞에 남아 있으면 음악이 안 끊긴다** —
// 플레이리스트 객체도 그대로이고 재생 중인 곡도 그 안에 그대로 있기 때문이다.
// 통째로 갈아끼우는 ReplaceQueue 가 소리를 끊는 것과 갈리는 지점이 여기다.
// 그래서 부르는 쪽은 from 이 지금 나오는 곡보다 뒤인지를 먼저 확인해야 한다.
//
// 지우는 대상은 언제나 **우리 플레이리스트의 곡**이다. RemoveQueueTrack 과
// 같은 규칙이다 — 라이브러리 쪽으로 새면 파일이 사라지고 되돌릴 수 없다.
func RewriteQueueTail(pid string, from int, persistentIDs []string) error {
	if !Running() {
		return ErrNotRunning
	}
	if pid == "" || from < 1 {
		return errNoPlaylist
	}
	_, err := run(rewriteQueueTailScript(pid, from, persistentIDs))
	return err
}

func rewriteQueueTailScript(pid string, from int, persistentIDs []string) string {
	var b strings.Builder
	b.WriteString("tell application \"Music\"\n")
	fmt.Fprintf(&b, "\tset pl to (first user playlist whose persistent ID is %q)\n", pid)
	// 뒤에서부터 지운다. 앞에서 지우면 그 뒤 곡들의 자리번호가 밀려 다음
	// 한 바퀴가 엉뚱한 줄을 가리킨다 — removeTracks 가 뒤에서부터 빼는 것과
	// 같은 이유다.
	fmt.Fprintf(&b, "\trepeat with i from (count of tracks of pl) to %d by -1\n", from)
	b.WriteString("\t\tdelete track i of pl\n\tend repeat\n")
	if len(persistentIDs) > 0 {
		writeAppend(&b, persistentIDs)
	}
	b.WriteString("end tell")
	return b.String()
}
