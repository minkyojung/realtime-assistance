package music

import (
	"context"
	"errors"
)

// Music.app 으로 가는 통로는 **하나**다.
//
// Music.app 은 Apple Event 를 한 줄로 처리한다. 우리가 여러 개를 동시에
// 보내도 저쪽에서 줄을 서는데, 그 줄이 길어지면 Music.app 자기 버튼까지
// 안 먹는다. 실측: 동시에 40개를 보내면 `next track` 이 0.13초에서 1.47초가
// 됐다.
//
// 그리고 이 줄은 스스로 길어진다. 1초 폴링이 앞 요청이 끝났는지 안 보고
// 계속 쏘면, Music.app 이 잠깐만 느려져도(방금 담은 곡을 받아오는 순간)
// 요청이 겹겹이 쌓이고 → Music.app 이 더 느려지고 → 더 쌓인다. 한 번
// 시작되면 굳는다.
//
// 그래서 줄을 저쪽이 아니라 **이쪽에서** 세운다. Apple 의 프레임워크가
// 대상 앱 하나에 이벤트를 직렬로 보내는 것과 같은 자리다.
//
//   - 폴링(Status)은 통로가 막혀 있으면 이번 것을 **버린다.** 1초 뒤에
//     또 물을 것이고, 그 사이 화면은 위치 보간으로 매끄럽다.
//   - 명령(재생·큐 쓰기·덤프)은 통로가 빌 때까지 **기다린다.** 사람이
//     시킨 일은 버릴 수 없다. 기다리는 시간은 폴링 하나(보통 0.1초)다.
var lane = make(chan struct{}, 1)

// ErrBusy — 통로가 차 있어서 이번 폴링을 건너뛰었다. 실패가 아니다.
var ErrBusy = errors.New("music app is busy")

// hold 는 통로가 빌 때까지 기다린 뒤 잡는다. ctx 가 먼저 끝나면 포기한다.
func hold(ctx context.Context) (release func(), err error) {
	select {
	case lane <- struct{}{}:
		return func() { <-lane }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// tryHold 는 지금 비어 있을 때만 잡는다. 기다리지 않는다.
func tryHold() (release func(), ok bool) {
	select {
	case lane <- struct{}{}:
		return func() { <-lane }, true
	default:
		return nil, false
	}
}
