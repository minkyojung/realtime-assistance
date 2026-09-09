// Package fixture 는 화면 테스트용 라이브러리다.
//
// 작성자 본인 Music.app 에서 뽑은 195곡 (docs/05-스파이크-라이브러리-실측.md).
// 골든 테스트가 화면을 바이트 단위로 비교하려면 라이브러리가 기계마다,
// 시각마다 달라져서는 안 된다.
//
// **제품 바이너리는 이것을 링크하지 않는다.** 남의 라이브러리이기 때문이다.
package fixture

import (
	_ "embed"

	"amcli/tui/internal/data"
)

//go:embed library.json
var libraryJSON []byte

// Lib 은 픽스처 스냅샷을 돌려준다.
// 못 읽으면 패닉이다 — 빌드 시점에 이미 정해진 약속이라 실패할 수 없다.
func Lib() *data.Library {
	l, err := data.FromJSON(libraryJSON)
	if err != nil {
		panic("could not read the fixture library.json: " + err.Error())
	}
	return l
}
