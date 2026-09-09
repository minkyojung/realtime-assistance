package intent

import (
	"strings"
	"testing"

	"amcli/tui/internal/api"
)

// 자리번호를 곡 id 로 되돌리는 자리다. 여기서 틀리면 엉뚱한 곡이 사라진다.
//
// 스키마는 "정수 배열"까지만 막아 준다. 범위와 중복은 여기서 막는다.
func TestPositionsBecomeTrackIDs(t *testing.T) {
	cur := queueOf(11, 22, 33)

	for _, c := range []struct {
		name string
		in   []int
		want []int64
	}{
		{"자리번호 그대로", []int{1, 3}, []int64{11, 33}},
		{"순서를 지킨다", []int{3, 1}, []int64{33, 11}},
		{"범위 밖은 버린다", []int{0, 4, 2, -1}, []int64{22}},
		{"같은 자리를 두 번 짚으면 한 번만", []int{2, 2}, []int64{22}},
		{"빈 목록", nil, []int64{}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := idsAt(cur, c.in)
			if len(got) != len(c.want) {
				t.Fatalf("%v → %v, 원한 것 %v", c.in, got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("%v → %v, 원한 것 %v", c.in, got, c.want)
					break
				}
			}
		})
	}
}

// 모델이 보는 큐. 1부터 세고, 지금 나오는 곡에 표가 있어야 한다.
func TestRenderedQueueIsNumberedFromOne(t *testing.T) {
	cur := queueOf(11, 22, 33)
	cur.Playing = 22

	got := renderQueue(cur)
	for _, want := range []string{"  1 | track 11", "▶ 2 | track 22", "  3 | track 33"} {
		if !strings.Contains(got, want) {
			t.Errorf("큐에 %q 가 없다:\n%s", want, got)
		}
	}
}

// 고칠 큐가 없으면 물어볼 것도 없다. 모델을 부르지 않는다.
func TestTriageSkipsWhenThereIsNoQueue(t *testing.T) {
	e, err := Triage(nil, "조용한 거", Current{})
	if err != nil {
		t.Fatalf("빈 큐인데 실패했다: %v", err)
	}
	if e.Kind != EditNew {
		t.Error("큐가 없는데 고치라고 판단했다")
	}
}

func queueOf(ids ...int64) Current {
	items := make([]api.QueueItem, 0, len(ids))
	for _, id := range ids {
		items = append(items, api.QueueItem{Track: api.Track{
			Id:     id,
			Title:  "track " + itoa(id),
			Artist: api.Artist{Name: "someone"},
		}})
	}
	return Current{Items: items}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
