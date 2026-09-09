// LRCLIB 커버리지 스파이크.
//
// 가사를 화면에 붙이기 전에 물어야 할 것은 하나다 —
// **내 라이브러리의 몇 퍼센트에 가사가 있는가.**
//
// 30% 밖에 없으면 화면이 대부분 비어서 없느니만 못하고,
// 70% 넘으면 만들 값어치가 있다. 그 숫자를 재는 것이 전부다.
//
//	cd spikes/lrclib-coverage && go run . [-limit N]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

// 라이브러리 캐시에서 필요한 것만 읽는다. tui 의 타입을 가져오지 않는 이유는
// 스파이크가 본체를 컴파일하지 않고 혼자 돌아야 하기 때문이다.
type library struct {
	Tracks []struct {
		Title      string `json:"title"`
		DurationMs int    `json:"durationMs"`
		Artist     struct {
			Name string `json:"name"`
		} `json:"artist"`
		Album struct {
			Title string `json:"title"`
		} `json:"album"`
	} `json:"tracks"`
}

// LRCLIB 의 응답. 우리가 쓰는 것은 두 필드뿐이다.
type lrc struct {
	SyncedLyrics string `json:"syncedLyrics"`
	PlainLyrics  string `json:"plainLyrics"`
	Instrumental bool   `json:"instrumental"`
}

type result struct {
	title, artist string
	hangul        bool
	synced        bool // 타임스탬프가 붙은 가사 — 하이라이트가 되는 것
	plain         bool // 글자만 있는 가사 — 하이라이트가 안 된다
	instrumental  bool
	viaSearch     bool // 정확 매칭은 실패하고 검색으로 찾은 것
	viaClean      bool // 제목을 깎아서야 찾은 것
}

const agent = "amcli-lyrics-spike/0.1 (coverage check)"

func main() {
	limit := flag.Int("limit", 0, "앞에서 N곡만 (0 이면 전부)")
	flag.Parse()

	lib, err := loadLibrary()
	if err != nil {
		fmt.Fprintln(os.Stderr, "라이브러리를 읽을 수 없습니다:", err)
		fmt.Fprintln(os.Stderr, "먼저 tui 를 한 번 띄워 캐시를 만들어야 합니다.")
		os.Exit(1)
	}
	tracks := lib.Tracks
	if *limit > 0 && *limit < len(tracks) {
		tracks = tracks[:*limit]
	}
	fmt.Printf("%d곡 조회 중…\n\n", len(tracks))

	// 남의 무료 서버다. 네 갈래까지만 동시에 두드린다.
	results := make([]result, len(tracks))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	var done int
	var mu sync.Mutex

	start := time.Now()
	for i, t := range tracks {
		wg.Add(1)
		go func(i int, title, artist, album string, ms int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			r := result{title: title, artist: artist, hangul: hasHangul(title + artist)}
			if l, ok := get(artist, title, album, ms); ok {
				fill(&r, l)
			} else if l, ok := search(artist, title); ok {
				r.viaSearch = true
				fill(&r, l)
			} else if c := clean(title); c != title || artist == "" {
				// 라이브러리의 제목에는 라이브·리마스터 꼬리가 붙어 있고,
				// 아티스트가 통째로 비어 제목 끝에 들어간 곡도 있다.
				// 깎아서 다시 물어본다.
				if l, ok := search(artist, c); ok {
					r.viaClean = true
					fill(&r, l)
				} else if l, ok := search("", c); ok {
					r.viaClean = true
					fill(&r, l)
				}
			}
			results[i] = r

			mu.Lock()
			done++
			if done%25 == 0 {
				fmt.Printf("  %d/%d\n", done, len(tracks))
			}
			mu.Unlock()
		}(i, t.Title, t.Artist.Name, t.Album.Title, t.DurationMs)
	}
	wg.Wait()

	report(results, time.Since(start))
}

func fill(r *result, l lrc) {
	r.instrumental = l.Instrumental
	r.synced = strings.TrimSpace(l.SyncedLyrics) != ""
	r.plain = !r.synced && strings.TrimSpace(l.PlainLyrics) != ""
}

// get — 제목·아티스트·앨범·길이가 모두 맞는 것을 찾는다. 가장 정확하다.
func get(artist, title, album string, ms int) (lrc, bool) {
	q := url.Values{}
	q.Set("artist_name", artist)
	q.Set("track_name", title)
	q.Set("album_name", album)
	q.Set("duration", fmt.Sprint(ms/1000))
	var l lrc
	if !fetch("https://lrclib.net/api/get?"+q.Encode(), &l) {
		return lrc{}, false
	}
	return l, true
}

