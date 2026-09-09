package applemusic

import "testing"

// 배포 빌드에는 p8 이 없다. 그래도 떠야 한다 —
// 없는 파일을 내놓으라고 하면 그 빌드는 카탈로그를 통째로 잃는다.
func TestEmbeddedTokenNeedsNoP8(t *testing.T) {
	t.Setenv("AM_P8", "")
	t.Setenv("AM_KEY_ID", "")
	t.Setenv("AM_TEAM_ID", "")

	old := EmbeddedToken
	EmbeddedToken = "signed.at.build"
	t.Cleanup(func() { EmbeddedToken = old })

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig with an embedded token: %v", err)
	}
	c, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient with an embedded token: %v", err)
	}
	if c.DevToken != "signed.at.build" {
		t.Errorf("DevToken = %q, want the embedded one", c.DevToken)
	}
}

// 개발 빌드는 지금까지처럼 환경변수를 요구한다.
func TestWithoutEmbeddedTokenConfigIsRequired(t *testing.T) {
	t.Setenv("AM_P8", "")
	t.Setenv("AM_KEY_ID", "")
	t.Setenv("AM_TEAM_ID", "")

	old := EmbeddedToken
	EmbeddedToken = ""
	t.Cleanup(func() { EmbeddedToken = old })

	if _, err := LoadConfig(); err == nil {
		t.Fatal("LoadConfig said yes with nothing set")
	}
}
