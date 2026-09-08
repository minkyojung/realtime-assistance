package intent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openai/openai-go/v3"
)

// decode 는 구조화 출력 응답에서 JSON 을 꺼낸다.
//
// 거절이나 길이 초과는 예외가 아니라 정상 응답으로 온다.
// 내용을 읽기 전에 finish_reason 과 refusal 을 먼저 본다.
func decode(resp *openai.ChatCompletion, v any) error {
	if len(resp.Choices) == 0 {
		return fmt.Errorf("응답에 결과가 없습니다")
	}
	c := resp.Choices[0]

	if r := strings.TrimSpace(c.Message.Refusal); r != "" {
		return fmt.Errorf("요청이 거절되었습니다: %s", r)
	}
	if c.FinishReason == "length" {
		return fmt.Errorf("응답이 잘렸습니다. 곡 수를 줄여 다시 시도하세요")
	}

	s := strings.TrimSpace(c.Message.Content)
	if s == "" {
		return fmt.Errorf("응답이 비어 있습니다 (finish_reason=%s)", c.FinishReason)
	}
	if err := json.Unmarshal([]byte(s), v); err != nil {
		return fmt.Errorf("응답을 읽을 수 없습니다: %w", err)
	}
	return nil
}
