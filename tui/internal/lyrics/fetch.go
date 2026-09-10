package lyrics

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// LRCLIB 에서 받는다. 키가 없고 무료다.
//
// 남의 서버이므로 이름을 밝히고, 한 곡을 두 번 묻지 않는다 — 받은 것은
// 파일로 남긴다. 없다는 답도 남긴다. 없는 곡을 곡 바뀔 때마다 다시
// 물어보는 것이 제일 미안한 짓이다.

const (
	agent   = "amcli/0.1 (Apple Music CLI; https://github.com/minkyojung)"
	timeout = 12 * time.Second
)

// 서버 주소. 테스트가 갈아끼운다.
var baseURL = "https://lrclib.net"

var (
	// ErrNotFound — 사다리를 다 돌았는데 없다. 이건 캐시에 남긴다.
	ErrNotFound = errors.New("no lyrics")
	// ErrUnreachable — 물어보지 못했다. **없다는 뜻이 아니므로 캐시에 남기지 않는다.**
	ErrUnreachable = errors.New("lrclib unreachable")
)

// Fetch 는 한 곡의 가사를 준다. 캐시가 있으면 그것을 쓴다.
func Fetch(ctx context.Context, artist, title, album string, durationMs int) (*Lyrics, error) {
	key := cacheKey(artist, title, durationMs)
	if l, found := readCache(key); found {
		if l.Empty() {
			return nil, ErrNotFound
		}
		return l, nil
	}

	l, err := lookup(ctx, artist, title, album, durationMs)
	// 못 물어본 것을 "없다"로 못 박지 않는다. 와이파이가 끊긴 동안 지나간
	// 곡이 영영 가사 없는 곡이 되어 버린다.
	if errors.Is(err, ErrUnreachable) {
		return nil, err
	}
	writeCache(key, l)
	if err != nil {
		return nil, err
	}
	return l, nil
}

// lookup — 후보를 모아 점수로 고른다.
//
// 처음에는 "찾으면 곧장 반환"이었다. 성공 조건을 **가사를 찾았나** 로
// 잡았기 때문인데, 실제로 원하는 것은 **이 녹음에 맞는 시간표를 찾았나**
// 다. 그 둘을 같은 것으로 본 탓에 시간표 없는 답에서 멈추거나(하이라이트
// 안 됨) 딴 녹음의 시간표를 집었다(박자 안 맞음).
//
// 그래서 사다리를 끝까지 두되, **만점이면 즉시 멈춘다.** 잘 되는 곡은
// 1단에서 끝나므로 요청 횟수가 예전과 같다. 지금 실패하는 곡만 더 묻는다.
//
// 실측 — 라이브러리 202곡에서 싱크 가사 68% → 89%.
func lookup(ctx context.Context, artist, title, album string, durationMs int) (*Lyrics, error) {
	clean := Clean(title)

	// 사다리. 위에서 아래로 좁은 것에서 넓은 것으로 간다.
	// 한 칸이라도 못 물어봤으면 "없다"고 단정하지 않는다.
	reachable := true

	ladder := []func() []payload{
		// 1. 제목·아티스트·앨범·길이가 전부 맞는 것. 제일 정확하다.
		func() []payload {
			q := url.Values{}
			q.Set("artist_name", artist)
			q.Set("track_name", title)
			q.Set("album_name", album)
			q.Set("duration", fmt.Sprint(durationMs/1000))
			var one payload
			switch get(ctx, baseURL+"/api/get?"+q.Encode(), &one) {
			case failed:
				reachable = false
				return nil
			case absent:
				return nil
			}
			// /api/get 은 길이로 걸러 준 답이다. duration 을 안 실어 보내는
			// 경우가 있어 여기서 채운다 — 안 그러면 점수에서 손해를 본다.
			if one.Duration == 0 {
				one.Duration = float64(durationMs) / 1000
			}
			return []payload{one}
		},
		// 2. 앨범명이 어긋난 곡. 리마스터·디럭스판이 흔하다.
		func() []payload {
			return search(ctx, url.Values{"track_name": {title}, "artist_name": {artist}}, &reachable)
		},
		// 3. 제목의 꼬리를 깎는다. "(Live at …)" "(feat. …)"
		func() []payload {
			if clean == title {
				return nil
			}
			return search(ctx, url.Values{"track_name": {clean}, "artist_name": {artist}}, &reachable)
		},
		// 4. 아티스트 칸이 통째로 빈 곡. 제목 끝에 들어가 있는 경우다.
		func() []payload { return search(ctx, url.Values{"track_name": {clean}}, &reachable) },
		// 5. 통째 검색. 표기가 미묘하게 달라 위의 것들이 다 빗나갈 때 걸린다.
		func() []payload { return search(ctx, url.Values{"q": {clean + " " + artist}}, &reachable) },
	}

	var best *payload
	bestScore := unusable
	for _, ask := range ladder {
		for _, c := range ask() {
			s := score(c, durationMs)
			if s <= bestScore {
				continue
			}
			hit := c
			best, bestScore = &hit, s
			if s >= scorePerfect {
				return best.lyrics(), nil // 더 볼 것이 없다
			}
		}
	}
	if best == nil {
		if !reachable {
			return nil, ErrUnreachable
		}
		return nil, ErrNotFound
	}
	return best.lyrics(), nil
}

