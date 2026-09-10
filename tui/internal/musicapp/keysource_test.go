package musicapp

import (
	"strings"
	"testing"

	"amcli/tui/internal/secrets"
)

// 첫 화면은 **어느 키를 쓰는지**까지 말한다.
//
// 환경변수가 키체인을 이긴다(secrets). 그 순서는 옳지만, 말하지 않으면
// 앱 안에서 넣은 키가 조용히 무시된다. 잔액이 없는 쪽이 셸 것이면
// 사용자는 방금 넣은 키를 의심하게 된다 — 화면이 맞다고 한 것을.
func TestFirstScreenNamesTheKeySource(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-from-env")
	if got := aiDetail(t); !strings.Contains(got, secrets.EnvOpenAIKey) {
		t.Errorf("%q — 어느 키를 쓰는지 말하지 않는다", got)
	}
}

// 꺼져 있으면 켜는 법만 말한다. 없는 키의 출처를 적을 자리는 없다.
func TestOffSaysHowToTurnOn(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Cleanup(secrets.UseAccountForTest("test-musicapp-no-key"))
	got := aiDetail(t)
	if !strings.Contains(got, "/ai") {
		t.Errorf("%q — 켜는 법이 없다", got)
	}
	// 비밀은 이어 치지 않는다(host/secret.go). 시키는 말에 `<key>` 가
	// 있으면 화면에 남기지 말라고 만든 길을 화면이 스스로 권한다.
	if strings.Contains(got, "<key>") {
		t.Errorf("%q — 키를 이어 치라고 한다", got)
	}
}

func aiDetail(t *testing.T) string {
	t.Helper()
	m := New()
	for _, f := range m.Facts() {
		if f.Name == "AI" {
			return f.Detail
		}
	}
	t.Fatal("첫 화면에 AI 줄이 없다")
	return ""
}
