package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// 요청 기록.
//
// **요청도 상태다.** 라이브러리 캐시는 지워져도 다시 읽으면 그만이지만,
// 오늘 무엇을 청했는지는 지워지면 영영 돌아오지 않는다 — plays.go 와 같은 자리다.
//
// 지금까지 요청은 메모리에만 살았다. 앱을 끄면 사라졌고, 그래서 재생 기록의
// `context` 칸이 큐 제목을 문자열로 베껴 적고 있었다. 제목은 매번 다른
// 문장이라 묶을 수 없고, 원본 문장은 어디에도 남지 않았다.
//
// 여기에 한 번 적고 재생 기록은 번호만 단다. 40바이트가 8바이트가 되고,
// **무엇보다 원문이 남는다.**

// Context 는 이 요청이 어떤 자리를 위한 것인가다.
//
// 자유 문장으로 두면 집계가 안 되므로 고정된 몇 개로 받는다. **해석이지만
// 되돌릴 수 있는 해석이다** — 원문(Prompt)이 그대로 남아 있으므로, 분류가
// 틀렸다고 판단되면 지난 것을 다시 나눌 수 있다. 그것이 이 값을 저장해도
// 되는 유일한 이유다.
type Context string

// Contexts 는 이 서비스가 아는 자리의 전부다.
//
// **여기가 유일한 목록이다.** 모델에게 보내는 스키마도 이것으로 만든다
// (intent). 두 곳에 적으면 한쪽만 늘어나고, 그러면 모델이 우리가 모르는
// 값을 돌려주는 날이 온다 — host/keys.go 가 겪은 어긋남과 같은 종류다.
var Contexts = []Context{
	"focus",      // 일하거나 공부하는 중
	"workout",    // 운동
	"sleep",      // 자려고
	"commute",    // 이동 중
	"social",     // 사람들과
	"background", // 틀어만 두는 것
	"other",      // 위 어디에도 아닌 것
}

// Turn 은 사람이 한 번 청한 것이다.
type Turn struct {
	ID int64     `json:"id"`
	At time.Time `json:"at"`

	// Prompt 는 **사람이 실제로 친 문장**이다. 모델이 고쳐 쓴 것이 아니다.
	// 이 값이 남아 있는 한 Context 는 언제든 다시 매길 수 있다.
	Prompt string `json:"prompt"`

	// Context 는 모델이 Prompt 를 보고 고른 자리다.
	Context Context `json:"context,omitempty"`

	// Title 은 그 큐에 붙은 이름이다. 사람이 읽을 때만 쓴다.
	Title string `json:"title,omitempty"`
}

func turnsPath() (string, error) {
	dir := os.Getenv("AMCLI_STATE_DIR")
	if dir == "" {
		d, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(d, "amcli")
	}
	return filepath.Join(dir, "turns.jsonl"), nil
}

// AppendTurn 은 요청 하나를 덧붙인다. AppendPlay 와 같은 모양이다.
func AppendTurn(t Turn) error {
	path, err := turnsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(t)
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

// LoadTurns 는 적어 둔 요청을 전부 읽는다. 깨진 줄은 건너뛴다.
func LoadTurns() ([]Turn, error) {
	path, err := turnsPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil // 아직 아무것도 안 물어봤다. 오류가 아니다
	}
	if err != nil {
		return nil, err
	}
	var out []Turn
	for _, line := range splitLines(b) {
		var t Turn
		if json.Unmarshal(line, &t) == nil && t.ID != 0 {
			out = append(out, t)
		}
	}
	return out, nil
}

// TurnsByID 는 번호로 찾을 수 있게 접는다. 재생 기록과 이어 붙일 때 쓴다.
func TurnsByID(ts []Turn) map[int64]Turn {
	out := make(map[int64]Turn, len(ts))
	for _, t := range ts {
		out[t.ID] = t
	}
	return out
}

var (
	turnIDMu   sync.Mutex
	lastTurnID int64
)

// NewTurnID 는 새 번호를 만든다.
//
// 파일 기반이라 셀 곳이 없다. 시각을 쓰면 단조 증가하고 정렬도 된다.
// 줄 번호를 쓰지 않는 이유는 파일이 한 번이라도 다시 써지면 깨지기 때문이다.
//
// **다만 시계만으로는 부족하다.** UnixNano 라는 이름과 달리 macOS 의 실제
// 해상도는 나노초가 아니어서, 연달아 부르면 같은 값이 나온다(테스트가 잡았다).
// 사람이 그 간격 안에 두 번 물어볼 일은 없지만, 같은 번호 둘이 생기면 재생
// 기록 두 무리가 한 요청으로 접히고 **그것은 조용히 틀린다.**
//
// 그래서 마지막에 준 값을 기억한다. 시계가 안 움직였으면 하나 올린다.
func NewTurnID() int64 {
	turnIDMu.Lock()
	defer turnIDMu.Unlock()
	id := time.Now().UnixNano()
	if id <= lastTurnID {
		id = lastTurnID + 1
	}
	lastTurnID = id
	return id
}
