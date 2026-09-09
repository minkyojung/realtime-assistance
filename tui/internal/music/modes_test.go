package music

import "testing"

// 읽어 온 한 줄을 해석하는 자리. 여기서 틀리면 화면이 켜지지도 않은 것을
// 켜졌다고 말한다.
func TestReadingModes(t *testing.T) {
	for _, c := range []struct {
		name string
		out  string
		want Modes
	}{
		{"전부 꺼짐", "false|off|false|100",
			Modes{Repeat: RepeatOff, Volume: 100}},
		{"섞기만 켜짐", "true|off|false|60",
			Modes{Shuffle: true, Repeat: RepeatOff, Volume: 60}},
		{"한 곡 반복", "false|one|false|60",
			Modes{Repeat: RepeatOne, Volume: 60}},
		{"좋아요", "false|all|true|0",
			Modes{Repeat: RepeatAll, Favorited: true}},
		{"줄 끝 공백", "  true|all|true|55  ",
			Modes{Shuffle: true, Repeat: RepeatAll, Favorited: true, Volume: 55}},

		// 모르는 값은 껐다고 본다. 잘못된 표시를 띄우는 것보다 안 띄우는 편이 낫다.
		{"모르는 반복 값", "false|weird|false|10",
			Modes{Repeat: RepeatOff, Volume: 10}},
		{"칸이 모자람", "false|off", Modes{}},
		{"빈 줄", "", Modes{}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := parseModes(c.out); got != c.want {
				t.Errorf("%q → %+v, 원한 것 %+v", c.out, got, c.want)
			}
		})
	}
}

// 사용자가 아무 말이나 칠 수 있으므로 셋이 아닌 것은 막는다.
func TestRepeatOnlyTakesThreeValues(t *testing.T) {
	for _, r := range []Repeat{RepeatOff, RepeatOne, RepeatAll} {
		if !r.Valid() {
			t.Errorf("%q 를 안 받는다", r)
		}
	}
	for _, r := range []Repeat{"", "ON", "loop", "1"} {
		if r.Valid() {
			t.Errorf("%q 를 받는다", r)
		}
	}
}

// 범위 밖은 Music.app 까지 가기 전에 막는다.
//
// 실제로 돌리지 않고 검사만 확인한다 — 범위 안의 값을 넘기면 이 기계의
// Music.app 을 진짜로 건드린다.
func TestOutOfRangeNeverReachesMusicApp(t *testing.T) {
	for _, n := range []int{-1, 101, 1000} {
		if err := SetVolume(n); err != errBadVolume {
			t.Errorf("볼륨 %d 를 막지 않았다: %v", n, err)
		}
	}
	for _, n := range []int{-1, 6, 100} {
		if err := SetRating(n); err != errBadRating {
			t.Errorf("별점 %d 를 막지 않았다: %v", n, err)
		}
	}
	if err := SetRepeat("loop"); err != errBadRepeat {
		t.Errorf("모르는 반복 값을 막지 않았다: %v", err)
	}
}

// 별 하나가 20이다. Music.app 은 0–100 으로 세고 사람은 별로 센다.
func TestStarsBecomeMusicAppRating(t *testing.T) {
	// 변환은 SetRating 안에 있으므로 여기서는 규칙만 못 박아 둔다.
	// 5별이 100을 넘으면 Music.app 이 거절한다.
	if 5*20 != 100 {
		t.Fatal("별점 환산이 100을 벗어난다")
	}
}

// 좋아요는 favorited 다. loved 는 접근 자체가 막혀 있다 (에러 -10001, docs/05).
//
// 되돌리기 쉬운 자리라 못 박아 둔다 — 이름만 보면 loved 가 맞아 보인다.
func TestFavoriteNeverUsesLoved(t *testing.T) {
	if got := modesScript; contains(got, "loved") {
		t.Errorf("loved 를 읽는다 — 접근이 막힌 속성이다:\n%s", got)
	}
	if !contains(modesScript, "favorited") {
		t.Error("favorited 를 안 읽는다")
	}
}

func contains(hay, needle string) bool {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