// search — 정확 매칭이 실패했을 때. 제목과 아티스트만으로 첫 결과를 본다.
//
// 라이브러리의 앨범명이 리마스터·디럭스판이라 어긋나는 경우가 많은데,
// 그때도 가사는 같은 곡이다.
func search(artist, title string) (lrc, bool) {
	q := url.Values{}
	q.Set("track_name", title)
	q.Set("artist_name", artist)
	var hits []lrc
	if !fetch("https://lrclib.net/api/search?"+q.Encode(), &hits) || len(hits) == 0 {
		return lrc{}, false
	}
	// 타임스탬프가 있는 것을 우선한다. 그것만이 하이라이트에 쓸 수 있다.
	for _, h := range hits {
		if strings.TrimSpace(h.SyncedLyrics) != "" {
			return h, true
		}
	}
	return hits[0], true
}

func fetch(u string, into any) bool {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", agent)
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return false
	}
	return json.NewDecoder(resp.Body).Decode(into) == nil
}

// clean — 제목에서 가사와 무관한 꼬리를 떼어낸다.
//
// "(Live at Knebworth)" · "(feat. X)" · "- Jack Johnson"(아티스트가 제목에
// 들어가 버린 경우). 라이브러리 메타데이터가 지저분한 것이지 가사가
// 없는 것이 아니다.
var (
	reParen = regexp.MustCompile(`(?i)\s*[\(\[](live|feat\.?|remaster|acoustic|deluxe|version|from |explicit|mono|stereo)[^)\]]*[)\]]`)
	reTail  = regexp.MustCompile(`\s+-\s+.*$`)
)

func clean(title string) string {
	t := reParen.ReplaceAllString(title, "")
	t = reTail.ReplaceAllString(t, "")
	return strings.TrimSpace(t)
}

func hasHangul(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Hangul, r) {
			return true
		}
	}
	return false
}

func loadLibrary() (*library, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(filepath.Join(dir, "amcli", "library.json"))
	if err != nil {
		return nil, err
	}
	var l library
	return &l, json.Unmarshal(b, &l)
}

func report(rs []result, took time.Duration) {
	var synced, plain, none, inst, viaSearch int
	var hanTotal, hanSynced int
	var missing []string
	for _, r := range rs {
		if r.hangul {
			hanTotal++
			if r.synced {
				hanSynced++
			}
		}
		switch {
		case r.instrumental:
			inst++
		case r.synced:
			synced++
		case r.plain:
			plain++
		default:
			none++
			missing = append(missing, r.artist+" — "+r.title)
		}
		if (r.viaSearch || r.viaClean) && (r.synced || r.plain) {
			viaSearch++
		}
	}
	n := len(rs)
	fmt.Printf("\n─── %d곡 · %.0f초 ───\n\n", n, took.Seconds())
	line := func(label string, v int) {
		fmt.Printf("  %-28s %4d   %5.1f%%\n", label, v, 100*float64(v)/float64(max(n, 1)))
	}
	line("싱크 가사 (하이라이트 가능)", synced)
	line("글자만 (하이라이트 불가)", plain)
	line("연주곡", inst)
	line("없음", none)
	fmt.Println()
	line("└ 검색으로 겨우 찾은 것", viaSearch)
	if hanTotal > 0 {
		fmt.Printf("\n  한글 곡 %d개 중 싱크 가사 %d개 (%.0f%%)\n",
			hanTotal, hanSynced, 100*float64(hanSynced)/float64(hanTotal))
	}

	sort.Strings(missing)
	fmt.Printf("\n─── 가사가 없는 곡 (%d) ───\n", len(missing))
	for i, m := range missing {
		if i == 20 {
			fmt.Printf("  … 그리고 %d곡 더\n", len(missing)-20)
			break
		}
		fmt.Println("  " + m)
	}

	fmt.Printf("\n판정: ")
	switch pct := 100 * float64(synced) / float64(max(n, 1)); {
	case pct >= 70:
		fmt.Printf("%.0f%% — 만든다\n", pct)
	case pct >= 40:
		fmt.Printf("%.0f%% — 애매하다. 없을 때 화면을 먼저 정해야 한다\n", pct)
	default:
		fmt.Printf("%.0f%% — 없느니만 못하다\n", pct)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
