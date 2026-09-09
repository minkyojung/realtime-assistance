package slackapp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// 토큰을 어디에 둘 것인가.
//
// **이 제품은 지금까지 아무것도 저장하지 않았다.** 로그조차 세션 동안만
// 산다. 로그인을 브라우저로 옮기는 대가가 정확히 이것이다 — 저장할 것이
// 하나 생긴다.
//
// # 왜 키체인이 아닌가
//
// macOS 전용 앱이니 키체인이 맞아 보이지만, Go 에서 키체인에 닿으려면
// security(1) 를 부르는 수밖에 없고 그 명령은 비밀을 **인자로** 받는다.
// 같은 사용자의 다른 프로세스가 ps 로 볼 수 있다. 파일 권한 0600 이
// 그보다 낫고, gh·aws·gcloud 가 전부 이 방식이다.
//
// 유효 범위는 사용자 계정이다. 이 파일을 읽을 수 있는 사람은 이미
// 그 사람의 Slack 세션도 열 수 있다.

const (
	storeDir  = "amcli"
	storeFile = "slack.json"
)

// creds 는 로그인 한 번의 결과 전부다.
//
// Refresh 가 비어 있는 것은 정상이다. 앱 설정에서 토큰 회전을 켜지 않으면
// Slack 은 만료되지 않는 액세스 토큰 하나만 준다.
type creds struct {
	Access  string    `json:"access_token"`
	Refresh string    `json:"refresh_token,omitempty"`
	Expires time.Time `json:"expires_at,omitempty"`
	Team    string    `json:"team,omitempty"`
}

// expired 는 지금 이 토큰으로 부르면 안 되는지 본다.
// 만료 직전에 걸리지 않도록 1분을 미리 당긴다.
func (c creds) expired() bool {
	return !c.Expires.IsZero() && time.Now().After(c.Expires.Add(-time.Minute))
}

func storePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, storeDir, storeFile), nil
}

// loadCreds 는 저장된 토큰을 읽는다.
//
// 없는 것은 실패가 아니다 — 아직 로그인하지 않았다는 뜻이고, 그 상태는
// 관문 화면이 이미 다룬다. 깨진 파일도 같게 취급한다. 다시 로그인하면
// 덮어쓰이므로 사용자가 할 일은 어느 쪽이든 같다.
func loadCreds() creds {
	path, err := storePath()
	if err != nil {
		return creds{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return creds{}
	}
	var c creds
	if json.Unmarshal(data, &c) != nil {
		return creds{}
	}
	return c
}

// saveCreds 는 토큰을 사용자만 읽을 수 있게 쓴다.
//
// 임시 파일에 쓰고 옮긴다. 갱신 도중에 죽으면 반쯤 쓰인 파일이 남고,
// 그러면 다음 실행에서 조용히 로그아웃된 것처럼 보인다.
func saveCreds(c creds) error {
	path, err := storePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func clearCreds() error {
	path, err := storePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
