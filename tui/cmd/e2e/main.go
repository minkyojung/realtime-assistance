// 엔드투엔드 — 실제 라이브러리·실제 모델·실제 카탈로그로 한 바퀴 돈다.
//
// 마지막 재생 쓰기(cmdWriteQueue)는 일부러 안 돌린다. 확인하려는 것은
// "고르고 담고 앉히는 것"까지이고, 재생은 이 커밋이 건드리지 않은 옛 길이다.
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"amcli/tui/internal/data"
	"amcli/tui/internal/host"
	"amcli/tui/internal/music"
	"amcli/tui/internal/musicapp"
	tea "charm.land/bubbletea/v2"
)

// pump 은 Cmd 트리를 돌리며 나오는 메시지를 모델에 먹인다.
//
// depth 로 막는 이유: 폴링 틱이 자기를 다시 낳는다(tickMsg -> tick()).
// 끝까지 따라가면 영원히 안 끝난다 — 실제 앱에서는 그것이 옳은 모양이고,
// 하네스에서만 잘라야 한다.
//
// stop 이 참인 메시지를 만나면 **먹이지 않고** 돌려준다. 그래야 그 다음을
// 하네스가 직접 고를 수 있다.
func pump(m tea.Model, cmd tea.Cmd, depth int, stop func(tea.Msg) bool) (tea.Model, tea.Msg) {
	if cmd == nil || depth <= 0 {
		return m, nil
	}
	msg := cmd()
	if b, ok := msg.(tea.BatchMsg); ok {
		for _, c := range b {
			mm, found := pump(m, c, depth-1, stop)
			m = mm
			if found != nil {
				return m, found
			}
		}
		return m, nil
	}
	if msg == nil {
		return m, nil
	}
	if stop != nil && stop(msg) {
		return m, msg
	}
	mm, next := m.Update(msg)
	return pump(mm, next, depth-1, stop)
}

func isQueueMsg(msg tea.Msg) bool { return fmt.Sprintf("%T", msg) == "musicapp.queueMsg" }

func plain(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func main() {
	prompt := "내가 한 번도 안 들어본 새로운 곡 위주로 틀어줘"
	if len(os.Args) > 1 {
		prompt = strings.Join(os.Args[1:], " ")
	}

	before, err := music.LibraryIDs()
	if err != nil {
		fmt.Println("Music.app 을 못 읽는다:", err)
		os.Exit(1)
	}
	fmt.Printf("담기 전 라이브러리: %d곡\n", len(before))

	hm := host.New(musicapp.New())
	var m tea.Model = &hm
	m, _ = m.Update(tea.WindowSizeMsg{Width: 110, Height: 34})
	m, _ = pump(m, m.Init(), 3, nil)
	fmt.Printf("라이브러리 스냅샷: %d곡\n요청: %q\n\n", len(data.Lib().Tracks), prompt)

	// 첫 화면은 관문이다. 한 번 들어가야 입력창이 산다.
	m, gate := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m, _ = pump(m, gate, 2, nil)

	for _, r := range prompt {
		m, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	start := time.Now()
	fmt.Println("① 라우터가 도구를 고르고 → 선곡이 돈다…")
	m, qmsg := pump(m, cmd, 12, isQueueMsg)
	if qmsg == nil {
		fmt.Println("   선곡이 안 왔다. 화면:")
		fmt.Println(plain(m.View().Content))
		return
	}
	fmt.Printf("   선곡 도착 (%.1fs)\n\n", time.Since(start).Seconds())

	fmt.Println("② 라이브러리 밖에서 고른 곡이 있으면 담는다…")
	if !musicapp.QueueNeedsAdding(qmsg) {
		// 담을 것이 없으면 다음 Cmd 는 곧바로 재생 쓰기다. **실행하지 않는다.**
		// 물어보고 나서 멈춰야 한다 — 실행해서 알아내면 이미 소리가 난 뒤다.
		fmt.Println("   밖에서 고른 곡 없음 — 라이브러리 안에서만 골랐다")
		m, _ = m.Update(qmsg)
	} else {
		m2, cmd2 := m.Update(qmsg)
		m2, resolved := pump(m2, cmd2, 8, isQueueMsg)
		if resolved == nil {
			fmt.Println("   담기 단계가 답을 안 돌려줬다")
			m = m2
		} else {
			fmt.Printf("   담기·대기 끝 (%.1fs)\n", time.Since(start).Seconds())
			// 여기서 돌려받는 Cmd 도 실행하지 않는다 — 그것이 재생 쓰기다.
			m, _ = m2.Update(resolved)
		}
	}

	fmt.Println("\n──────── 결과 화면 ────────")
	fmt.Println(plain(m.View().Content))

	after, err := music.LibraryIDs()
	if err != nil {
		fmt.Println("\n뒷정리 확인 실패:", err)
		return
	}
	var added []string
	for id := range after {
		if !before[id] {
			added = append(added, id)
		}
	}
	fmt.Printf("\n──────── 라이브러리 변화 ────────\n담긴 곡: %d개  (%d → %d)\n", len(added), len(before), len(after))
	for _, id := range added {
		fmt.Println("   +", id)
	}
}
