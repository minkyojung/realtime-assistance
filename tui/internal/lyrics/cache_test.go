package lyrics

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// 서버를 하나 세우고 요청 수를 센다. 캐시가 도는지 아닌지는 그것으로 안다.
func serve(t *testing.T, h http.HandlerFunc) *int32 {
	t.Helper()
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	old := baseURL
	baseURL = srv.URL
	t.Cleanup(func() { baseURL = old })
	t.Setenv("HOME", t.TempDir())
	return &n
}

// 서버가 없다고 하면 그것은 캐시한다. 두 번째는 묻지 않는다.
func TestAbsentIsCached(t *testing.T) {
	n := serve(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	ctx := context.Background()
	if _, err := Fetch(ctx, "A", "B", "C", 200_000); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, 기대 ErrNotFound", err)
	}
	first := atomic.LoadInt32(n)
	if first == 0 {
		t.Fatal("아무것도 안 물어봤다")
	}
	if _, err := Fetch(ctx, "A", "B", "C", 200_000); !errors.Is(err, ErrNotFound) {
		t.Fatalf("두 번째 err = %v", err)
	}
	if got := atomic.LoadInt32(n); got != first {
		t.Errorf("없다고 확인된 곡을 %d번 더 물어봤다", got-first)
	}
}

// 못 물어본 것은 캐시하지 않는다. 와이파이가 돌아오면 다시 찾아야 한다.
func TestUnreachableIsNotCached(t *testing.T) {
	var down atomic.Bool
	down.Store(true)
	n := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if down.Load() {
			w.WriteHeader(http.StatusBadGateway) // 서버 사정. 가사가 없는 게 아니다
			return
		}
		json.NewEncoder(w).Encode([]payload{
			{Synced: "[00:01.00] hi", Duration: 200},
		})
	})
	ctx := context.Background()

	if _, err := Fetch(ctx, "A", "B", "C", 200_000); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("err = %v, 기대 ErrUnreachable", err)
	}
	before := atomic.LoadInt32(n)

	down.Store(false)
	got, err := Fetch(ctx, "A", "B", "C", 200_000)
	if err != nil {
		t.Fatalf("서버가 돌아왔는데 err = %v", err)
	}
	if !got.Synced() {
		t.Error("서버가 돌아왔는데 가사를 못 받았다")
	}
	if atomic.LoadInt32(n) <= before {
		t.Error("못 물어본 곡을 다시 안 물어봤다 — 캐시에 박혔다")
	}
}

// 규칙이 바뀌면 예전에 받아 둔 답도 낡은 것이다. 번호가 다르면 다시 받는다.
func TestOldCacheIsIgnored(t *testing.T) {
	n := serve(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]payload{
			{Synced: "[00:01.00] new", Duration: 200},
		})
	})
	// 옛 판으로 저장된 파일을 심는다.
	dir := cacheDir()
	os.MkdirAll(dir, 0o755)
	b, _ := json.Marshal(cacheFile{V: cacheVersion - 1, Plain: []string{"old"}})
	os.WriteFile(filepath.Join(dir, cacheKey("A", "B", 200_000)+".json"), b, 0o644)

	got, err := Fetch(context.Background(), "A", "B", "C", 200_000)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Synced() {
		t.Error("낡은 캐시를 그대로 썼다 — 코드를 고쳐도 화면이 안 바뀐다")
	}
	if atomic.LoadInt32(n) == 0 {
		t.Error("낡은 캐시인데 다시 안 물어봤다")
	}
}

// 받은 것은 다시 묻지 않는다.
func TestFoundIsCached(t *testing.T) {
	n := serve(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]payload{
			{Synced: "[00:01.00] hi", Duration: 200},
		})
	})
	ctx := context.Background()
	if _, err := Fetch(ctx, "A", "B", "C", 200_000); err != nil {
		t.Fatal(err)
	}
	first := atomic.LoadInt32(n)
	got, err := Fetch(ctx, "A", "B", "C", 200_000)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Synced() {
		t.Error("캐시에서 읽은 것에 시간표가 없다")
	}
	if atomic.LoadInt32(n) != first {
		t.Error("이미 받은 곡을 또 물어봤다")
	}
}

// 만점을 찾으면 사다리를 멈춘다. 잘 되는 곡은 요청 한 번이다.
func TestPerfectStopsEarly(t *testing.T) {
	n := serve(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(payload{Synced: "[00:01.00] hi", Duration: 200})
	})
	if _, err := Fetch(context.Background(), "A", "B", "C", 200_000); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(n); got != 1 {
		t.Errorf("요청 %d번 — 1단에서 만점이면 한 번이어야 한다", got)
	}
}
