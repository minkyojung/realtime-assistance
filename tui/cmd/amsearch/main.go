// amsearch 는 Apple Music 카탈로그 연결을 눈으로 확인하는 도구다.
//
//	go run ./cmd/amsearch login      — 브라우저를 열어 사용자 토큰을 받는다
//	go run ./cmd/amsearch <검색어>   — 카탈로그 전곡에서 찾는다
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"amcli/tui/internal/applemusic"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "쓰임: amsearch login | amsearch <검색어>")
		os.Exit(2)
	}

	cfg, err := applemusic.LoadConfig()
	die(err)
	c, err := applemusic.NewClient(cfg)
	die(err)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	if os.Args[1] == "login" {
		fmt.Println("브라우저를 엽니다...")
		die(c.Login(ctx))
		fmt.Println("연결됐습니다. 지역:", c.Storefront)
		return
	}

	term := strings.Join(os.Args[1:], " ")
	tracks, err := c.Search(ctx, term, 25)
	die(err)

	fmt.Printf("%q — %d곡\n\n", term, len(tracks))
	for i, t := range tracks {
		album := ""
		if t.AlbumName != nil {
			album = "  · " + *t.AlbumName
		}
		fmt.Printf("%2d. %s — %s%s\n", i+1, t.Title, t.ArtistName, album)
	}
}

func die(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "오류:", err)
		os.Exit(1)
	}
}
