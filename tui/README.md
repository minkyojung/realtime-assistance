# Apple Music CLI — TUI

Bubble Tea v2 기반 터미널 클라이언트.

## 레이아웃 — Apple Music 과 같은 모양

```
┌──────────┬──────────────────────────┐
│ 사이드바  │  목록                     │
├──────────┴──────────────────────────┤
│ 선정 근거 (있을 때만)                 │
│ 입력창 (자연어 · / 명령 · 검색)        │
├─────────────────────────────────────┤
│ 재생 바 (항상 보임)                   │
└─────────────────────────────────────┘
```

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

| 키 | 동작 |
|---|---|
| `↑` `↓` / `j` `k` | 목록 이동 |
| `←` `→` / `h` `l` | 사이드바 ↔ 목록 |
| `tab` | 사이드바 → 목록 → 입력창 |
| 아무 글자 | 곧바로 입력창으로. **치는 동안 목록이 걸러진다** (검색 화면이 따로 없는 이유) |
| `space` | 재생 / 일시정지 (목록 포커스일 때) |
| `esc` | 입력 취소 · 종료 |
| `ctrl+c` | 종료 |

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
