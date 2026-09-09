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
	// 설정 폴더도 함께 비워야 한다. 환경변수만 지우면 이 기계에 실제로
	// 놓인 config.json 과 p8 을 읽어서 "설정이 있다"가 되어 버린다 —
	// 개발자 기계에 그것이 놓이는 순간 조용히 통과했다.
	sandbox(t)

	old := EmbeddedToken
	EmbeddedToken = ""
	t.Cleanup(func() { EmbeddedToken = old })

	if _, err := LoadConfig(); err == nil {
		t.Fatal("LoadConfig said yes with nothing set")
	}
}

// 로그인 전에는 이 맥의 지역이 답이다. 못 읽으면 빈 값이고,
// 그때는 storefront() 가 kr 로 떨어진다.
func TestRegionFrom(t *testing.T) {
	for _, c := range []struct{ locale, want string }{
		{"ko_KR", "kr"},
		{"en_KR", "kr"},
		{"en_US", "us"},
		{"zh-Hans_CN", "cn"},
		{"en_US@rg=krzzzz", "kr"}, // 지역을 언어와 따로 고른 경우
		{"en_US@rg=", "us"},       // 망가진 꼬리는 무시하고 뒤로 물러난다
		{"C", ""},
		{"", ""},
		{"en_1X", ""},
	} {
		if got := regionFrom(c.locale); got != c.want {
			t.Errorf("regionFrom(%q) = %q, want %q", c.locale, got, c.want)
		}
	}
}

// 지역을 모를 때 검색이 멎으면 안 된다. kr 로라도 간다.
func TestStorefrontFallsBackToKR(t *testing.T) {
	if got := (&Client{}).storefront(); got != "kr" {
		t.Errorf("storefront() = %q, want kr", got)
	}
	if got := (&Client{Storefront: "us"}).storefront(); got != "us" {
		t.Errorf("storefront() = %q, want us", got)
	}
}
