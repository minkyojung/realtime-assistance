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
	t.Setenv("OPENAI_API_KEY", "")
	// 키체인에 진짜 키가 있는 기계에서는 이 단정을 못 한다.
	if _, err := Get(AccountOpenAI); err == nil {
		t.Skip("이 기계 키체인에 실제 키가 들어 있다")
	}
	if HasOpenAIKey() {
		t.Error("키가 없는데 있다고 한다")
	}
}

var _ = os.Getenv
