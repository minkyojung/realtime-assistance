package intent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

// decode 는 구조화 출력 응답에서 JSON 을 꺼낸다.
//
// 거절(refusal)은 예외가 아니라 200 으로 온다. 내용을 읽기 전에
// stop_reason 을 먼저 본다.
func decode(resp *anthropic.Message, v any) error {
	if resp.StopReason == anthropic.StopReasonRefusal {
		return fmt.Errorf("요청이 거절되었습니다")
	}

	var b strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			b.WriteString(t.Text)
		}
	}
	s := strings.TrimSpace(b.String())
	if s == "" {
		return fmt.Errorf("응답이 비어 있습니다 (stop_reason=%s)", resp.StopReason)
	}
	if err := json.Unmarshal([]byte(s), v); err != nil {
		return fmt.Errorf("응답을 읽을 수 없습니다: %w", err)
	}
	return nil
}
