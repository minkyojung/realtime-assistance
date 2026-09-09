package applemusic

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// Music User Token 은 OAuth 리다이렉트로 받을 수 없다.
// Apple 은 이 토큰을 MusicKit(웹/네이티브)을 통해서만 내준다.
//
// 그래서 CLI 가 쓰는 방법은 하나다 — **자기 자신이 잠깐 웹서버가 되는 것**.
// localhost 에 임의 포트를 열고, MusicKit JS 를 띄운 페이지를 스스로 서빙하고,
// 그 페이지가 받아낸 토큰을 자기에게 POST 하게 한 뒤, 서버를 닫는다.
// gh·slack 의 로그인이 하는 일과 구조가 같다. 다른 것은 콜백이
// OAuth 리다이렉트가 아니라 우리가 심은 fetch 라는 점뿐이다.

// Authorize 는 브라우저를 열어 Music User Token 을 받아온다.
func Authorize(ctx context.Context, devToken string) (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer ln.Close()

	// 같은 기계의 다른 프로세스가 아무 토큰이나 POST 해 넣지 못하게 막는다.
	// 로컬이라도 열린 포트는 열린 포트다.
	nonce, err := randomNonce()
	if err != nil {
		return "", err
	}

	got := make(chan string, 1)
	fail := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		authPage.Execute(w, map[string]string{"Token": devToken, "Nonce": nonce})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("X-Nonce") != nonce {
			http.Error(w, "no", http.StatusForbidden)
			return
		}
		b, _ := io.ReadAll(io.LimitReader(r.Body, 4096))
		tok := strings.TrimSpace(string(b))
		w.Write([]byte("ok"))
		if tok == "" {
			fail <- errors.New("got an empty token back")
			return
		}
		got <- tok
	})

	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	defer srv.Shutdown(context.Background())

	url := fmt.Sprintf("http://%s/", ln.Addr().String())
	if err := exec.Command("open", url).Run(); err != nil {
		// 브라우저가 안 열려도 사람이 직접 열 수 있어야 한다.
		return "", fmt.Errorf("could not open a browser. Open this yourself: %s (%w)", url, err)
	}

	select {
	case tok := <-got:
		return tok, nil
	case err := <-fail:
		return "", err
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(3 * time.Minute):
		return "", errors.New("sign-in did not finish within 3 minutes")
	}
}

func randomNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// authorize() 는 팝업을 띄운다. 페이지가 열리자마자 부르면 브라우저가
// 팝업으로 막는다. 그래서 반드시 **사용자 클릭 안에서** 부른다.
var authPage = template.Must(template.New("auth").Parse(`<!doctype html>
<meta charset="utf-8">
<title>Connect Apple Music</title>
<style>
  body { font: 15px -apple-system, sans-serif; display: grid; place-content: center;
         height: 100vh; margin: 0; gap: 1rem; text-align: center; }
  button { font: inherit; padding: .6rem 1.4rem; border-radius: 8px;
           border: 0; background: #fa243c; color: #fff; cursor: pointer; }
  #msg { color: #666; min-height: 1.2em; }
</style>
<h2>Connect Apple Music</h2>
<button id="go">Sign in to Apple Music</button>
<p id="msg">Your terminal is waiting.</p>
<script src="https://js-cdn.music.apple.com/musickit/v3/musickit.js" data-web-components async></script>
<script>
  const msg = document.getElementById('msg');
  const btn = document.getElementById('go');
  let ready = false;

  document.addEventListener('musickitloaded', async () => {
    try {
      await MusicKit.configure({
        developerToken: {{.Token}},
        app: { name: 'Apple Music CLI', build: '1' },
      });
      ready = true;
    } catch (e) {
      msg.textContent = 'Developer token rejected: ' + e;
    }
  });

  btn.onclick = async () => {
    if (!ready) { msg.textContent = 'Still loading MusicKit…'; return; }
    btn.disabled = true;
    try {
      const token = await MusicKit.getInstance().authorize();
      await fetch('/token', { method: 'POST', headers: { 'X-Nonce': {{.Nonce}} }, body: token });
      msg.textContent = 'Connected. You can return to your terminal.';
      btn.remove();
    } catch (e) {
      btn.disabled = false;
      msg.textContent = 'Cancelled or failed: ' + e;
    }
  };
</script>`))
