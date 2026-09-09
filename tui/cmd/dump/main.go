// Music.app 덤프를 눈으로 확인하는 도구.
//
//	go run ./cmd/dump         # 요약
//	go run ./cmd/dump -raw    # 원시 JSON
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"amcli/tui/internal/data"
	"amcli/tui/internal/music"
)

func main() {
	raw := flag.Bool("raw", false, "print the raw JSON")
	flag.Parse()

	b, err := music.DumpLibrary(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed:", err)
		os.Exit(1)
	}
	if *raw {
		os.Stdout.Write(b)
		return
	}

	l, err := data.FromDump(b)
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not read:", err)
		os.Exit(1)
	}
	fmt.Printf("%d raw bytes · %d tracks (%d in library) · %d playlists\n\n",
		len(b), len(l.Tracks), len(l.Songs()), len(l.Playlists))
	for _, p := range l.Playlists {
		fmt.Printf("  %-20s %3d tracks\n", p.Name, p.TrackCount)
	}
	fmt.Println()
	for i, t := range l.Songs() {
		if i >= 5 {
			break
		}
		fmt.Printf("  %d  %s — %s\n", t.Id, t.Title, t.Artist.Name)
	}
}
