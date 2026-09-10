package musicapp

import (
	"fmt"
	"strings"

	"amcli/tui/internal/data"
	"amcli/tui/internal/intent"
	"amcli/tui/internal/style"
)

// 로그 상세 — ctrl+o 로 펼쳤을 때 보이는 것.
//
// "무슨 일이 있었나"를 담는다. "AI 가 무엇을 생각했나"는 담을 수 없다 —
// 모델이 추론 과정을 돌려주지 않기 때문이다. 대신 후보가 몇 곡이었고,
// 무엇을 왜 골랐고, 얼마나 걸리고 얼마 썼는지는 전부 우리가 안다.
//
// 곡별 근거가 사는 유일한 자리이기도 하다. 본문에 두면 8곡 중 하나만
// 보이고 나머지 일곱은 어차피 안 보인다.
func queueDetail(res intent.Result) []string {
	l := data.Lib()
	out := make([]string, 0, len(res.Picks)+3)

	total := 0
	for _, p := range res.Picks {
		if t, ok := l.Track(p.TrackID); ok {
			total += t.DurationMs
		}
	}
	out = append(out, fmt.Sprintf("candidates %d → %d tracks · %s",
		res.Candidates, len(res.Picks), style.HumanMinutes(total)))

	for i, p := range res.Picks {
		t, ok := l.Track(p.TrackID)
		if !ok {
			continue
		}
		out = append(out, fmt.Sprintf("%2d. %s|%s", i+1, t.Title, p.Reason))
	}

	u := res.Usage
	out = append(out, fmt.Sprintf("%s · %.1fs · ↑%s ↓%s · $%.4f",
		intent.Model(), res.Elapsed.Seconds(),
		style.Tokens(u.PromptTokens), style.Tokens(u.CompletionTokens), u.CostUsd))
	return out
}

var _ = strings.TrimSpace
