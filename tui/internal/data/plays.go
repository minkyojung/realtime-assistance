package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// 재생 사건 기록.
//
// Music.app 은 **무슨 일이 있었는지**만 센다 — 틀었다 3번, 넘겼다 2번.
// 그런데 **왜** 넘어갔는지는 세지 않고, 우리 앱에서는 넘어가는 이유가 넷이다.
// 사용자가 넘긴 것, 큐에서 뺀 것, 우리가 큐를 갈아끼운 것, 끝까지 들은 것.
// Music.app 의 스킵 수에는 이 넷이 섞여 있다.
//
// 그래서 우리만 아는 것을 적는다 — 왜 끝났나, 몇 초에 나갔나, 어느 요청에서
// 나온 곡인가, 섞기였나. Spotify 의 세션 로그가 reason_end · context_type ·
// is_shuffle 을 남기는 것과 같은 자리다.
//
// **해석은 적지 않는다.** "일찍 나감" 같은 판정을 저장하면 기준을 바꿀 때
// 지난 것을 다시 계산할 수 없다. 3초가 맞는지 5% 가 맞는지는 아직 모른다.
//
// 캐시가 아니라 상태다. 라이브러리 캐시는 지워져도 다시 읽으면 그만이지만,
// 오늘 들은 것은 지워지면 영영 돌아오지 않는다.

// Play 는 곡 하나가 끝난 사건이다.
type Play struct {
	At       time.Time `json:"at"`
	TrackID  int64     `json:"trackId"`
	PlayedMs int       `json:"playedMs"`

	// DurMs 는 **그때의** 곡 길이다. 비율은 나중에 다시 계산하므로,
	// 곡이 다른 판본으로 바뀌어도 그날의 셈이 틀어지지 않아야 한다.
	DurMs int `json:"durMs"`

	// EndedBy 는 무엇이 이 곡을 끝냈는가다. 이 기록의 존재 이유다.
	EndedBy EndedBy `json:"endedBy"`

	// Shuffle 은 섞기가 켜져 있었는가다. 켜진 상태의 스킵은 순서 재생의
	// 스킵과 뜻이 다르다 — 듣기 싫어서가 아니라 그 자리에 안 맞아서일 수 있다.
	Shuffle bool `json:"shuffle,omitempty"`

	// Context 는 어느 요청에서 나온 곡인가다. 취향 규칙은 이것 없이 못 뽑는다 —
	// "새벽엔 가사 있는 곡을 피한다" 는 언제 무엇을 청했는지를 알아야 나온다.
	Context string `json:"context,omitempty"`
}

// EndedBy 는 곡이 끝난 이유다.
type EndedBy string

const (
	// EndedDone — 끝까지 흘렀다.
	EndedDone EndedBy = "trackdone"
	// EndedSkipped — 누가 넘겼다. 우리가 아니면 사용자다 (터미널·Music.app·에어팟).
	EndedSkipped EndedBy = "skipped"
	// EndedRemoved — 큐에서 뺐다. 사용자가 그 곡을 지목해 거절한 것이다.
	EndedRemoved EndedBy = "removed"
	// EndedPicked — 사용자가 다른 곡을 골랐다.
	EndedPicked EndedBy = "picked"

	// EndedRequeue — **우리가 큐를 갈아끼웠다. 취향 신호가 아니다.**
	//
	// 이것을 구별하지 못하면 우리가 만든 행동을 사용자의 취향으로 읽는다.
	// 그리고 조용히 틀린다 — 쌓일수록 더 확신하면서 틀린다.
	EndedRequeue EndedBy = "requeue"

	// EndedStopped — 재생이 멈췄다.
	EndedStopped EndedBy = "stopped"
)

// Signal 은 이 사건이 취향의 근거가 되는가다.
//
// 우리가 끊은 것은 근거가 아니다. 판정이 아니라 분류이므로 여기 둔다 —
// 무엇이 신호인지는 기준이 바뀌어도 바뀌지 않는다.
func (e EndedBy) Signal() bool { return e != EndedRequeue && e != EndedStopped }

