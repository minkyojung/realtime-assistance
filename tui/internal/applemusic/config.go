package applemusic

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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

// Config 는 p8 과 두 개의 ID 다. 환경변수로 받는다.
type Config struct {
	P8Path string // AM_P8
	KeyID  string // AM_KEY_ID
	TeamID string // AM_TEAM_ID
}

func LoadConfig() (Config, error) {
	c := Config{
		P8Path: os.Getenv("AM_P8"),
		KeyID:  os.Getenv("AM_KEY_ID"),
		TeamID: os.Getenv("AM_TEAM_ID"),
	}
	if c.P8Path == "" || c.KeyID == "" || c.TeamID == "" {
		return c, errors.New("set AM_P8, AM_KEY_ID and AM_TEAM_ID")
	}
	if strings.HasPrefix(c.P8Path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return c, err
		}
		c.P8Path = filepath.Join(home, c.P8Path[2:])
	}
	return c, nil
}

// NewClient 는 개발자 토큰을 만들고, 저장된 사용자 토큰이 있으면 함께 싣는다.
//
// 사용자 토큰이 없어도 클라이언트는 돌아간다. 카탈로그 검색은 그것 없이 되고,
// 검색이 되는 것만으로 이 기능의 대부분이 산다.
func NewClient(cfg Config) (*Client, error) {
	p8, err := os.ReadFile(cfg.P8Path)
	if err != nil {
		return nil, err
	}
	dev, err := DeveloperToken(p8, cfg.KeyID, cfg.TeamID, 180*24*time.Hour)
	if err != nil {
		return nil, err
	}
	c := &Client{DevToken: dev}
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
