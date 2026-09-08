# Apple Music CLI — TUI

Bubble Tea v2 기반 터미널 클라이언트. 지금은 **화면만** 있다.
서버·Music.app 연동 없이 더미 데이터로 렌더한다.
목적은 산출물 ①(PDF)에 넣을 와이어프레임 캡처를 뽑는 것.

## 실행

```
cd tui
go run .          # 실제 TUI (alt screen)
go run ./cmd/render 96   # 헤드리스 렌더 — TTY 없이 화면을 찍어 본다
go test ./...     # 폭 계산 · 필수 문구 검증
```

`Esc` 또는 `Ctrl+C` 로 종료.

## 구조

```
main.go                    진입점
cmd/render/                헤드리스 렌더 (레이아웃 확인용)
internal/api/types.gen.go  ★ api/openapi.yml 에서 자동 생성 (oapi-codegen)
internal/data/dummy.go     더미 데이터 — 실제 라이브러리에서 뽑은 값
internal/ui/
  model.go     루트 모델 · 헤더 · 상태줄 · 입력창
  layout.go    공통 뼈대 (폭 계산 · 진행 바 · 시간 포맷)
  playing.go   S1 재생 뷰 본문
  theme.go     팔레트
```

## 타입은 손으로 쓰지 않는다

`internal/api/types.gen.go` 는 **API 명세에서 생성된다.**

```
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest \
  -config oapi-codegen.yaml ../api/openapi.yml
```

명세에 없는 필드를 화면이 그리려 하면 **컴파일이 안 된다.**
"UI 데이터와 API 불일치"라는 최대 감점 사유가 타입 시스템으로 막힌다.

## 화면 진행 상황

| | 화면 | 상태 |
|---|---|---|
| S1 | 재생 뷰 | ✅ |
| S2 | 묘비 | — |
| S3 | 라이브러리 진단 | — |
| S4 | 취향 규칙 | — |
| S5 | 인식 결과 | — |
| S6 | 예외 · 오류 | — |

## 알려진 제약

Bubble Tea 는 셀 기반 렌더러라 kitty graphics / iTerm2 inline image 를
쓸 수 없다. 앨범 아트는 색 블록으로 대체했다.