func playsPath() (string, error) {
	dir := os.Getenv("AMCLI_STATE_DIR")
	if dir == "" {
		d, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(d, "amcli")
	}
	return filepath.Join(dir, "plays.jsonl"), nil
}

// AppendPlay 는 사건 하나를 덧붙인다.
//
// 고치지도 지우지도 않으므로 망가질 데가 없다. 한 줄이 200바이트 남짓이라
// O_APPEND 쓰기가 통째로 들어가고, 다른 프로세스와 섞이지 않는다.
//
// 실패해도 재생을 막지 않는다. 못 적은 사건 하나보다 멎은 화면이 나쁘다.
func AppendPlay(p Play) error {
	path, err := playsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}

// LoadPlays 는 적어 둔 사건을 전부 읽는다.
//
// 통째로 읽는다. 한 줄이 200바이트라 1년치가 11MB 남짓이고, 요약본을 따로
// 두면 원본과 어긋나는 새 버그가 생긴다. 느려지면 그때 만든다.
//
// 깨진 줄은 건너뛴다. 쓰다 만 줄 하나 때문에 지난 기록을 통째로 잃을 이유가 없다.
func LoadPlays() ([]Play, error) {
	path, err := playsPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil // 아직 아무것도 안 들었다. 오류가 아니다
	}
	if err != nil {
		return nil, err
	}
	var out []Play
	for _, line := range splitLines(b) {
		var p Play
		if json.Unmarshal(line, &p) == nil && p.TrackID != 0 {
			out = append(out, p)
		}
	}
	return out, nil
}

func splitLines(b []byte) [][]byte {
	var out [][]byte
	start := 0
	for i, c := range b {
		if c == '\n' {
			if i > start {
				out = append(out, b[start:i])
			}
			start = i + 1
		}
	}
	if start < len(b) {
		out = append(out, b[start:])
	}
	return out
}

// 쌓아 둔 사건을 곡별로 접는다.
//
// **해석은 여기서 한다.** 파일에는 날것만 적는다(위) — 무엇이 "일찍"인지는
// 아직 확신이 없고, 기준을 바꿀 때 지난 기록을 다시 셀 수 있어야 하기
// 때문이다. 그 기준이 사는 곳이 여기다.

// 절반을 못 채우고 나갔으면 일찍 나간 것으로 본다.
//
// 업계의 두 관습 사이에서 고른 값이다 — Last.fm 은 절반이나 4분을 들으면
// 들은 것으로 치고, Spotify 는 30초 미만을 스킵으로 센다. 절반을 쓰면
// 3분짜리에서 90초, 8분짜리에서 4분이라 곡 길이에 따라 같이 움직인다.
//
// **틀려도 되는 값이다.** 날것이 남아 있으므로 나중에 다시 셀 수 있다.
const earlyRatio = 0.5

// Reaction 은 한 곡에 대해 우리가 본 것의 요약이다.
type Reaction struct {
	Early   int // 절반도 안 듣고 나갔다
	Done    int // 끝까지 흘렀다
	Removed int // 큐에서 지목해 뺐다
}

// Reactions 는 사건을 곡별로 접는다.
//
// **우리가 만든 사건은 안 센다**(Signal). 큐를 갈아끼우느라 끊긴 곡을
// 취향으로 읽으면, 우리 행동을 사용자의 마음으로 착각한 채 쌓일수록 더
// 확신하면서 틀린다.
func Reactions(ps []Play) map[int64]Reaction {
	out := make(map[int64]Reaction)
	for _, p := range ps {
		if !p.EndedBy.Signal() {
			continue
		}
		r := out[p.TrackID]
		switch {
		case p.EndedBy == EndedRemoved:
			r.Removed++
		case p.EndedBy == EndedDone:
			r.Done++
		case p.DurMs > 0 && float64(p.PlayedMs) < float64(p.DurMs)*earlyRatio:
			r.Early++
		}
		out[p.TrackID] = r
	}
	return out
}
