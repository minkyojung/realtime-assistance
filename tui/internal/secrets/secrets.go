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

// 실제로 읽고 쓰는 항목. 테스트가 갈아끼운다(UseAccountForTest).
var openAIAccount = AccountOpenAI

// UseAccountForTest 는 OpenAI 키가 사는 자리를 임시 항목으로 바꾼다.
//
// **테스트 전용이다.** 그럼에도 내보내는 이유는, 이것이 없으면 키를 이미
// 넣어 둔 기계 — 즉 실제로 쓰는 사람의 기계 — 에서 관련 테스트가 전부
// 건너뛰기 때문이다. 도는 곳에서만 도는 테스트는 없는 것과 같다.
//
// 화면을 재는 쪽(musicapp)도 이 자리가 필요해서 패키지 안에 숨길 수 없다.
func UseAccountForTest(name string) (restore func()) {
	old := openAIAccount
	openAIAccount = name
	return func() { openAIAccount = old }
}

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

// 키가 어디서 왔는가. 값만으로는 답할 수 없는 질문이라 따로 돌려준다.
//
// **출처를 숨기면 우선순위가 함정이 된다.** 앱 안에서 키를 넣었는데 셸에
// 다른 키가 있으면, 저장은 되고 "켜졌다"고 답하면서 정작 다른 키로 나간다.
// 잔액이 없는 쪽이 셸 것이면 사용자는 방금 넣은 키를 의심하게 된다 —
// 실제로 그 자리에서 한 시간을 썼다.
//
// gh 와 aws 가 같은 문제를 이렇게 푼다. 순서는 그대로 두고(환경변수가
// 이긴다) **어디서 왔는지를 반드시 말한다.**
type Source int

const (
	SourceNone     Source = iota // 키가 없다
	SourceEnv                    // 셸의 OPENAI_API_KEY
	SourceKeychain               // 앱 안에서 /ai 로 넣은 것
)

// EnvOpenAIKey — 이 이름 하나만 본다. 화면이 사용자에게 그대로 읽어 주므로
// 문자열을 두 번 적지 않는다.
const EnvOpenAIKey = "OPENAI_API_KEY"

func (s Source) String() string {
	switch s {
	case SourceEnv:
		return EnvOpenAIKey
	case SourceKeychain:
		return "keychain"
	}
	return ""
}

// OpenAIKey — ① 환경변수 → ② 키체인.
//
// 설정 파일과 같은 규칙이다(applemusic/settings.go). 위가 항상 이긴다 —
// 개발과 CI 는 환경변수로 덮어쓸 수 있어야 하고, 평소 쓰는 사람은 셸을
// 몰라도 돼야 한다. `OPENAI_API_KEY=<다른키> yarrr` 로 이번 한 번만 다른
// 계정을 쓰는 길이 여기서 나온다.
func OpenAIKey() string { k, _ := openAIKey(); return k }

// OpenAIKeySource — 지금 쓰이는 키가 어디서 왔는가.
func OpenAIKeySource() Source { _, src := openAIKey(); return src }

// EnvOverridesKeychain — 키체인에 넣어 뒀는데 셸이 그것을 덮고 있는가.
//
// 저장한 사람에게 알려 줄 것이 있는 유일한 경우다. 둘 중 하나만 있으면
// 헷갈릴 일이 없다.
func EnvOverridesKeychain() bool {
	if OpenAIKeySource() != SourceEnv {
		return false
	}
	_, err := Get(openAIAccount)
	return err == nil
}

func openAIKey() (string, Source) {
	if k := strings.TrimSpace(os.Getenv(EnvOpenAIKey)); k != "" {
		return k, SourceEnv
	}
	if k, err := Get(openAIAccount); err == nil && k != "" {
		return k, SourceKeychain
	}
	return "", SourceNone
}

// SaveOpenAIKey 는 키를 키체인에 넣는다.
func SaveOpenAIKey(k string) error { return Set(openAIAccount, strings.TrimSpace(k)) }

// HasOpenAIKey — AI 를 켤 수 있는가. 이 값 하나로 화면이 갈린다.
func HasOpenAIKey() bool { return OpenAIKey() != "" }
