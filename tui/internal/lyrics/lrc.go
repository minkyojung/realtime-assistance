// Package lyrics 는 가사를 가져와 재생 위치에 맞춰 짚어준다.
//
// 애플에서는 못 가져온다. 카탈로그 API 의 가사 엔드포인트는 서드파티
// 개발자 토큰에 403 을 주고, AppleScript 의 `lyrics of current track` 은
// 파일에 직접 박아둔 것만 돌려주므로 스트리밍 곡은 빈 값이다.
// 그래서 LRCLIB 에서 받는다 — spikes/lrclib-coverage 가 그 결정의 근거다.
package lyrics

import (
	"strconv"
	"strings"
	"time"
)

// Line 은 "이 시각부터 이 문장" 이다.
type Line struct {
	At   time.Duration
	Text string
}

// Lyrics — 한 곡의 가사.
//
// 셋 중 하나다. 시각이 붙은 것(Lines), 글자만 있는 것(Plain), 없는 것.
// 실측으로 68% · 23% · 8% 였다(spikes/lrclib-coverage).
type Lyrics struct {
	Lines []Line   // 시각이 붙은 줄. 하이라이트가 되는 것은 이것뿐이다
	Plain []string // 시각이 없는 줄글
}

func (l *Lyrics) Synced() bool { return l != nil && len(l.Lines) > 0 }
func (l *Lyrics) Empty() bool {
	return l == nil || (len(l.Lines) == 0 && len(l.Plain) == 0)
}

// At 은 그 시각에 불리고 있는 줄의 번호를 준다. 아직 첫 줄 전이면 -1 이다.
//
// 이진 탐색인 이유는 이것이 1초에 열 번 불리기 때문이다(호스트 스피너가
// 그 주기로 화면을 다시 그린다).
func (l *Lyrics) At(pos time.Duration) int {
	if !l.Synced() {
		return -1
	}
	lo, hi := 0, len(l.Lines)
	for lo < hi {
		mid := (lo + hi) / 2
		if l.Lines[mid].At <= pos {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo - 1
}

// ParseLRC 는 `[02:16.20] 문장` 을 줄 목록으로 바꾼다.
//
// 한 줄에 시각이 여럿 붙는 형식(같은 후렴을 여러 번 쓰는 것)도 있으므로
// 앞에서부터 시각을 계속 떼어낸다. `[ar:]` 같은 머리표는 시각이 아니라
// 버려진다 — 분:초로 안 읽히면 그렇게 걸러진다.
func ParseLRC(s string) []Line {
	var out []Line
	for _, raw := range strings.Split(s, "\n") {
		rest := strings.TrimRight(raw, "\r")
		var stamps []time.Duration
		for strings.HasPrefix(rest, "[") {
			end := strings.IndexByte(rest, ']')
			if end < 0 {
				break
			}
			if d, ok := parseStamp(rest[1:end]); ok {
				stamps = append(stamps, d)
			}
			rest = rest[end+1:]
		}
		if len(stamps) == 0 {
			continue
		}
		text := strings.TrimSpace(rest)
		for _, at := range stamps {
			out = append(out, Line{At: at, Text: text})
		}
	}
	sortLines(out)
	return out
}

// mm:ss.xx · mm:ss:xx · mm:ss 를 받는다.
func parseStamp(s string) (time.Duration, bool) {
	min, rest, ok := strings.Cut(s, ":")
	if !ok {
		return 0, false
	}
	m, err := strconv.Atoi(strings.TrimSpace(min))
	if err != nil || m < 0 {
		return 0, false
	}
	sec, frac, _ := strings.Cut(strings.NewReplacer(":", ".").Replace(rest), ".")
	sc, err := strconv.Atoi(strings.TrimSpace(sec))
	if err != nil || sc < 0 || sc > 59 {
		return 0, false
	}
	d := time.Duration(m)*time.Minute + time.Duration(sc)*time.Second
	if frac != "" {
		// 두 자리면 1/100초, 세 자리면 1/1000초다.
		if f, err := strconv.Atoi(frac); err == nil {
			switch len(frac) {
			case 2:
				d += time.Duration(f) * 10 * time.Millisecond
			case 3:
				d += time.Duration(f) * time.Millisecond
			}
		}
	}
	return d, true
}

// 시각순으로 세운다. 후렴이 여러 번 붙은 줄 때문에 원본 순서가 뒤섞인다.
func sortLines(l []Line) {
	for i := 1; i < len(l); i++ {
		for j := i; j > 0 && l[j].At < l[j-1].At; j-- {
			l[j], l[j-1] = l[j-1], l[j]
		}
	}
}