// search — 후보를 그대로 돌려준다. 고르는 것은 부르는 쪽의 일이다.
func search(ctx context.Context, q url.Values, reachable *bool) []payload {
	if q.Get("track_name") == "" && q.Get("q") == "" {
		return nil
	}
	if q.Get("artist_name") == "" {
		q.Del("artist_name") // 빈 값을 실어 보내면 아무것도 안 나온다
	}
	var hits []payload
	if get(ctx, baseURL+"/api/search?"+q.Encode(), &hits) == failed {
		*reachable = false
		return nil
	}
	return hits
}

type payload struct {
	Synced       string  `json:"syncedLyrics"`
	Plain        string  `json:"plainLyrics"`
	Instrumental bool    `json:"instrumental"`
	Duration     float64 `json:"duration"` // 초. 같은 녹음인지의 유일한 증거다
}

// 점수 — 이 후보가 **이 녹음의** 가사일 가능성.
//
// 검색은 제목이 같은 것을 전부 준다. `Perth` 를 물으면 스무 개가 오는데
// 그중에는 길이가 2452초인 것도 있다 — 앨범을 통째로 한 항목에 올린
// 것이다. 시간표가 붙어 있으므로 "시각이 있는 첫 번째"를 집으면 그게
// 걸리고, 가사가 노래와 40분 어긋난다.
//
// **틀린 시간표는 시간표가 없는 것보다 나쁘다.** 없으면 화면이 조용하지만
// 틀리면 노래와 따로 논다. 그래서 길이가 크게 어긋난 것은 시간표 점수를
// 통째로 상쇄한다 — 길이가 맞는 줄글이 길이가 틀린 싱크를 이긴다.
func score(p payload, wantMs int) int {
	if p.Instrumental {
		return unusable
	}
	synced := strings.TrimSpace(p.Synced) != ""
	if !synced && strings.TrimSpace(p.Plain) == "" {
		return unusable
	}

	s := 0
	if synced {
		s += 100 // 하이라이트가 되는 것은 이것뿐이다
	}
	// 길이를 모르면 더하지도 빼지도 않는다. 모르는 것을 벌하지 않는다.
	if wantMs > 0 && p.Duration > 0 {
		switch d := math.Abs(p.Duration - float64(wantMs)/1000); {
		case d <= 2:
			s += 50 // 같은 녹음이다
		case d <= 5:
			s += 20 // 같은 곡의 다른 마스터쯤
		case d > 30:
			s -= 100 // 딴 것이다. 시간표가 있어도 믿을 수 없다
		}
	}
	return s
}

const (
	// 시간표가 있고 길이도 맞는다. 더 물어볼 것이 없으므로 사다리를 멈춘다.
	scorePerfect = 150
	// 쓸 수 없는 후보. 연주곡이거나 가사가 비어 있다.
	unusable = -1
)

