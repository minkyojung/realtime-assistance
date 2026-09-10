package intent

import (
	"encoding/json"
	"testing"

	"amcli/tui/internal/data"
)

// 스키마의 목록과 data.Contexts 가 어긋나면, 모델이 우리가 모르는 값을
// 돌려주거나 아는 값을 못 쓴다. 한 곳에서만 적는다는 약속을 여기서 지킨다.
func TestContextEnumFollowsData(t *testing.T) {
	got := contextEnum()
	if len(got) != len(data.Contexts) {
		t.Fatalf("개수가 다르다: %d vs %d", len(got), len(data.Contexts))
	}
	for i, c := range data.Contexts {
		if got[i] != string(c) {
			t.Fatalf("%d 번째가 다르다: %q vs %q", i, got[i], c)
		}
	}
}

// strict 모드는 properties 전부가 required 에 있기를 요구한다.
// 하나라도 빠지면 OpenAI 가 400 을 돌려주고, 그것은 선곡이 통째로 실패하는 것이다.
func TestSchemaStrictShape(t *testing.T) {
	req := map[string]bool{}
	for _, r := range schema["required"].([]string) {
		req[r] = true
	}
	props := schema["properties"].(map[string]any)
	for name := range props {
		if !req[name] {
			t.Errorf("%q 가 required 에 없다 — strict 모드가 거부한다", name)
		}
	}
	for name := range req {
		if _, ok := props[name]; !ok {
			t.Errorf("%q 가 required 에만 있고 properties 에 없다", name)
		}
	}
	if schema["additionalProperties"] != false {
		t.Error("additionalProperties 가 false 가 아니다")
	}
}

// 모델이 돌려준 답이 Result 로 그대로 들어와야 한다.
func TestResultDecodesContext(t *testing.T) {
	var r Result
	body := `{"title":"Calm Hour","note":"...","context":"focus","picks":[{"trackId":3,"reason":"0 plays"}]}`
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatal(err)
	}
	if r.Context != "focus" {
		t.Fatalf("자리를 못 읽었다: %q", r.Context)
	}
}
