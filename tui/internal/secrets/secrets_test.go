package secrets

import (
	"os"
	"testing"
)

// 키체인은 진짜를 건드리면 안 된다. 임시 계정명으로만 오간다.
func tempAccount(t *testing.T) string {
	t.Helper()
	a := "test-" + t.Name()
	t.Cleanup(func() { Delete(a) })
	return a
}

// sandbox 는 OpenAI 키가 사는 자리를 임시 항목으로 갈아끼운다.
//
// 이것이 없으면 키를 이미 넣어 둔 기계 — 즉 실제로 쓰는 사람의 기계 —
// 에서는 아래 테스트가 전부 건너뛴다. 도는 곳에서만 도는 테스트는
// 없는 것과 같다.
func sandbox(t *testing.T) {
	t.Helper()
	t.Cleanup(UseAccountForTest(tempAccount(t)))
	t.Setenv(EnvOpenAIKey, "")
}

func TestRoundTrip(t *testing.T) {
	a := tempAccount(t)
	if _, err := Get(a); err != ErrNotFound {
		t.Fatalf("없는 항목인데 err = %v", err)
	}
	if err := Set(a, "sk-hello"); err != nil {
		t.Fatal(err)
	}
	got, err := Get(a)
	if err != nil {
		t.Fatal(err)
	}
	if got != "sk-hello" {
		t.Errorf("읽은 값 %q", got)
	}
	// 덮어쓰기. -U 가 없으면 여기서 "이미 있다"로 실패한다.
	if err := Set(a, "sk-second"); err != nil {
		t.Fatal(err)
	}
	if got, _ := Get(a); got != "sk-second" {
		t.Errorf("덮어쓴 뒤 %q", got)
	}
	if err := Delete(a); err != nil {
		t.Fatal(err)
	}
	if _, err := Get(a); err != ErrNotFound {
		t.Error("지웠는데 아직 있다")
	}
}

func TestSetRejectsEmpty(t *testing.T) {
	if err := Set(tempAccount(t), "   "); err == nil {
		t.Error("빈 값을 저장했다")
	}
}

// 환경변수가 키체인을 이긴다. 개발과 CI 는 덮어쓸 수 있어야 한다.
func TestEnvBeatsKeychain(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-from-env")
	if got := OpenAIKey(); got != "sk-from-env" {
		t.Errorf("%q — 환경변수가 이겨야 한다", got)
	}
	if !HasOpenAIKey() {
		t.Error("키가 있는데 없다고 한다")
	}
}

// 둘 다 없으면 빈 문자열이다. 그것이 "AI 가 꺼졌다"는 뜻이다.
func TestNoKeyMeansOff(t *testing.T) {
	sandbox(t)
	if HasOpenAIKey() {
		t.Error("키가 없는데 있다고 한다")
	}
}

var _ = os.Getenv

// ── 출처 ────────────────────────────────────────────────────────────
//
// 순서(환경변수가 이긴다)는 옳다. 숨기는 것이 함정이었다 — 앱 안에서 키를
// 넣었는데 셸에 다른 키가 있으면, 저장은 되고 "켜졌다"고 답하면서 정작
// 다른 키로 나간다. 잔액이 없는 쪽이 셸 것이면 사용자는 방금 넣은 키를
// 의심한다. 실제로 그 자리에서 한 시간을 썼다.

func TestSourceNamesWhereTheKeyCameFrom(t *testing.T) {
	t.Setenv(EnvOpenAIKey, "sk-from-env")
	if got := OpenAIKeySource(); got != SourceEnv {
		t.Errorf("출처가 %v — 환경변수여야 한다", got)
	}
	// 화면이 이 이름을 그대로 읽어 준다. 사용자가 셸에서 찾을 수 있어야 한다.
	if got := OpenAIKeySource().String(); got != EnvOpenAIKey {
		t.Errorf("%q — 사용자가 셸에서 찾을 이름이 아니다", got)
	}
}

func TestKeychainIsNamedToo(t *testing.T) {
	sandbox(t)
	if err := SaveOpenAIKey("sk-in-keychain"); err != nil {
		t.Fatal(err)
	}
	if got := OpenAIKey(); got != "sk-in-keychain" {
		t.Errorf("키체인 값을 못 읽는다: %q", got)
	}
	if got := OpenAIKeySource(); got != SourceKeychain {
		t.Errorf("출처가 %v — 키체인이어야 한다", got)
	}
}

func TestNoKeyHasNoSource(t *testing.T) {
	sandbox(t)
	if got := OpenAIKeySource(); got != SourceNone {
		t.Errorf("키가 없는데 출처가 %v", got)
	}
	if s := SourceNone.String(); s != "" {
		t.Errorf("없는 출처에 이름이 있다: %q", s)
	}
}

// 넣어 둔 것이 셸에 덮이고 있는가. 이 한 값이 경고를 낼지 정한다.
func TestEnvOverrideIsDetected(t *testing.T) {
	sandbox(t)
	if err := SaveOpenAIKey("sk-in-keychain"); err != nil {
		t.Fatal(err)
	}

	t.Setenv(EnvOpenAIKey, "sk-from-env")
	if !EnvOverridesKeychain() {
		t.Error("둘 다 있는데 덮이고 있다고 안 한다 — 이때 말해 줘야 한다")
	}
	if got := OpenAIKey(); got != "sk-from-env" {
		t.Errorf("%q — 그래도 환경변수가 이겨야 한다", got)
	}

	// 셸에 없으면 알릴 것이 없다. 헷갈릴 일 자체가 없다.
	t.Setenv(EnvOpenAIKey, "")
	if EnvOverridesKeychain() {
		t.Error("셸에 없는데 덮인다고 한다")
	}
}

// 셸에만 있으면 알릴 것이 없다 — 덮인 것이 없다.
func TestEnvAloneIsNotAnOverride(t *testing.T) {
	sandbox(t)
	t.Setenv(EnvOpenAIKey, "sk-from-env")
	if EnvOverridesKeychain() {
		t.Error("키체인이 비었는데 덮인다고 한다")
	}
}