func (p payload) lyrics() *Lyrics {
	if p.Instrumental {
		return &Lyrics{} // 연주곡. 없는 것과 구별해 둘 이유가 아직 없다
	}
	l := &Lyrics{Lines: ParseLRC(p.Synced)}
	if len(l.Lines) == 0 {
		for _, s := range strings.Split(strings.TrimSpace(p.Plain), "\n") {
			l.Plain = append(l.Plain, strings.TrimSpace(s))
		}
		if len(l.Plain) == 1 && l.Plain[0] == "" {
			l.Plain = nil
		}
	}
	return l
}

// 물어본 결과. **없다** 와 **못 물어봤다** 를 구별해야 한다.
// 그 둘을 같이 취급하면 와이파이가 끊긴 동안 지나간 곡이 영영
// "가사 없음"으로 캐시에 박힌다.
type outcome int

const (
	answered outcome = iota // 답을 받아 읽었다
	absent                  // 서버가 없다고 했다
	failed                  // 못 물어봤다 — 연결·시간초과·깨진 응답
)

func get(ctx context.Context, u string, into any) outcome {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return failed
	}
	req.Header.Set("User-Agent", agent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return failed
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return absent
	case resp.StatusCode != http.StatusOK:
		return failed // 5xx·429. 서버 사정이지 가사가 없는 것이 아니다
	}
	if json.NewDecoder(resp.Body).Decode(into) != nil {
		return failed
	}
	return answered
}

// Clean 은 제목에서 가사와 무관한 꼬리를 떼어낸다.
var (
	reParen = regexp.MustCompile(`(?i)\s*[(\[](live|feat\.?|remaster|acoustic|deluxe|version|from |explicit|mono|stereo)[^)\]]*[)\]]`)
	reTail  = regexp.MustCompile(`\s+-\s+.*$`)
)

func Clean(title string) string {
	return strings.TrimSpace(reTail.ReplaceAllString(reParen.ReplaceAllString(title, ""), ""))
}

// ── 캐시 ────────────────────────────────────────────────────────────
//
// 곡 하나에 파일 하나. 없다는 답도 빈 파일로 남긴다.

func cacheKey(artist, title string, durationMs int) string {
	sum := sha1.Sum([]byte(strings.ToLower(artist + "\x00" + title + "\x00" + fmt.Sprint(durationMs/1000))))
	return hex.EncodeToString(sum[:])
}

func cacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "amcli", "lyrics")
}

// 캐시 파일의 모양.
//
// **v 를 두는 이유**는 고르는 규칙이 바뀌면 예전에 받아 둔 답도 낡은 것이
// 되기 때문이다. 실제로 그런 일이 있었다 — 시간표 없는 답에서 멈추던 시절의
// 캐시가 코드를 고친 뒤에도 그대로 쓰여서, 화면이 안 바뀌었다.
// 번호를 올리면 손으로 지울 필요 없이 다시 받는다.
//
// lines·plain 이 둘 다 비어 있으면 **없다고 확인된 곡**이다.
type cacheFile struct {
	V     int      `json:"v"`
	Lines []Line   `json:"lines,omitempty"`
	Plain []string `json:"plain,omitempty"`
}

// 2 — 후보에 점수를 매기기 시작한 판(fetch.go score).
const cacheVersion = 2

// readCache — (가사, 기록이 있는가).
func readCache(key string) (*Lyrics, bool) {
	dir := cacheDir()
	if dir == "" {
		return nil, false
	}
	b, err := os.ReadFile(filepath.Join(dir, key+".json"))
	if err != nil {
		return nil, false
	}
	var f cacheFile
	if json.Unmarshal(b, &f) != nil || f.V != cacheVersion {
		return nil, false // 낡았거나 깨졌다. 다시 받는다
	}
	return &Lyrics{Lines: f.Lines, Plain: f.Plain}, true
}

func writeCache(key string, l *Lyrics) {
	dir := cacheDir()
	if dir == "" {
		return
	}
	if os.MkdirAll(dir, 0o755) != nil {
		return
	}
	f := cacheFile{V: cacheVersion}
	if l != nil {
		f.Lines, f.Plain = l.Lines, l.Plain
	}
	b, err := json.Marshal(f)
	if err != nil {
		return
	}
	os.WriteFile(filepath.Join(dir, key+".json"), b, 0o644)
}
