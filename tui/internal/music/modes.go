package music

import (
	"errors"
	"fmt"
	"strings"
)

// Music.app 의 버튼들.
//
// 이 제품이 파는 것은 리모컨 위의 층이지만(docs/01 §2), 그렇다고 리모컨이
// 없어도 된다는 뜻은 아니다. 섞기를 켜자고 Music.app 을 열어야 하면 "터미널
// 안에서 끝난다"가 거기서 깨진다.
//
// 여기 있는 것은 전부 AppleScript 한 줄짜리다. 무엇을 노출할지 고르는 일이
// 코드를 쓰는 일보다 오래 걸렸다 — 볼륨과 탐색은 미디어 키가 이미 하므로
// 넣지 않았고, 좋아요와 별점은 버튼이 아니라 취향 신호라서 넣었다(docs/03 §7).

var (
	errBadRepeat = errors.New("repeat must be off, one or all")
	errBadVolume = errors.New("volume must be between 0 and 100")
	errBadRating = errors.New("rating must be between 0 and 5")
)

// Repeat 은 반복 상태다. Music.app 의 `song repeat` 그대로다.
type Repeat string

const (
	RepeatOff Repeat = "off"
	RepeatOne Repeat = "one"
	RepeatAll Repeat = "all"
)

// Valid 은 사용자가 친 말이 셋 중 하나인지 본다.
func (r Repeat) Valid() bool {
	return r == RepeatOff || r == RepeatOne || r == RepeatAll
}

// SetShuffle 은 섞기를 켜고 끈다.
//
// **켜면 큐의 순서가 뜻을 잃는다.** 우리가 정한 차례가 곧 큐인데 Music.app 이
// 제멋대로 고르기 때문이고, n번째로 건너뛰는 계산(PlayQueueAt)도 어긋난다.
// 그 어긋남을 다루는 것은 부르는 쪽의 몫이다 — musicapp 은 이미 "화면과
// Music.app 이 다르다"는 표시를 갖고 있다.
func SetShuffle(on bool) error {
	return simple(fmt.Sprintf("set shuffle enabled to %t", on))
}

// SetRepeat 은 반복을 정한다.
func SetRepeat(r Repeat) error {
	if !r.Valid() {
		return errBadRepeat
	}
	return simple("set song repeat to " + string(r))
}

// SetVolume 은 소리 크기를 정한다 (0–100).
func SetVolume(n int) error {
	if n < 0 || n > 100 {
		return errBadVolume
	}
	return simple(fmt.Sprintf("set sound volume to %d", n))
}

// SetFavorite 은 지금 나오는 곡에 좋아요를 켜고 끈다.
//
// `loved` 가 아니라 `favorited` 다. `loved` 는 접근 자체가 막혀 있다 —
// 에러 -10001, docs/05 §3 실측.
//
// 이것은 리모컨 버튼이 아니라 **취향 신호**다. docs/03 §7 이 반응 수집의
// "명시적 긍정"으로 분류해 두었고, 라이브러리 덤프가 이미 이 값을 읽어
// 온다. 다음 /reload 부터 선곡의 근거가 된다.
func SetFavorite(on bool) error {
	return simple(fmt.Sprintf("set favorited of current track to %t", on))
}

// SetRating 은 지금 나오는 곡에 별점을 준다 (0–5).
//
// Music.app 은 0–100 으로 센다. 별 하나가 20이다.
func SetRating(stars int) error {
	if stars < 0 || stars > 5 {
		return errBadRating
	}
	return simple(fmt.Sprintf("set rating of current track to %d", stars*20))
}

// Modes 는 지금 켜져 있는 것들이다.
//
// 우리가 켠 것을 기억하지 않고 매번 읽는다. 사용자가 Music.app 에서 직접
// 켤 수도 있는데, 그때 화면이 옛말을 하면 그것이 거짓말이 된다 —
// "Music.app 이 말해주는 것을 그대로 그린다"(player.go).
type Modes struct {
	Shuffle bool
	Repeat  Repeat

	// Favorited 는 지금 나오는 곡이 좋아요인지다.
	Favorited bool

	// Volume 은 0–100 이다.
	Volume int
}

const modesScript = `tell application "Music"
	set fav to false
	try
		set fav to favorited of current track
	end try
	return (shuffle enabled as text) & "|" & (song repeat as text) & "|" & (fav as text) & "|" & (sound volume as text)
end tell`

// ReadModes 는 켜져 있는 것들을 읽어 온다.
func ReadModes() (Modes, error) {
	if !Running() {
		return Modes{}, ErrNotRunning
	}
	out, err := run(modesScript)
	if err != nil {
		return Modes{}, err
	}
	return parseModes(out), nil
}

// parseModes 는 스크립트가 뱉은 한 줄을 읽는다.
//
// 읽기와 나누어 둔 이유는 이것만 테스트할 수 있게 하기 위해서다.
// 실제로 돌리는 테스트는 이 기계의 Music.app 상태에 기댄다.
func parseModes(out string) Modes {
	f := strings.Split(strings.TrimSpace(out), "|")
	if len(f) < 4 {
		return Modes{}
	}
	m := Modes{
		Shuffle:   f[0] == "true",
		Repeat:    Repeat(strings.TrimSpace(f[1])),
		Favorited: f[2] == "true",
		Volume:    atoiSafe(f[3]),
	}
	// 모르는 값은 껐다고 본다. 화면에 표시가 안 뜰 뿐이고, 잘못된 표시를
	// 띄우는 것보다 낫다.
	if !m.Repeat.Valid() {
		m.Repeat = RepeatOff
	}
	return m
}

func atoiSafe(s string) int {
	n := 0
	for _, r := range strings.TrimSpace(s) {
		if r < '0' || r > '9' {
			return n
		}
		n = n*10 + int(r-'0')
	}
	return n
}
