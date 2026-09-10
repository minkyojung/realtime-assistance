package main

import (
	"bytes"
	"strings"
	"testing"
)

// 화면 하나짜리 앱이지만 시작하는 길은 명령줄이다.
//
// `--help` 를 쳤는데 도움말 대신 앱이 켜지면, 그것은 이 도구가 명령줄
// 도구가 아니라는 뜻이다.

func TestHelpPrintsAndDoesNotStart(t *testing.T) {
	for _, a := range []string{"-h", "--help", "help"} {
		var b bytes.Buffer
		start, code := handleArgs([]string{a}, &b)
		if start {
			t.Errorf("%s 인데 앱을 켜려 한다", a)
		}
		if code != 0 {
			t.Errorf("%s 가 %d 로 끝났다 — 도움말은 오류가 아니다", a, code)
		}
		if !strings.Contains(b.String(), "usage:") {
			t.Errorf("%s 에 쓰는 법이 안 나온다:\n%s", a, b.String())
		}
	}
}

func TestVersionPrintsAndDoesNotStart(t *testing.T) {
	for _, a := range []string{"-v", "--version", "version"} {
		var b bytes.Buffer
		start, code := handleArgs([]string{a}, &b)
		if start || code != 0 {
			t.Errorf("%s 가 앱을 켜거나 오류로 끝났다", a)
		}
		if !strings.HasPrefix(b.String(), "amcli ") {
			t.Errorf("%s 의 답이 이름으로 시작하지 않는다: %q", a, b.String())
		}
	}
}

// 버전은 손으로 관리하지 않는다. 올리는 것을 잊으면 그 숫자가 거짓말이 되고,
// 버그 제보에서 거짓말하는 버전은 없느니만 못하다.
func TestVersionIsNeverEmpty(t *testing.T) {
	if v := strings.TrimSpace(version()); v == "" {
		t.Error("버전이 비었다 — 제보에 적을 것이 없다")
	}
}

// 모르는 인자는 오류다. 조용히 켜지면 오타를 알 길이 없다.
func TestUnknownArgumentIsAnError(t *testing.T) {
	var b bytes.Buffer
	start, code := handleArgs([]string{"--nope"}, &b)
	if start {
		t.Error("모르는 인자인데 앱을 켠다")
	}
	if code == 0 {
		t.Error("모르는 인자인데 성공으로 끝났다")
	}
	if !strings.Contains(b.String(), "--nope") {
		t.Errorf("무엇이 문제인지 안 말한다:\n%s", b.String())
	}
	if !strings.Contains(b.String(), "usage:") {
		t.Errorf("어떻게 쓰는지 안 알려준다:\n%s", b.String())
	}
}

// 인자가 없으면 그냥 앱이다. 이것이 평소의 길이다.
func TestNoArgumentsStartsTheApp(t *testing.T) {
	var b bytes.Buffer
	start, code := handleArgs(nil, &b)
	if !start || code != 0 {
		t.Error("인자가 없는데 앱을 안 켠다")
	}
	if b.Len() != 0 {
		t.Errorf("아무것도 안 물었는데 무언가 찍었다: %q", b.String())
	}
}

// 셸이 남긴 빈 인자 때문에 앱을 못 켜지 않는다.
func TestBlankArgumentsAreIgnored(t *testing.T) {
	if got := clean([]string{"", "  "}); len(got) != 0 {
		t.Errorf("빈 인자가 남았다: %q", got)
	}
	if start, _ := handleArgs(clean([]string{" "}), &bytes.Buffer{}); !start {
		t.Error("빈 인자 하나에 앱이 안 켜진다")
	}
}
