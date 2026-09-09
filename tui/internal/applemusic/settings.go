package applemusic

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// 설정이 사는 곳.
//
// 한때 환경변수만 봤다. 그것은 서버·컨테이너의 방식(12-factor)이고 데스크톱
// CLI 에는 안 맞는다 — 터미널마다 다시 export 해야 하고, 셸을 안 거치고 띄우면
// (Finder·앱 번들·launchd) 아예 못 받고, 껐다 켜면 사라진다. 실제로 그래서
// **어제 되던 검색이 오늘 조용히 꺼져 있었다.**
//
// 그래서 세 단으로 본다. 위가 항상 이기고, 아래로 갈수록 느슨하다.
//
//	① 환경변수                     있으면 이긴다. CI·일회성 실험용
//	② ~/.config/amcli/config.json   평소 경로. 셸과 무관하게 산다
//	③ 자동 탐색                     같은 폴더의 AuthKey_*.p8
//
// ③이 이 경우에 특히 값이 크다. 애플이 키 파일을 `AuthKey_<KEYID>.p8` 로
// 내려주므로 **Key ID 를 파일명에서 읽을 수 있다.** p8 경로와 Key ID 가
// 저절로 채워지고, 사람이 적어야 하는 것은 Team ID 하나로 준다.

// settingsFile — config.json 의 모양.
//
// 여기 적히는 것은 **비밀이 아니다.** Key ID 와 Team ID 는 개발자 포털에
// 그냥 보이는 식별자다. 진짜 비밀은 둘뿐이고 각자 제자리가 있다 —
// p8 개인키는 0600 파일, 사용자 토큰은 키체인(config.go).
type settingsFile struct {
	P8Path string `json:"p8Path,omitempty"`
	KeyID  string `json:"keyID,omitempty"`
	TeamID string `json:"teamID,omitempty"`
}

// ConfigDir — `$XDG_CONFIG_HOME/amcli` 또는 `~/.config/amcli`.
//
// os.UserConfigDir 을 쓰지 않는다. macOS 에서 그것은 Library/Application
// Support 를 주는데, p8 은 이미 ~/.config/amcli 에 있고 CLI 도구의 관례도
// 그쪽이다. 두 자리로 갈라 두면 사람이 어느 쪽에 뒀는지 헷갈린다.
func ConfigDir() (string, error) {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "amcli"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "amcli"), nil
}

func settingsPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func readSettings() settingsFile {
	p, err := settingsPath()
	if err != nil {
		return settingsFile{}
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return settingsFile{}
	}
	var s settingsFile
	if json.Unmarshal(b, &s) != nil {
		return settingsFile{}
	}
	return s
}

// SaveTeamID 는 Team ID 를 설정 파일에 적는다.
//
// 사람이 적어야 하는 유일한 값이라 이것만 받는다. 나머지는 탐색이 채운다.
func SaveTeamID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("team ID is empty")
	}
	dir, err := ConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	s := readSettings()
	s.TeamID = id
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	p, err := settingsPath()
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}

// 애플이 내려주는 키 파일 이름. 가운데가 Key ID 다.
var reAuthKey = regexp.MustCompile(`^AuthKey_([A-Z0-9]+)\.p8$`)

// discoverP8 — 설정 폴더에서 키 파일을 찾고 이름에서 Key ID 를 읽는다.
//
// 둘 이상이면 고르지 않는다. 어느 것이 맞는지 우리가 알 수 없고, 잘못
// 고르면 401 이 뜨는데 그 원인이 여기라는 것은 아무도 못 찾는다.
func discoverP8() (path, keyID string) {
	dir, err := ConfigDir()
	if err != nil {
		return "", ""
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", ""
	}
	var found []string
	var ids []string
	for _, e := range entries {
		if m := reAuthKey.FindStringSubmatch(e.Name()); m != nil {
			found = append(found, filepath.Join(dir, e.Name()))
			ids = append(ids, m[1])
		}
	}
	if len(found) != 1 {
		return "", ""
	}
	return found[0], ids[0]
}

// expandHome — `~/` 를 실제 경로로 편다. 설정 파일에도 환경변수에도 쓰인다.
func expandHome(p string) string {
	if !strings.HasPrefix(p, "~/") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, p[2:])
}
