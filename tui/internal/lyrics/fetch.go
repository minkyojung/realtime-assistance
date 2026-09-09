package lyrics

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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

// ErrNotFound — 세 번 물어봤는데 없다.
var ErrNotFound = errors.New("no lyrics")

// Fetch 는 한 곡의 가사를 준다. 캐시가 있으면 그것을 쓴다.
func Fetch(ctx context.Context, artist, title, album string, durationMs int) (*Lyrics, error) {
	key := cacheKey(artist, title, durationMs)
	if l, ok, found := readCache(key); found {
		if !ok {
			return nil, ErrNotFound
		}
		return l, nil
	}
	l, err := lookup(ctx, artist, title, album, durationMs)
	writeCache(key, l)
	if err != nil {
		return nil, err
	}
	return l, nil
}

// lookup — 세 단계로 묻는다. 뒤로 갈수록 느슨하다.
//
// 3단계가 커버리지를 4%p 올렸다. 라이브러리 메타데이터가 지저분한
// 것이지 가사가 없는 것이 아니었다 — spikes/lrclib-coverage 참조.
func lookup(ctx context.Context, artist, title, album string, durationMs int) (*Lyrics, error) {
	// 1. 제목·아티스트·앨범·길이가 전부 맞는 것
	q := url.Values{}
	q.Set("artist_name", artist)
	q.Set("track_name", title)
	q.Set("album_name", album)
	q.Set("duration", fmt.Sprint(durationMs/1000))
	var one payload
	if get(ctx, "https://lrclib.net/api/get?"+q.Encode(), &one) {
		return one.lyrics(), nil
	}
	// 2. 제목 + 아티스트로 검색
	if l := searchBest(ctx, artist, title); l != nil {
		return l, nil
	}
	// 3. 제목을 깎아서 다시. 라이브·리마스터 꼬리와, 아티스트가 통째로
	//    비어 제목 끝에 들어간 경우("제목 - Jack Johnson")를 잡는다.
	if c := Clean(title); c != title || artist == "" {
		if l := searchBest(ctx, artist, c); l != nil {
			return l, nil
		}
		if l := searchBest(ctx, "", c); l != nil {
			return l, nil
		}
	}
	return nil, ErrNotFound
}

func searchBest(ctx context.Context, artist, title string) *Lyrics {
	q := url.Values{}
	q.Set("track_name", title)
	if artist != "" {
		q.Set("artist_name", artist)
	}
	var hits []payload
	if !get(ctx, "https://lrclib.net/api/search?"+q.Encode(), &hits) || len(hits) == 0 {
		return nil
	}
	// 시각이 붙은 것을 우선한다. 그것만이 하이라이트에 쓸 수 있다.
	for _, h := range hits {
		if strings.TrimSpace(h.Synced) != "" {
			return h.lyrics()
		}
	}
	return hits[0].lyrics()
}

type payload struct {
	Synced       string `json:"syncedLyrics"`
	Plain        string `json:"plainLyrics"`
	Instrumental bool   `json:"instrumental"`
}

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

func get(ctx context.Context, u string, into any) bool {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", agent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	return json.NewDecoder(resp.Body).Decode(into) == nil
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

// readCache — (가사, 있음, 캐시에 기록이 있음).
func readCache(key string) (*Lyrics, bool, bool) {
	dir := cacheDir()
	if dir == "" {
		return nil, false, false
	}
	b, err := os.ReadFile(filepath.Join(dir, key+".json"))
	if err != nil {
		return nil, false, false
	}
	if len(b) == 0 {
		return nil, false, true // 없다고 확인된 곡
	}
	var l Lyrics
	if json.Unmarshal(b, &l) != nil {
		return nil, false, false
	}
	return &l, true, true
}

func writeCache(key string, l *Lyrics) {
	dir := cacheDir()
	if dir == "" {
		return
	}
	if os.MkdirAll(dir, 0o755) != nil {
		return
	}
	var b []byte
	if l != nil {
		b, _ = json.Marshal(l)
	}
	os.WriteFile(filepath.Join(dir, key+".json"), b, 0o644)
}
