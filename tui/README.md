# Apple Music CLI — TUI

Bubble Tea v2 기반 터미널 클라이언트.

## 레이아웃 — Apple Music 과 같은 모양

```
┌─────────────────────────────────────┐
│ ▶ 지금 재생 중 ── 진행 바 ── 1:22/4:22 │  ← 화면의 첫 줄
├──────────┬──────────────────────────┤
│ 사이드바  │  목록 (최대 14줄)          │
├──────────┴──────────────────────────┤
│ 선정 근거 (있을 때만)                 │
│ 입력창 (자연어 · / 명령 · 검색)        │
└─────────────────────────────────────┘
```

**선정 근거 줄은 의도 층이 만든 곡에서만 나타난다.**
사람이 목록에서 직접 고른 곡에는 근거가 없으므로 줄 자체가 없다.
빈 줄을 남기지 않는다.

**섹션 제목과 곡 수는 두지 않는다.** 사이드바의 레일이 이미 어디인지 말해주고,
그 자리는 지금 재생 중인 곡에 준다.

**목록은 14줄까지만 보여준다.** 화면을 꽉 채우면 읽을 것이 아니라
스캔할 것이 되어버린다.

**사이드바가 무엇을 고르든 목록 패널 하나가 다 그린다.**
Recently Added · Artists · Albums · Songs · 플레이리스트 · Queue 가 전부
같은 패널이므로 화면이 늘어나지 않는다.

## 실행

```
go run .                  실제 TUI
go run ./cmd/render 96    헤드리스 렌더 (TTY 없이 화면 확인)
go test ./...             폭 계산 · 필수 요소 검증
```

## 조작

**입력창은 늘 활성이다.** 포커스 개념이 없으므로 언제든 바로 칠 수 있다.

| 키 | 동작 |
|---|---|
| 아무 글자 | 입력창에 들어가고 **동시에 목록이 걸러진다** (검색 화면이 따로 없는 이유) |
| `↑` `↓` (`ctrl+p` `ctrl+n`) | 목록 이동 |
| `tab` / `shift+tab` | 사이드바 섹션 이동 |
| `enter` | 입력이 있으면 요청, 없으면 **선택한 곡 재생** |
| `esc` | 입력 비우기 (비어 있으면 종료) |
| `ctrl+c` | 종료 |

`j` `k` `h` `l` 같은 알파벳 단축키는 두지 않는다.
입력창이 늘 활성인 구조에서는 자연어와 반드시 충돌한다.

## 구조

```
main.go                    진입점
cmd/render/                헤드리스 렌더
internal/api/types.gen.go  ★ api/openapi.yml 에서 자동 생성 (oapi-codegen)
internal/data/
  library.json             ★ 실제 Music.app 라이브러리 157곡
  library.go               조회 · 정렬 · 검색
  dummy.go                 의도 층이 만든 재생 큐 (고정값)
internal/ui/
  model.go     루트 모델 · 레이아웃 · 키 처리
  sidebar.go   사이드바
  list.go      목록 패널 (곡 · 아티스트 · 앨범 공용)
  player.go    재생 바 · 선정 근거
  layout.go    폭 계산 · 진행 바 · 시간 포맷
  theme.go     팔레트
```

## 타입은 손으로 쓰지 않는다

`internal/api/types.gen.go` 는 API 명세에서 생성된다.

```
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest \
  -config oapi-codegen.yaml ../api/openapi.yml
```

명세에 없는 필드를 화면이 그리려 하면 컴파일이 안 된다.

## 진행 상황

| | 상태 |
|---|---|
| 레이아웃 뼈대 (사이드바 · 목록 · 재생 바) | ✅ |
| Recently Added · Songs | ✅ |
| Artists · Albums · Playlists | ✅ |
| Queue | ✅ |
| 검색 (입력창 타이핑) | ✅ |
| 재생 제어 실연동 (AppleScript) | — |
| 자연어 의도 → 선곡 (서버) | — |
| 취향 규칙 화면 | — |

## 알려진 제약

Bubble Tea 는 셀 기반 렌더러라 kitty graphics / iTerm2 inline image 를
쓸 수 없다. 앨범 아트는 현재 화면에서 뺐다 (반블록 렌더링으로 넣는 것은 가능).
