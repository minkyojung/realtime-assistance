package applemusic

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

// 자격 증명이 사는 곳.
//
// 개발자 토큰은 p8 에서 매번 새로 만든다 — 서명이 순간이라 캐시할 이유가 없다.
// 사용자 토큰만 남겨야 하는데, 이건 남의 계정을 여는 열쇠라
// 홈 디렉터리 평문 파일이 아니라 **키체인**에 둔다.
const (
	keychainService = "amcli-apple-music"
	keychainAccount = "music-user-token"
)

// EmbeddedToken — 배포 빌드가 박아 넣는 개발자 토큰.
//
//	go build -ldflags "-X amcli/tui/internal/applemusic.EmbeddedToken=$TOKEN"
//
// p8 은 개인 키라 배포물에 넣을 수 없다. 넣으면 누구나 뽑아서 우리 명의로
// API 를 쓴다. 그래서 배포되는 것은 그 키로 **미리 서명해 둔 토큰**이다 —
// 유효기간이 최대 6개월이고, 새더라도 만료되면 끝이다.
//
// 비어 있으면 개발 빌드다. 그때는 지금까지처럼 p8 로 직접 서명한다.
var EmbeddedToken string

// Config 는 p8 과 두 개의 ID 다. settings.go 의 3단 폴백이 채운다.
type Config struct {
	P8Path string // AM_P8      · config.json p8Path · 자동 탐색
	KeyID  string // AM_KEY_ID  · config.json keyID  · 파일명에서
	TeamID string // AM_TEAM_ID · config.json teamID
}

// ErrNoTeamID — 나머지는 다 찾았는데 Team ID 만 없다.
//
// 다른 실패와 구별하는 이유는 **사람이 할 일이 다르기 때문이다.** 이건
// 값 하나를 적으면 끝나고, 화면은 그 다음 행동만 말하면 된다.
var ErrNoTeamID = errors.New(
	`team ID is missing — write {"teamID":"..."} to ~/.config/amcli/config.json` +
		"  (developer.apple.com › Membership)")

func LoadConfig() (Config, error) {
	// 박아 넣은 토큰이 있으면 p8 이 필요 없다. 배포판 사용자에게
	// 없는 파일을 내놓으라고 할 수는 없다.
	if EmbeddedToken != "" {
		return Config{}, nil
	}

	file := readSettings()
	c := Config{
		P8Path: firstOf(os.Getenv("AM_P8"), file.P8Path),
		KeyID:  firstOf(os.Getenv("AM_KEY_ID"), file.KeyID),
		TeamID: firstOf(os.Getenv("AM_TEAM_ID"), file.TeamID),
	}
	// ③ 자동 탐색 — 설정 폴더에 AuthKey_<KEYID>.p8 이 하나뿐이면 그것을
	// 쓰고 이름에서 Key ID 를 읽는다. 사람이 적을 것이 하나로 준다.
	if c.P8Path == "" || c.KeyID == "" {
		if path, id := discoverP8(); path != "" {
			c.P8Path = firstOf(c.P8Path, path)
			c.KeyID = firstOf(c.KeyID, id)
		}
	}
	c.P8Path = expandHome(c.P8Path)

	if c.P8Path == "" || c.KeyID == "" {
		return c, errors.New("put AuthKey_<KEYID>.p8 in " + configDirForMessage())
	}
	if c.TeamID == "" {
		return c, ErrNoTeamID
	}
	return c, nil
}

// firstOf — 앞에서부터 비어 있지 않은 첫 값. 우선순위가 곧 순서다.
func firstOf(vs ...string) string {
	for _, v := range vs {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func configDirForMessage() string {
	dir, err := ConfigDir()
	if err != nil {
		return "~/.config/amcli"
	}
	return dir
}

// NewClient 는 개발자 토큰을 만들고, 저장된 사용자 토큰이 있으면 함께 싣는다.
//
// 사용자 토큰이 없어도 클라이언트는 돌아간다. 카탈로그 검색은 그것 없이 되고,
// 검색이 되는 것만으로 이 기능의 대부분이 산다.
func NewClient(cfg Config) (*Client, error) {
	dev := EmbeddedToken
	if dev == "" {
		p8, err := os.ReadFile(cfg.P8Path)
		if err != nil {
			return nil, err
		}
		dev, err = DeveloperToken(p8, cfg.KeyID, cfg.TeamID, 180*24*time.Hour)
		if err != nil {
			return nil, err
		}
	}
	// 로그인 전까지의 지역. 로그인하면 LookupStorefront 가 덮는다.
	c := &Client{DevToken: dev, Storefront: systemStorefront()}
	if tok, err := LoadUserToken(); err == nil {
		c.UserToken = tok
	}
	return c, nil
}

// Login 은 브라우저를 열어 사용자 토큰을 받고 키체인에 넣는다.
func (c *Client) Login(ctx context.Context) error {
	tok, err := Authorize(ctx, c.DevToken)
	if err != nil {
		return err
	}
	if err := SaveUserToken(tok); err != nil {
		return err
	}
	c.UserToken = tok
	if sf, err := c.LookupStorefront(ctx); err == nil {
		c.Storefront = sf
	}
	return nil
}

func SaveUserToken(tok string) error {
	// -U 는 이미 있으면 덮어쓴다. 없으면 새로 만든다.
	cmd := exec.Command("security", "add-generic-password",
		"-s", keychainService, "-a", keychainAccount, "-w", tok, "-U")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return errors.New("could not save to the keychain: " + strings.TrimSpace(stderr.String()))
	}
	return nil
}

func LoadUserToken() (string, error) {
	out, err := exec.Command("security", "find-generic-password",
		"-s", keychainService, "-a", keychainAccount, "-w").Output()
	if err != nil {
		return "", errors.New("no saved sign-in")
	}
	return strings.TrimSpace(string(out)), nil
}

func DeleteUserToken() error {
	return exec.Command("security", "delete-generic-password",
		"-s", keychainService, "-a", keychainAccount).Run()
}
