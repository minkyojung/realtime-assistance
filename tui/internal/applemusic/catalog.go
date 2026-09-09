package applemusic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"amcli/tui/internal/api"
)

// E7 의 두 얼굴. 개발자 토큰이 죽은 것과 사용자 토큰이 죽은 것은
// 사용자가 할 일이 다르다 — 앞은 우리 잘못이고, 뒤는 다시 로그인하면 된다.
var (
	// ErrCatalogUnavailable — E7. 스펙의 CATALOG_UNAVAILABLE 과 같은 뜻이다.
	ErrCatalogUnavailable = errors.New("catalog unavailable")
	// ErrUserTokenRejected — 사용자 토큰이 만료되거나 취소됐다.
	ErrUserTokenRejected = errors.New("user token rejected")
)

// Client 는 api.music.apple.com 에 붙는다.
//
// UserToken 은 비어 있어도 된다. 카탈로그 검색은 개발자 토큰만으로 되고,
// 사용자 토큰은 /v1/me/* — 내 라이브러리를 읽고 쓸 때만 필요하다.
type Client struct {
	DevToken   string
	UserToken  string
	Storefront string // 비면 "kr"
	HTTP       *http.Client
}

func (c *Client) do(ctx context.Context, method, path string, q url.Values) ([]byte, error) {
	u := "https://api.music.apple.com" + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.DevToken)
	if c.UserToken != "" {
		req.Header.Set("Music-User-Token", c.UserToken)
	}

	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	res, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCatalogUnavailable, err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	switch {
	case res.StatusCode == http.StatusUnauthorized:
		// 배포판 사용자는 key ID 도 team ID 도 갖고 있지 않다. 그들이 할 수
		// 있는 일은 업데이트뿐이므로 그렇게 말해야 한다.
		if EmbeddedToken != "" {
			return nil, errors.New("this build's Apple Music token has expired — update yarrr")
		}
		return nil, errors.New("developer token rejected (401) — check key ID, team ID and expiry")
	case res.StatusCode == http.StatusForbidden:
		return nil, fmt.Errorf("%w: 403", ErrUserTokenRejected)
	case res.StatusCode >= 300:
		return nil, fmt.Errorf("%w: %s: %s", ErrCatalogUnavailable, res.Status, trim(body))
	}
	return body, nil
}

func (c *Client) storefront() string {
	if c.Storefront == "" {
		return "kr"
	}
	return c.Storefront
}

// Search 는 카탈로그 전곡에서 찾는다. 라이브러리 밖의 곡이 여기 있다.
func (c *Client) Search(ctx context.Context, term string, limit int) ([]api.CatalogTrack, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 25 {
		limit = 25 // Apple 이 한 번에 주는 최대치
	}
	body, err := c.do(ctx, http.MethodGet, "/v1/catalog/"+c.storefront()+"/search", url.Values{
		"term":  {term},
		"types": {"songs"},
		"limit": {fmt.Sprint(limit)},
	})
	if err != nil {
		return nil, err
	}

	var res struct {
		Results struct {
			Songs struct {
				Data []struct {
					ID         string `json:"id"`
					Attributes struct {
						Name        string   `json:"name"`
						ArtistName  string   `json:"artistName"`
						AlbumName   string   `json:"albumName"`
						GenreNames  []string `json:"genreNames"`
						Isrc        string   `json:"isrc"`
						ReleaseDate string   `json:"releaseDate"`
						DurationMs  int      `json:"durationInMillis"`
						Artwork     struct {
							URL string `json:"url"`
						} `json:"artwork"`
					} `json:"attributes"`
				} `json:"data"`
			} `json:"songs"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("%w: could not read the response: %v", ErrCatalogUnavailable, err)
	}

	out := make([]api.CatalogTrack, 0, len(res.Results.Songs.Data))
	for _, s := range res.Results.Songs.Data {
		a := s.Attributes
		t := api.CatalogTrack{
			AppleMusicId: s.ID,
			Title:        a.Name,
			ArtistName:   a.ArtistName,
		}
		if a.AlbumName != "" {
			t.AlbumName = &a.AlbumName
		}
		if len(a.GenreNames) > 0 {
			t.Genre = &a.GenreNames[0]
		}
		if a.Isrc != "" {
			t.Isrc = &a.Isrc
		}
		if y := year(a.ReleaseDate); y > 0 {
			t.Year = &y
		}
		if a.Artwork.URL != "" {
			// {w}x{h} 자리표시자를 실제 크기로 바꿔야 쓸 수 있는 주소가 된다.
			u := strings.NewReplacer("{w}", "300", "{h}", "300").Replace(a.Artwork.URL)
			t.ArtworkUrl = &u
		}
		out = append(out, t)
	}
	return out, nil
}

// AddToLibrary 는 카탈로그 곡을 내 라이브러리에 담는다. 사용자 토큰이 필요하다.
//
// 담아야 재생할 수 있다. Apple Music API 는 재생을 시키지 못하고,
// 우리 재생 경로(music.PlayPersistentID)는 Music.app 안의 곡만 튼다.
// 담긴 곡이 Music.app 에 나타나기까지는 동기화 지연이 있다.
func (c *Client) AddToLibrary(ctx context.Context, appleMusicIDs []string) error {
	if c.UserToken == "" {
		return errors.New("sign-in required")
	}
	if len(appleMusicIDs) == 0 {
		return nil
	}
	_, err := c.do(ctx, http.MethodPost, "/v1/me/library", url.Values{
		"ids[songs]": {strings.Join(appleMusicIDs, ",")},
	})
	return err
}

// Storefront 는 이 계정의 지역을 읽는다. 사용자 토큰이 있을 때만 된다.
func (c *Client) LookupStorefront(ctx context.Context) (string, error) {
	if c.UserToken == "" {
		return "", errors.New("sign-in required")
	}
	body, err := c.do(ctx, http.MethodGet, "/v1/me/storefront", nil)
	if err != nil {
		return "", err
	}
	var res struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &res); err != nil || len(res.Data) == 0 {
		return "", errors.New("could not read the storefront")
	}
	return res.Data[0].ID, nil
}

func year(s string) int {
	if len(s) < 4 {
		return 0
	}
	n := 0
	for _, r := range s[:4] {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func trim(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		return s[:200]
	}
	return s
}
