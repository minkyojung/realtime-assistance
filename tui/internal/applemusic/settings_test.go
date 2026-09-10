package applemusic

import (
	"os"
	"path/filepath"
	"testing"
)

// 설정 폴더를 임시로 돌려놓는다. 진짜 설정을 건드리면 안 된다.
func sandbox(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	for _, v := range []string{"AM_P8", "AM_KEY_ID", "AM_TEAM_ID"} {
		t.Setenv(v, "")
	}
	amcli := filepath.Join(dir, "amcli")
	if err := os.MkdirAll(amcli, 0o700); err != nil {
		t.Fatal(err)
	}
	return amcli
}

func writeKey(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("-----BEGIN PRIVATE KEY-----"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// 키 파일 하나만 두면 경로와 Key ID 가 저절로 채워진다.
// 사람이 적어야 하는 것은 Team ID 하나뿐이다.
func TestDiscoversKeyAndID(t *testing.T) {
	dir := sandbox(t)
	want := writeKey(t, dir, "AuthKey_4CD6AXSGN5.p8")

	if _, err := LoadConfig(); err != ErrNoTeamID {
		t.Fatalf("Team ID 만 없어야 하는데 err = %v", err)
	}
	if err := SaveTeamID("ABCDE12345"); err != nil {
		t.Fatal(err)
	}
	c, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.P8Path != want {
		t.Errorf("p8 경로 %q, 기대 %q", c.P8Path, want)
	}
	if c.KeyID != "4CD6AXSGN5" {
		t.Errorf("Key ID %q — 파일명에서 읽어야 한다", c.KeyID)
	}
	if c.TeamID != "ABCDE12345" {
		t.Errorf("Team ID %q", c.TeamID)
	}
}

// 환경변수가 설정 파일을 이긴다. 위가 항상 이기는 것이 3단 폴백의 규칙이다.
func TestEnvWins(t *testing.T) {
	dir := sandbox(t)
	writeKey(t, dir, "AuthKey_4CD6AXSGN5.p8")
	if err := SaveTeamID("FROMFILE00"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AM_TEAM_ID", "FROMENV000")

	c, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.TeamID != "FROMENV000" {
		t.Errorf("Team ID %q — 환경변수가 이겨야 한다", c.TeamID)
	}
}

// 키 파일이 둘이면 고르지 않는다. 잘못 고르면 401 이 뜨는데,
// 그 원인이 여기라는 것은 아무도 못 찾는다.
func TestTwoKeysIsAmbiguous(t *testing.T) {
	dir := sandbox(t)
	writeKey(t, dir, "AuthKey_AAAAAAAAAA.p8")
	writeKey(t, dir, "AuthKey_BBBBBBBBBB.p8")
	if err := SaveTeamID("ABCDE12345"); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(); err == nil {
		t.Error("키가 둘인데 하나를 골랐다")
	}
}

// Team ID 를 다시 적어도 앞서 적힌 것이 안 날아간다.
func TestSaveKeepsOtherFields(t *testing.T) {
	dir := sandbox(t)
	writeKey(t, dir, "AuthKey_4CD6AXSGN5.p8")
	if err := SaveTeamID("FIRST00000"); err != nil {
		t.Fatal(err)
	}
	if err := SaveTeamID("SECOND0000"); err != nil {
		t.Fatal(err)
	}
	c, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.TeamID != "SECOND0000" {
		t.Errorf("Team ID %q", c.TeamID)
	}
	// 파일 권한은 0600 이어야 한다. 비밀은 아니지만 남의 계정 정보다.
	p, _ := settingsPath()
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("설정 파일 권한이 %v", fi.Mode().Perm())
	}
}

// 아무것도 없으면 무엇을 하면 되는지 말해야 한다.
func TestNoKeySaysWhereToPutIt(t *testing.T) {
	sandbox(t)
	_, err := LoadConfig()
	if err == nil {
		t.Fatal("키가 없는데 성공했다")
	}
	if !filepath.IsAbs(err.Error()[len(err.Error())-1:]) && err == ErrNoTeamID {
		t.Error("키가 없는데 Team ID 탓을 한다")
	}
	t.Logf("문구: %v", err)
}
