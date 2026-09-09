package host

import "runtime/debug"

// 버전. 릴리스 빌드가 박아 넣는다.
//
//	go build -ldflags "-X amcli/tui/internal/host.Version=0.1.0"
//
// 커밋은 여기 두지 않는다. go build 가 git 에서 읽어 빌드 정보에 넣어
// 주므로, 같은 것을 두 군데서 관리할 이유가 없다.
var Version = "0.1.0-dev"

// buildStamp 는 "v0.1.0 · 9ff5de0" 을 만든다.
//
// 커밋을 모르는 실행이 있다 — `go run` 과 테스트에는 vcs 정보가 붙지
// 않는다. 그때는 버전만 적는다. 없는 자리를 "unknown" 으로 채우면
// 화면이 모르는 것을 아는 척하게 된다.
func buildStamp() string {
	s := "v" + Version
	if rev := vcsRevision(); rev != "" {
		s += " · " + rev
	}
	return s
}

func vcsRevision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && len(s.Value) >= 7 {
			return s.Value[:7]
		}
	}
	return ""
}
