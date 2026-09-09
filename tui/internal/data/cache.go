package data

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// 첫 실행에 라이브러리를 읽는 데 몇 초가 든다. 그 값을 두 번 치르지 않는다.
//
// 만료로 지우지 않는다. 시작할 때마다 뒤에서 실물을 다시 읽으므로
// 낡음의 수명은 언제나 덤프 한 번이다.
const libraryVersion = 1

func cachePath() (string, error) {
	if dir := os.Getenv("AMCLI_CACHE_DIR"); dir != "" {
		return filepath.Join(dir, "library.json"), nil
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "amcli", "library.json"), nil
}

// LoadCache 는 마지막으로 읽은 스냅샷을 돌려준다. 없으면 오류다.
func LoadCache() (*Library, error) {
	p, err := cachePath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	l, err := FromJSON(b)
	if err != nil {
		return nil, err
	}
	// 모양이 바뀌었으면 없는 것으로 친다. 곧 실물이 덮는다.
	if l.Version != libraryVersion || len(l.Tracks) == 0 {
		return nil, errors.New("cache is stale")
	}
	return l, nil
}

// SaveCache 는 스냅샷을 캐시에 쓴다.
//
// 임시 파일에 쓰고 옮긴다. 쓰다 죽어도 반쪽짜리 캐시가 남지 않는다.
// 실패해도 치명적이지 않다 — 다음 시작이 조금 느릴 뿐이다.
func SaveCache(l *Library) error {
	p, err := cachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(l)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), "library-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), p)
}

// 큐 플레이리스트의 persistent ID.
//
// 이것을 기억하는 이유는 **이름으로 지우지 않기 위해서**다. 사용자가 우연히
// 같은 이름의 플레이리스트를 갖고 있을 때 그것을 지워 버리면 되돌릴 수 없다.
// 우리가 만든 것의 ID 를 적어 두고 그것만 지운다(music/queue.go).
//
// 이 파일이 사라지면 지난번 플레이리스트가 사이드바에 남고 새 것이 하나 더
// 생긴다. 지저분하지만 남의 것을 지우는 것보다는 낫다.
func queuePIDPath() (string, error) {
	p, err := cachePath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "queue-playlist"), nil
}

// LoadQueuePID 는 지난번에 우리가 만든 큐 플레이리스트의 ID 를 읽는다.
// 없으면 빈 문자열이다 — 첫 실행의 정상 상태이므로 오류로 만들지 않는다.
func LoadQueuePID() string {
	p, err := queuePIDPath()
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// SaveQueuePID 는 방금 만든 큐 플레이리스트의 ID 를 적어 둔다.
func SaveQueuePID(pid string) error {
	p, err := queuePIDPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(pid), 0o644)
}
