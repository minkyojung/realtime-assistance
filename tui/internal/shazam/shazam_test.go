package shazam

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 헬퍼가 뱉는 한 줄이 화면의 갈래를 정한다. 갈래마다 사용자가 할 일이 다르다.
func TestDecode(t *testing.T) {
	cases := []struct {
		name string
		line string
		want error
	}{
		{"못 알아들음", `{"ok":false,"error":"no-match","message":"x"}`, ErrNoMatch},
		{"마이크 거부", `{"ok":false,"error":"microphone-denied","message":"turn it on"}`, ErrMicDenied},
		{"카탈로그 거부", `{"ok":false,"error":"catalog-refused","message":"entitlement"}`, ErrCatalogRefused},
		// 제목이 없으면 성공이라 해도 보여줄 것이 없다.
		{"빈 제목", `{"ok":true,"title":""}`, ErrNoMatch},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Decode([]byte(c.line)); !errors.Is(err, c.want) {
				t.Fatalf("%v 를 기대했는데 %v", c.want, err)
			}
		})
	}
}

// 사용자가 읽을 문장은 헬퍼가 만든다. 그것을 잃어버리면 안 된다.
func TestDecodeKeepsMessage(t *testing.T) {
	_, err := Decode([]byte(`{"ok":false,"error":"microphone-denied","message":"turn it on"}`))
	if err == nil || !strings.Contains(err.Error(), "turn it on") {
		t.Fatalf("헬퍼의 문장이 사라졌다: %v", err)
	}
}

func TestDecodeMatch(t *testing.T) {
	r, err := Decode([]byte(`{"ok":true,"title":"Holocene","artist":"Bon Iver",` +
		`"shazamId":"s1","appleMusicId":"a1","isrc":"US123","genre":"Alternative","year":2011}`))
	if err != nil {
		t.Fatal(err)
	}
	if r.Title != "Holocene" || r.Artist != "Bon Iver" {
		t.Fatalf("곡을 잘못 읽었다: %+v", r)
	}
	// 대조(F11)가 쓰는 키다. 빠뜨리면 라이브러리 보유 여부를 말할 수 없다.
	if r.AppleMusicID != "a1" || r.ISRC != "US123" {
		t.Errorf("대조 키가 비었다: %+v", r)
	}
	if r.Year != 2011 || r.Genre != "Alternative" {
		t.Errorf("S5 가 적는 값이 비었다: %+v", r)
	}
}

// 알아들을 수 없는 출력은 조용히 성공으로 넘어가면 안 된다.
func TestDecodeGarbage(t *testing.T) {
	if _, err := Decode([]byte("not json")); err == nil {
		t.Fatal("쓰레기를 받고도 성공했다")
	}
}

// 헬퍼가 없는 것은 흔한 상태다 — 아직 안 만들었다. 그렇게 말해야 한다.
func TestHelperPathMissing(t *testing.T) {
	t.Setenv("SHAZAMD", filepath.Join(t.TempDir(), "nope"))
	if _, err := HelperPath(); !errors.Is(err, ErrNoHelper) {
		t.Fatalf("ErrNoHelper 를 기대했는데 %v", err)
	}
}

// 실행 권한이 없는 파일은 헬퍼가 아니다.
func TestHelperPathNotExecutable(t *testing.T) {
	p := filepath.Join(t.TempDir(), "shazamd")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHAZAMD", p)
	if _, err := HelperPath(); !errors.Is(err, ErrNoHelper) {
		t.Fatalf("ErrNoHelper 를 기대했는데 %v", err)
	}
	if err := os.Chmod(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := HelperPath(); err != nil || got != p {
		t.Fatalf("실행 가능한 헬퍼를 못 찾았다: %v %v", got, err)
	}
}
