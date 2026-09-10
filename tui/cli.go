package main

import (
	"fmt"
	"io"
	"runtime/debug"
	"strings"
)

// 명령줄에서 물어볼 수 있는 것들.
//
// 화면 하나짜리 앱이지만 시작하는 길은 명령줄이다. `--help` 를 쳤는데
// 도움말 대신 앱이 켜지면, 그것은 이 도구가 명령줄 도구가 아니라는 뜻이다.
//
// 플래그 패키지를 쓰지 않는다. 받는 것이 둘뿐이고, flag 를 들이면 `-h` 가
// 자기 형식으로 답하면서 아래 글이 화면에 안 나온다.

// 이 도구를 부르는 이름. **한 곳에만 산다.**
//
// 한때 usage 와 --version 과 오류 문구에 각각 적혀 있었다. 이름을 바꾸는
// 순간 세 군데를 다 고쳐야 하고, 하나를 빠뜨리면 화면이 자기를 두 이름으로
// 부른다. 실제로 amcli 에서 바꿀 때 테스트가 그것을 잡았다.
const name = "yarrr"

const usage = `Apple Music from your terminal — ask for music in your own words.

usage: ` + name + ` [--help] [--version]

Nothing to configure to look around. To let it pick for you, add a key:

    /ai             turn on AI — it asks for your provider API key

Inside the app, ? lists every key.
`

// run 은 명령줄 인자를 본다. 앱을 켤 것이면 true 를 돌려준다.
//
// 종료 코드를 직접 안 부르고 돌려주는 이유는 테스트에서 부를 수 있게
// 하려는 것이다 — os.Exit 는 테스트까지 같이 죽인다.
func handleArgs(args []string, out io.Writer) (start bool, code int) {
	for _, a := range args {
		switch a {
		case "-h", "--help", "help":
			fmt.Fprint(out, usage)
			return false, 0
		case "-v", "--version", "version":
			fmt.Fprintln(out, name, version())
			return false, 0
		default:
			fmt.Fprintf(out, "%s: unknown argument %q\n\n%s", name, a, usage)
			return false, 2
		}
	}
	return true, 0
}

// version 은 빌드에 박힌 것을 읽는다.
//
// 손으로 관리하는 상수를 두지 않는다. 올리는 것을 잊으면 그 숫자가
// 거짓말이 되고, 버그 제보에서 거짓말하는 버전은 없느니만 못하다.
//
// go build 는 저장소가 깨끗하면 커밋 해시를, 아니면 dirty 표시를 심는다.
// go install 로 받았으면 모듈 버전이 들어 있다.
func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "(unknown build)"
	}
	var rev, dirty string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "+dirty"
			}
		}
	}
	if rev != "" {
		if len(rev) > 12 {
			rev = rev[:12]
		}
		return rev + dirty
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	return "(built from source)"
}

// 인자에 붙은 공백은 셸이 남기는 흔한 사고다. 그것 때문에 앱을 못 켜지 않는다.
func clean(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a = strings.TrimSpace(a); a != "" {
			out = append(out, a)
		}
	}
	return out
}
