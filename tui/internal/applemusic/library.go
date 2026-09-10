package applemusic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"amcli/tui/internal/api"
)

// 카탈로그 곡이 이미 내 라이브러리에 있는지는 **Apple 이 안다.**
//
// 한때 우리가 제목과 아티스트를 소문자로 맞춰 보고 짐작했다. 그 판정은
// 같은 곡의 다른 판을 절대 못 가른다 — 실측한 예가 이것이다:
//
//	✗ Woods | Bon Iver | Blood Bank - EP                       | US38Y0913404
//	✓ Woods | Bon Iver | Blood Bank (10th Anniversary Edition)  | US38Y0913404
//
// 제목도 아티스트도 ISRC 까지 같고 앨범만 다르다. 한 판은 담겨 있고 한 판은
// 아니다. 짐작으로는 닿을 수 없는 구분이고, 틀리면 이미 가진 곡을 또 담으면서
// "새 곡"이라고 말하게 된다 — 이 제품에서 제일 나쁜 실패다.
//
// Apple Music API 의 카탈로그 곡에는 그 답이 관계 하나로 달려 있다.
//
//	Songs.Relationships.library — "Library song for a catalog song if added to library."
//
// include=library 를 붙이면 곡마다 그 관계가 따라오고, 비어 있으면 없는 것이다.

// ErrSignInRequired — 사용자 토큰이 있어야 하는 일을 토큰 없이 시켰다.
//
// 검색은 개발자 토큰만으로 되지만 /me/* 는 안 된다. 이것을 따로 두는 이유는
// 부르는 쪽이 **조용히 옛 방식으로 돌아갈 수 있어야** 하기 때문이다.
var ErrSignInRequired = errors.New("sign-in required")

// 한 번에 물어볼 수 있는 곡 수. Apple 의 상한이 300 이다.
const idsPerRequest = 300

// MarkInLibrary 는 각 곡의 InLibrary 를 Apple 의 답으로 채워 돌려준다.
//
// 원본을 고치지 않고 복사본을 돌려준다 — 부르는 쪽이 실패했을 때 원래
// 목록을 그대로 쓸 수 있어야 한다.
func (c *Client) MarkInLibrary(ctx context.Context, tracks []api.CatalogTrack) ([]api.CatalogTrack, error) {
	if len(tracks) == 0 {
		return nil, nil
	}
	if c.UserToken == "" {
		return nil, ErrSignInRequired
	}

	ids := make([]string, 0, len(tracks))
	for _, t := range tracks {
		if t.AppleMusicId != "" {
			ids = append(ids, t.AppleMusicId)
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}

	held := make(map[string]bool, len(ids))
	for len(ids) > 0 {
		n := min(len(ids), idsPerRequest)
		if err := c.askInLibrary(ctx, ids[:n], held); err != nil {
			return nil, err
		}
		ids = ids[n:]
	}

	out := make([]api.CatalogTrack, len(tracks))
	copy(out, tracks)
	for i := range out {
		// 답에 없는 id 는 건드리지 않는다. 안 왔다는 것은 "없다"가 아니라
		// **모른다**이고, 모르는 것을 아는 것처럼 적으면 화면이 거짓말을 한다.
		yes, ok := held[out[i].AppleMusicId]
		if !ok {
			continue
		}
		v := yes
		out[i].InLibrary = &v
	}
	return out, nil
}

// askInLibrary 는 한 묶음을 물어 held 에 적는다.
func (c *Client) askInLibrary(ctx context.Context, ids []string, held map[string]bool) error {
	body, err := c.do(ctx, http.MethodGet, "/v1/catalog/"+c.storefront()+"/songs", url.Values{
		"ids":     {strings.Join(ids, ",")},
		"include": {"library"},
	})
	if err != nil {
		return err
	}
	var res struct {
		Data []struct {
			ID            string `json:"id"`
			Relationships struct {
				Library struct {
					Data []struct {
						ID string `json:"id"`
					} `json:"data"`
				} `json:"library"`
			} `json:"relationships"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return fmt.Errorf("%w: could not read the response: %v", ErrCatalogUnavailable, err)
	}
	for _, s := range res.Data {
		held[s.ID] = len(s.Relationships.Library.Data) > 0
	}
	return nil
}
