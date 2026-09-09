package host

import (
	"os"
	"testing"

	"amcli/tui/internal/data"
	"amcli/tui/internal/data/fixture"
)

// 화면 테스트는 언제나 같은 라이브러리를 본다. 실물은 기계마다 다르다.
//
// 실시간 덤프는 Init()·/reload 에서만 돈다. 테스트는 둘 다 부르지 않으므로
// 여기서 한 번 심어두면 실물이 테스트로 새지 않는다.
func TestMain(m *testing.M) {
	data.Set(fixture.Lib())
	os.Exit(m.Run())
}
