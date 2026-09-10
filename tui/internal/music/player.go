package music

import "context"

// Player 는 Music.app 을 하나의 장치로 본 것이다.
//
// 이 패키지는 한때 Music.app 에 말을 거는 함수 스물 몇 개의 모음이었다.
// 그 함수들을 부르는 쪽(musicapp)은 Music.app 이 실제로 떠 있어야만
// 돌았고, 그래서 재생·큐 경로는 테스트가 없었다 — 있는 테스트는 스크립트
// 문자열을 조립하는 부분까지였다.
//
// 장치 하나로 묶으면 두 가지가 생긴다. 가짜 장치를 꽂아 musicapp 을
// Music.app 없이 끝까지 돌릴 수 있고, 구현을 바꿀 수 있다 — 지금은
// 호출마다 osascript 를 띄우지만(AppleScript), 상주 프로세스 하나로 바꾸는
// 날이 온다. 부르는 쪽은 그날 아무것도 모른다.
//
// 메서드 이름과 시그니처는 예전 함수 그대로다. 이 커밋은 경계를 세우는
// 것이지 동작을 바꾸는 것이 아니다.
type Player interface {
	Status() (PlayerState, error)

	PlayPause() error
	Next() error
	Previous() error
	PlayPersistentID(id string) error
	PlayByTitleArtist(title, artist string) error

	CreatePlaylist(name string, persistentIDs []string) error
	ReplaceQueue(prevPID string, persistentIDs []string) (string, error)
	PlayQueueAt(pid string, n int, positionSec int) error
	RemoveQueueTrack(pid string, n int, advance bool) error
	RewriteQueueTail(pid string, from int, persistentIDs []string) error

	LibraryCount() (int, error)
	LibraryIDs() (map[string]bool, error)
	DumpLibrary(ctx context.Context) ([]byte, error)
	Artwork(ctx context.Context) ([]byte, error)

	SetShuffle(on bool) error
	SetRepeat(r Repeat) error
	SetVolume(n int) error
	SetFavorite(on bool) error
	SetRating(stars int) error
}

// AppleScript 는 osascript 로 Music.app 에 말을 거는 Player 다. 지금의 구현이다.
type AppleScript struct{}

var _ Player = AppleScript{}

func (AppleScript) Status() (PlayerState, error)     { return Status() }
func (AppleScript) PlayPause() error                 { return PlayPause() }
func (AppleScript) Next() error                      { return Next() }
func (AppleScript) Previous() error                  { return Previous() }
func (AppleScript) PlayPersistentID(id string) error { return PlayPersistentID(id) }
func (AppleScript) PlayByTitleArtist(title, artist string) error {
	return PlayByTitleArtist(title, artist)
}
func (AppleScript) CreatePlaylist(name string, ids []string) error { return CreatePlaylist(name, ids) }
func (AppleScript) ReplaceQueue(prev string, ids []string) (string, error) {
	return ReplaceQueue(prev, ids)
}
func (AppleScript) PlayQueueAt(pid string, n, pos int) error { return PlayQueueAt(pid, n, pos) }
func (AppleScript) RemoveQueueTrack(pid string, n int, adv bool) error {
	return RemoveQueueTrack(pid, n, adv)
}
func (AppleScript) RewriteQueueTail(pid string, from int, ids []string) error {
	return RewriteQueueTail(pid, from, ids)
}
func (AppleScript) LibraryCount() (int, error)                      { return LibraryCount() }
func (AppleScript) LibraryIDs() (map[string]bool, error)            { return LibraryIDs() }
func (AppleScript) DumpLibrary(ctx context.Context) ([]byte, error) { return DumpLibrary(ctx) }
func (AppleScript) Artwork(ctx context.Context) ([]byte, error)     { return Artwork(ctx) }
func (AppleScript) SetShuffle(on bool) error                        { return SetShuffle(on) }
func (AppleScript) SetRepeat(r Repeat) error                        { return SetRepeat(r) }
func (AppleScript) SetVolume(n int) error                           { return SetVolume(n) }
func (AppleScript) SetFavorite(on bool) error                       { return SetFavorite(on) }
func (AppleScript) SetRating(stars int) error                       { return SetRating(stars) }
