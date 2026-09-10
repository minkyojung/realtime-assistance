package applemusic

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"amcli/tui/internal/api"
)

// stub 은 망을 타지 않고 대신 답한다. 요청을 붙잡아 두어 무엇을 물었는지 본다.
type stub struct {
	got  *http.Request
	body string
	code int
}

func (s *stub) RoundTrip(r *http.Request) (*http.Response, error) {
	s.got = r
	code := s.code
	if code == 0 {
		code = http.StatusOK
	}
	return &http.Response{
		StatusCode: code,
		Body:       io.NopCloser(strings.NewReader(s.body)),
		Header:     make(http.Header),
	}, nil
}

func clientWith(s *stub, userToken string) *Client {
	return &Client{
		DevToken:   "dev",
		UserToken:  userToken,
		Storefront: "kr",
		HTTP:       &http.Client{Transport: s},
	}
}

func track(id, title string) api.CatalogTrack {
	return api.CatalogTrack{AppleMusicId: id, Title: title, ArtistName: "Bon Iver"}
}

// 같은 제목·같은 아티스트·같은 ISRC 인데 한 판만 담겨 있다.
// 이름 비교로는 닿을 수 없는 구분이고, 그것이 이 함수가 있는 이유다.
func TestMarkInLibrarySplitsTwoPressingsOfTheSameRecording(t *testing.T) {
	s := &stub{body: `{"data":[
		{"id":"1","relationships":{"library":{"data":[]}}},
		{"id":"2","relationships":{"library":{"data":[{"id":"i.5PkLoYEfOKNmpR6"}]}}}
	]}`}
	c := clientWith(s, "user")

	out, err := c.MarkInLibrary(context.Background(), []api.CatalogTrack{
		track("1", "Woods"), // Blood Bank - EP
		track("2", "Woods"), // Blood Bank (10th Anniversary Edition)
	})
	if err != nil {
		t.Fatalf("MarkInLibrary: %v", err)
	}
	if out[0].InLibrary == nil || *out[0].InLibrary {
		t.Errorf("첫 판 InLibrary = %v, want false", out[0].InLibrary)
	}
	if out[1].InLibrary == nil || !*out[1].InLibrary {
		t.Errorf("둘째 판 InLibrary = %v, want true", out[1].InLibrary)
	}
}

// 무엇을 물었는지. include=library 가 빠지면 관계가 안 오고,
// 그러면 전부 "없음"으로 읽혀 조용히 틀린다.
func TestMarkInLibraryAsksAppleForTheLibraryRelationship(t *testing.T) {
	s := &stub{body: `{"data":[]}`}
	c := clientWith(s, "user")

	if _, err := c.MarkInLibrary(context.Background(), []api.CatalogTrack{track("1", "a"), track("2", "b")}); err != nil {
		t.Fatalf("MarkInLibrary: %v", err)
	}
	q := s.got.URL.Query()
	if got := s.got.URL.Path; got != "/v1/catalog/kr/songs" {
		t.Errorf("path = %q", got)
	}
	if got := q.Get("include"); got != "library" {
		t.Errorf("include = %q, want library", got)
	}
	if got := q.Get("ids"); got != "1,2" {
		t.Errorf("ids = %q, want 1,2", got)
	}
	if got := s.got.Header.Get("Music-User-Token"); got != "user" {
		t.Errorf("Music-User-Token = %q — 이게 없으면 /me 관계가 안 온다", got)
	}
}

// 답에 없는 id 는 "없음"이 아니라 **모름**이다. 모르는 것을 false 로 적으면
// 화면이 `+` 를 띄우고, 사용자는 이미 가진 곡을 또 담는다.
func TestMarkInLibraryLeavesUnansweredTracksAlone(t *testing.T) {
	s := &stub{body: `{"data":[{"id":"1","relationships":{"library":{"data":[]}}}]}`}
	c := clientWith(s, "user")

	out, err := c.MarkInLibrary(context.Background(), []api.CatalogTrack{track("1", "a"), track("2", "b")})
	if err != nil {
		t.Fatalf("MarkInLibrary: %v", err)
	}
	if out[1].InLibrary != nil {
		t.Errorf("답에 없던 곡의 InLibrary = %v, want nil", *out[1].InLibrary)
	}
}

// 로그인 전에는 이 길이 없다. 부르는 쪽이 옛 방식으로 돌아갈 수 있도록
// 다른 실패와 구별되는 오류여야 한다.
func TestMarkInLibraryNeedsAUserToken(t *testing.T) {
	s := &stub{body: `{"data":[]}`}
	c := clientWith(s, "")

	_, err := c.MarkInLibrary(context.Background(), []api.CatalogTrack{track("1", "a")})
	if err != ErrSignInRequired {
		t.Fatalf("err = %v, want ErrSignInRequired", err)
	}
	if s.got != nil {
		t.Error("토큰도 없이 망을 탔다")
	}
}

// 300 이 Apple 의 상한이다. 넘으면 400 이 오므로 나눠 묻는다.
func TestMarkInLibrarySplitsBeyondAppleLimit(t *testing.T) {
	calls := 0
	s := &stub{body: `{"data":[]}`}
	c := clientWith(s, "user")
	c.HTTP = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if n := len(strings.Split(r.URL.Query().Get("ids"), ",")); n > idsPerRequest {
			t.Errorf("한 번에 %d개를 물었다, 상한 %d", n, idsPerRequest)
		}
		return s.RoundTrip(r)
	})}

	many := make([]api.CatalogTrack, idsPerRequest+1)
	for i := range many {
		many[i] = track(string(rune('a'+i%26))+string(rune('a'+i/26)), "t")
	}
	if _, err := c.MarkInLibrary(context.Background(), many); err != nil {
		t.Fatalf("MarkInLibrary: %v", err)
	}
	if calls != 2 {
		t.Errorf("호출 %d회, want 2", calls)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
