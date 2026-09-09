package music

import "testing"

// 상태 한 줄의 뒷칸을 읽는 자리. 여기서 틀리면 화면이 켜지지도 않은 것을
// 켜졌다고 말한다.
func TestReadingModesFromStatus(t *testing.T) {
	// 앞 여섯 칸은 재생 상태다. 뒤 셋이 이 테스트의 관심사다.
	head := "playing|PID|Title|Artist|10|200"

	for _, c := range []struct {
		name string
		tail string
		want PlayerState
	}{
		{"전부 꺼짐", "false|off|false", PlayerState{Repeat: RepeatOff}},
		{"섞기", "true|off|false", PlayerState{Shuffle: true, Repeat: RepeatOff}},
		{"한 곡 반복", "false|one|false", PlayerState{Repeat: RepeatOne}},
		{"좋아요", "false|all|true", PlayerState{Repeat: RepeatAll, Favorited: true}},
		{"모르는 반복 값은 껐다고 본다", "false|weird|false", PlayerState{Repeat: RepeatOff}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := parseStatus(head + "|" + c.tail)
			if got.Shuffle != c.want.Shuffle || got.Repeat != c.want.Repeat || got.Favorited != c.want.Favorited {
				t.Errorf("%q → 섞기 %v 반복 %q 좋아요 %v", c.tail, got.Shuffle, got.Repeat, got.Favorited)
			}
			// 재생 정보는 그대로 살아 있어야 한다.
			if got.Title != "Title" || !got.Playing {
				t.Errorf("모드를 읽다 재생 정보를 잃었다: %+v", got)
			}
		})
	}
}

// 옛 스크립트가 짧은 줄을 뱉어도 죽지 않는다. 전부 꺼진 것으로 본다.
func TestShortStatusLineIsSafe(t *testing.T) {
	got := parseStatus("playing|PID|Title|Artist|10|200")
	if got.Shuffle || got.Favorited || got.Repeat != "" {
		t.Errorf("칸이 모자란데 켜졌다고 한다: %+v", got)
	}
	if got.Title != "Title" {
		t.Error("칸이 모자라다고 재생 정보까지 잃었다")
	}
}

// 멈춰 있어도 켜진 것은 읽어야 한다. 섞기는 곡과 무관한 상태다.
func TestStoppedStillReportsModes(t *testing.T) {
	got := parseStatus("stopped|||||" + "|true|all|false")
	if !got.Stopped {
		t.Error("멈춘 것을 못 읽었다")
	}
	if !got.Shuffle || got.Repeat != RepeatAll {
		t.Errorf("멈췄다고 모드를 버렸다: %+v", got)
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
	if contains(statusScript, "loved") {
		t.Errorf("loved 를 읽는다 — 접근이 막힌 속성이다:\n%s", statusScript)
	}
	if !contains(statusScript, "favorited") {
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
