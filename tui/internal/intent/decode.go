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
		return fmt.Errorf("the response had no choices")
	}
	c := resp.Choices[0]

	if r := strings.TrimSpace(c.Message.Refusal); r != "" {
		return fmt.Errorf("the request was refused: %s", r)
	}
	if c.FinishReason == "length" {
		return fmt.Errorf("the response was cut off — ask for fewer tracks")
	}

	s := strings.TrimSpace(c.Message.Content)
	if s == "" {
		return fmt.Errorf("the model returned nothing (finish_reason=%s)", c.FinishReason)
	}
	if err := json.Unmarshal([]byte(s), v); err != nil {
		return fmt.Errorf("could not read the response: %w", err)
	}
	return nil
}
