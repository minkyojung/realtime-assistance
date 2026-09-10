package style

// 터미널은 폭이 예측 불가하고 폰트 메트릭이 없다. 그래서 "자를지 말지"를
// 우리가 정해야 하고, **자르는 것은 곧 우선순위 선언**이다.
//
// 모든 TUI 레이아웃은 결국 칸마다 숫자 세 개를 정하는 일로 환원된다.
// 앱이 그 셋을 적어두면 이 계산기가 그대로 실행한다.
//
// 음악만의 문제가 아니다. 채팅 앱도 좁아지면 시각과 채널명 중
// 무엇을 버릴지 정해야 한다.

// Col 은 한 칸이 좁아질 때 어떻게 행동할지에 대한 선언이다.
type Col struct {
	// Min 은 이 칸이 살아남기 위한 최소 폭이다.
	Min int

	// Weight 는 남는 공간을 가져가는 비율이다. 0 이면 Min 에 고정된다.
	// 남는 공간이 아무 의미 없는 곳(칸 사이 여백)으로 가지 않게 하는 장치다.
	Weight int

	// Max 는 이 칸이 커질 수 있는 한계다. 0 이면 제한이 없다.
	//
	// 아주 넓은 창에서는 어떤 칸도 더 필요하지 않다. 상한이 없으면
	// 남는 공간이 한 칸에 다 몰려 가운데가 텅 빈다.
	Max int

	// Drop 은 희생 순서다. 클수록 먼저 사라진다. 0 이면 절대 버리지 않는다.
	Drop int
}

// Columns 는 주어진 폭에 칸들을 앉힌다.
// 돌려주는 폭이 0 이면 그 칸은 이번에 그리지 않는다는 뜻이다.
//
// gap 은 칸 사이 간격이다. 살아남은 칸 사이에만 들어간다.
func Columns(width, gap int, cols []Col) []int {
	out := make([]int, len(cols))
	alive := make([]bool, len(cols))
	for i := range cols {
		alive[i] = true
	}

	// 최소폭 합이 들어갈 때까지 Drop 이 큰 것부터 버린다.
	for {
		need, n := 0, 0
		for i, c := range cols {
			if alive[i] {
				need += c.Min
				n++
			}
		}
		if n <= 1 || need+gap*(n-1) <= width {
			break
		}
		// 살아 있는 것 중 Drop 이 가장 큰 하나를 버린다. 0 은 버리지 않는다.
		victim, worst := -1, 0
		for i, c := range cols {
			if alive[i] && c.Drop > worst {
				victim, worst = i, c.Drop
			}
		}
		if victim < 0 {
			break // 전부 필수 칸이다. 더 버릴 수 없으니 그대로 둔다
		}
		alive[victim] = false
	}

	n, need, weight := 0, 0, 0
	for i, c := range cols {
		if !alive[i] {
			continue
		}
		n++
		need += c.Min
		weight += c.Weight
		out[i] = c.Min
	}
	if n == 0 {
		return out
	}

	// 남는 공간은 가중치대로 나눈다. 가중치가 없으면 남겨 둔다.
	slack := width - need - gap*(n-1)
	if slack <= 0 || weight == 0 {
		return out
	}
	// 상한에 걸린 칸이 생기면 남은 몫을 다시 나눈다.
	// 한 번에 끝나지 않으므로 더 나눌 곳이 없을 때까지 돈다.
	for slack > 0 {
		weight = 0
		for i, c := range cols {
			if alive[i] && c.Weight > 0 && (c.Max == 0 || out[i] < c.Max) {
				weight += c.Weight
			}
		}
		if weight == 0 {
			break
		}

		moved := 0
		for i, c := range cols {
			if !alive[i] || c.Weight == 0 || (c.Max > 0 && out[i] >= c.Max) {
				continue
			}
			add := slack * c.Weight / weight
			if add == 0 {
				add = 1 // 나눗셈이 0 이 되면 영원히 안 끝난다
			}
			if c.Max > 0 && out[i]+add > c.Max {
				add = c.Max - out[i]
			}
			if add > slack-moved {
				add = slack - moved
			}
			out[i] += add
			moved += add
			if moved >= slack {
				break
			}
		}
		if moved == 0 {
			break
		}
		slack -= moved
	}
	return out
}
