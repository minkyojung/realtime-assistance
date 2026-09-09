package data

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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
