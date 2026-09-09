// amtoken 은 p8 로 개발자 토큰을 서명해 stdout 에 한 줄로 뱉는다.
//
// 배포 빌드가 이 값을 바이너리에 박는다. p8 은 여기까지만 오고
// 배포물에는 서명된 토큰만 들어간다.
//
//	go run ./cmd/amtoken            여섯 달짜리
//	go run ./cmd/amtoken -days 90   더 짧게
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"amcli/tui/internal/applemusic"
)

func main() {
	days := flag.Int("days", 180, "유효기간 (최대 180)")
	flag.Parse()

	cfg, err := applemusic.LoadConfig()
	die(err)
	p8, err := os.ReadFile(cfg.P8Path)
	die(err)
	tok, err := applemusic.DeveloperToken(p8, cfg.KeyID, cfg.TeamID,
		time.Duration(*days)*24*time.Hour)
	die(err)

	fmt.Println(tok)
}

func die(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "amtoken:", err)
		os.Exit(1)
	}
}
