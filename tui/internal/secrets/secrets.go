// Package secrets 는 살아 있는 자격 증명이 사는 곳이다.
//
// 무엇을 어디에 두느냐가 이 저장소의 규칙이다.
//
//	p8 개인키          파일 0600      라이브러리가 경로를 받으므로 파일이어야 한다
//	Key ID · Team ID   config.json    비밀이 아니다 — 개발자 포털에 그냥 보인다
//	Music User Token   키체인         남의 계정을 여는 열쇠
//	OpenAI 키          키체인         뽑히면 남의 돈이 나간다
//
// 키체인을 Security 프레임워크로 직접 열지 않고 `security` 명령을 부른다.
// 쓰는 것도 읽는 것도 같은 바이너리(/usr/bin/security)라 **권한을 다시 묻지
// 않기 때문이다.** 우리 바이너리가 직접 열면 빌드할 때마다 서명이 달라져
// 사용자에게 프롬프트가 뜬다.
package secrets

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
)

// 키체인 항목 하나를 가리키는 이름. 서비스는 하나로 두고 계정으로 가른다.
const service = "amcli"

// 계정 이름. 여기 없는 이름을 쓰지 않는다 — 오타 하나가 조용히 다른
// 항목을 만들고, 그러면 저장은 되는데 읽히지 않는다.
const (
	AccountOpenAI = "openai-api-key"
)

// ErrNotFound — 그런 항목이 없다. 지워진 것과 처음부터 없던 것을 구별하지 않는다.
var ErrNotFound = errors.New("not in the keychain")

// Get 은 키체인에서 값을 읽는다.
func Get(account string) (string, error) {
	out, err := exec.Command("security", "find-generic-password",
		"-s", service, "-a", account, "-w").Output()
	if err != nil {
		return "", ErrNotFound
	}
	return strings.TrimSpace(string(out)), nil
}

// Set 은 값을 넣는다. 이미 있으면 덮어쓴다.
func Set(account, value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("nothing to save")
	}
	// -U 는 있으면 덮어쓴다. 없으면 새로 만든다.
	cmd := exec.Command("security", "add-generic-password",
		"-s", service, "-a", account, "-w", value, "-U")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return errors.New("could not save to the keychain: " + strings.TrimSpace(stderr.String()))
	}
	return nil
}

// Delete 는 항목을 지운다. 없어도 오류가 아니다.
func Delete(account string) error {
	exec.Command("security", "delete-generic-password",
		"-s", service, "-a", account).Run()
	return nil
}

// ── OpenAI ──────────────────────────────────────────────────────────

// OpenAIKey — ① 환경변수 → ② 키체인.
//
// 설정 파일과 같은 규칙이다(applemusic/settings.go). 위가 항상 이긴다 —
// 개발과 CI 는 환경변수로 덮어쓸 수 있어야 하고, 평소 쓰는 사람은 셸을
// 몰라도 돼야 한다.
func OpenAIKey() string {
	if k := strings.TrimSpace(os.Getenv("OPENAI_API_KEY")); k != "" {
		return k
	}
	k, err := Get(AccountOpenAI)
	if err != nil {
		return ""
	}
	return k
}

// SaveOpenAIKey 는 키를 키체인에 넣는다.
func SaveOpenAIKey(k string) error { return Set(AccountOpenAI, strings.TrimSpace(k)) }

// HasOpenAIKey — AI 를 켤 수 있는가. 이 값 하나로 화면이 갈린다.
func HasOpenAIKey() bool { return OpenAIKey() != "" }
