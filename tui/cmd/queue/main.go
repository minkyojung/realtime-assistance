// 의도 층을 TUI 없이 확인하는 도구.
//
//   go run ./cmd/queue "1시간 코딩용 큐"

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"amcli/tui/internal/data"
	"amcli/tui/internal/intent"
)

func main() {
	prompt := "앞으로 1시간 동안 코딩하면서 들을 큐 만들어줘"
	if len(os.Args) > 1 {
		prompt = strings.Join(os.Args[1:], " ")
	}
	l := data.Lib()

	fmt.Printf("요청: %s\n라이브러리: %d곡\n\n", prompt, len(l.Tracks))
	start := time.Now()
	res, err := intent.Build(context.Background(), prompt, l.Tracks, intent.Current{}, time.Now())
	if err != nil {
		fmt.Println("실패:", err)
		os.Exit(1)
	}

	total := 0
	fmt.Printf("── %s ──\n%s\n\n", res.Title, res.Note)
	for i, p := range res.Picks {
		t, ok := l.Track(p.TrackID)
		if !ok {
			fmt.Printf("%2d. [없는 id %d]\n", i+1, p.TrackID)
			continue
		}
		total += t.DurationMs
		last := "never"
		if t.LastPlayedAt != nil {
			last = fmt.Sprintf("%dd", int(time.Since(*t.LastPlayedAt).Hours()/24))
		}
		fmt.Printf("%2d. %-34s %-18s %3dp %-6s  %s\n",
			i+1, trunc(t.Title, 33), trunc(t.Artist.Name, 17), t.PlayCount, last, p.Reason)
	}
	fmt.Printf("\n%d곡 · %d분 · %.1fs · ↑%d ↓%d · $%.4f\n",
		len(res.Picks), total/60000, time.Since(start).Seconds(),
		res.Usage.PromptTokens, res.Usage.CompletionTokens, res.Usage.CostUsd)
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
