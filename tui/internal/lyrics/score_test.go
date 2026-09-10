package lyrics

import "testing"

// LRCLIB 이 실제로 돌려준 `Perth` 검색 결과다. 실제 곡은 4:22(262초).
//
// 첫 번째가 2452초짜리 — 앨범을 통째로 한 항목에 올린 것이고, 시간표가
// 붙어 있다. "시각이 있는 첫 번째"를 집으면 이게 걸린다.
var perth = []payload{
	{Synced: "[00:01.00] a", Plain: "a", Duration: 2452.0},
	{Plain: "b", Duration: 255.55},
	{Synced: "[00:01.00] c", Plain: "c", Duration: 262.08},
	{Plain: "d", Duration: 262.0},
	{Synced: "[00:01.00] e", Plain: "e", Duration: 262.0},
}

const perthMs = 262_000

// 길이가 40분 어긋난 후보가 이기면 안 된다. 이것이 이 함수의 존재 이유다.
func TestScoreRejectsWrongRecording(t *testing.T) {
	album := score(perth[0], perthMs) // 2452초, 시간표 있음
	right := score(perth[2], perthMs) // 262초, 시간표 있음
	if album >= right {
		t.Fatalf("40분짜리(%d)가 제 길이(%d)를 이겼다", album, right)
	}
	if right != scorePerfect {
		t.Errorf("길이 맞고 시간표 있는데 %d점 — 만점(%d)이어야 한다", right, scorePerfect)
	}
}

// 틀린 시간표는 시간표가 없는 것보다 나쁘다.
func TestRightPlainBeatsWrongSynced(t *testing.T) {
	wrongSynced := score(perth[0], perthMs) // 2452초 + 시간표
	rightPlain := score(perth[3], perthMs)  // 262초, 글자만
	if rightPlain <= wrongSynced {
		t.Errorf("길이 맞는 줄글(%d)이 길이 틀린 싱크(%d)를 못 이긴다", rightPlain, wrongSynced)
	}
}

// 후보 다섯 중 최고점은 길이도 맞고 시간표도 있는 것이어야 한다.
func TestBestOfPerth(t *testing.T) {
	best, bestScore := -1, unusable
	for i, p := range perth {
		if s := score(p, perthMs); s > bestScore {
			best, bestScore = i, s
		}
	}
	if best != 2 && best != 4 {
		t.Errorf("%d번째를 골랐다 — 2 또는 4여야 한다 (262초 + 시간표)", best)
	}
	if bestScore != scorePerfect {
		t.Errorf("최고점 %d, 기대 %d", bestScore, scorePerfect)
	}
}

func TestScoreUnusable(t *testing.T) {
	for name, p := range map[string]payload{
		"연주곡":  {Instrumental: true, Duration: 262},
		"빈 가사": {Duration: 262},
		"공백뿐":  {Synced: "  ", Plain: "\n", Duration: 262},
	} {
		if got := score(p, perthMs); got != unusable {
			t.Errorf("%s 을 %d점 줬다 — 쓰면 안 된다", name, got)
		}
	}
}

// 길이를 모르는 후보를 벌하지 않는다. 모르는 것과 틀린 것은 다르다.
func TestUnknownDurationIsNeutral(t *testing.T) {
	unknown := score(payload{Synced: "[00:01.00] x", Duration: 0}, perthMs)
	if unknown != 100 {
		t.Errorf("길이 모르는 싱크가 %d점 — 100이어야 한다", unknown)
	}
	// 그래도 길이가 확인된 것에는 져야 한다.
	if unknown >= score(perth[2], perthMs) {
		t.Error("길이를 모르는 것이 길이가 맞는 것을 이겼다")
	}
}

// 곡 길이를 모를 때(Music.app 이 안 줄 때)는 시간표만 보고 고른다.
func TestNoWantedDuration(t *testing.T) {
	if score(perth[0], 0) != 100 {
		t.Error("곡 길이를 모르면 길이 점수를 매기면 안 된다")
	}
	if score(perth[3], 0) != 0 {
		t.Error("곡 길이를 모를 때 줄글은 0점이어야 한다")
	}
}
