package music

import (
	"os"
	"testing"
)

// 살았는지는 프로세스를 새로 띄우지 않고 본다.
func TestAliveSeesThisProcess(t *testing.T) {
	if !alive(os.Getpid()) {
		t.Fatal("자기 자신이 죽었다고 한다")
	}
}

// 죽은 PID 는 즉시 죽었다고 한다. 여기서 틀리면 Music.app 을 끈 직후
// 폴링이 스크립트를 보내고, 그 스크립트가 Music.app 을 도로 켠다.
func TestAliveSeesADeadPID(t *testing.T) {
	// PID 최댓값 근처는 사실상 비어 있다. 우연히 살아 있으면 건너뛴다.
	const far = 99999999
	if alive(far) {
		t.Skip("이 PID 가 살아 있다")
	}
}

// 기억해 둔 PID 가 죽으면 다시 찾는다. 기억만 믿고 끝내면 안 된다.
func TestRunningRefindsAfterTheProcessDies(t *testing.T) {
	musicPID.Lock()
	old := musicPID.pid
	musicPID.pid = 99999999 // 죽은 것으로 기억해 둔다
	musicPID.Unlock()
	t.Cleanup(func() { musicPID.Lock(); musicPID.pid = old; musicPID.Unlock() })

	got := Running()
	musicPID.Lock()
	now := musicPID.pid
	musicPID.Unlock()
	// Music.app 이 떠 있든 아니든, 죽은 PID 는 버려져야 한다.
	if now == 99999999 {
		t.Fatal("죽은 PID 를 그대로 믿는다")
	}
	if got != (now > 0) {
		t.Errorf("답(%v)과 기억한 PID(%d)가 어긋난다", got, now)
	}
}
