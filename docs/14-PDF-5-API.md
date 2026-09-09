# §5. API 명세 정의

> PDF 5장. 배점 20점.
> 전체 명세는 `판교_3반_{이름}_AppleMusic-API.yml` (OAS 3.0.3) 에 있고, 여기에는 요약을 싣는다.
> **Security Schemes 는 과제 명세에 따라 정의하지 않는다.**

---

## 5-1. 구조

| 항목 | 수 |
|---|---|
| 경로 | 28 |
| 오퍼레이션 | 34 |
| 공통 스키마 | 40 |
| 공통 응답 (에러) | 6 |
| 공통 파라미터 | 9 |
| **`$ref` 재사용** | **151회** |

**서버는 로컬 데몬이다.** `http://127.0.0.1:8787/v1` 에 바인딩되며 이 기기 밖으로
나가지 않는다. Music.app 을 만지는 것은 서버뿐이고, 화면은 이 API 만 부른다.
그래서 AppleScript 권한 · Apple Music 토큰 · AI 키가 전부 서버 한 곳에 모인다.

## 5-2. 설계 원칙 4개

**① 자연어 요청은 비동기다.**
`POST /turns` 는 `202` 를 즉시 돌려주고, 화면은 `GET /turns/{turnId}` 로
진행을 본다. 선곡은 초 단위이므로 기다리게 하면 입력창이 그동안 죽는다.

**② 실시간 값은 저장하지 않는다.**
재생 위치 · 섞기 · 반복은 서버가 Music.app 에 그때그때 물어 돌려준다.
`GET /playback` 의 응답 필드 중 DB 컬럼에서 오는 것은 곡 정보뿐이다.

**③ 관문은 로그인이 아니라 권한이다. 그래서 `401` 이 없다.**
대신 `403 AUTOMATION_PERMISSION_DENIED` 와 `503 MUSIC_APP_NOT_RUNNING` 이 있다.

**④ AI 실패는 HTTP 에러가 아니다.**
요청은 이미 `202` 로 받았으므로, 실패는 `turn.errorCode` 로 온다.
선정 근거를 못 만들어도 `reason` 이 `null` 일 뿐 큐는 그대로 돈다.

## 5-3. 엔드포인트 전체

### status

| 엔드포인트 | 하는 일 | 응답 |
|---|---|---|
| `GET /status` | 무엇에 붙어 있고 무엇을 갖고 있는가 | 200 |
| `GET /settings` | 설정 조회 | 200 |
| `PATCH /settings` | 설정 변경 | 200 · 400 |

### library

| 엔드포인트 | 하는 일 | 응답 |
|---|---|---|
| `GET /library/sync` | 동기화 상태 | 200 |
| `POST /library/sync` | Music.app 을 다시 읽는다 | 202 · 403 · 409 · 503 |
| `GET /tracks` | 곡 목록 | 200 · 400 |
| `GET /tracks/{trackId}` | 곡 하나 | 200 · 404 |
| `GET /artists` | 아티스트 묶음 목록 | 200 · 400 |
| `GET /artists/{artistId}/tracks` | 그 아티스트의 곡 — 파고들기 | 200 · 404 |
| `GET /albums` | 앨범 묶음 목록 | 200 · 400 |
| `GET /albums/{albumId}/tracks` | 그 앨범의 곡 — 파고들기 | 200 · 404 |
| `GET /playlists` | 플레이리스트 목록 | 200 |
| `POST /playlists` | 지금 큐를 플레이리스트로 저장 | 201 · 400 · 403 · 409 · 503 |
| `GET /playlists/{playlistId}/tracks` | 플레이리스트의 곡 — 담긴 순서대로 | 200 · 404 |

### catalog

| 엔드포인트 | 하는 일 | 응답 |
|---|---|---|
| `GET /catalog/tracks` | Apple Music 카탈로그 검색 | 200 · 400 · 409 · 502 |
| `POST /library/tracks` | 카탈로그 곡을 내 라이브러리에 담는다 | 201 · 400 · 409 · 502 · 503 |

### playback

| 엔드포인트 | 하는 일 | 응답 |
|---|---|---|
| `GET /playback` | 지금 재생 중인 것 | 200 · 403 · 503 |
| `PATCH /playback` | 켜져 있는 것을 바꾼다 | 200 · 400 · 403 · 503 |
| `POST /playback/play` | 튼다 | 200 · 400 · 403 · 404 · 503 |
| `POST /playback/pause` | 멈춘다 | 200 · 403 · 503 |
| `POST /playback/next` | 다음 곡으로 넘긴다 | 200 · 403 · 503 |
| `GET /playback/artwork` | 지금 곡의 앨범 커버 | 200 · 403 · 404 · 503 |
| `GET /playback/lyrics` | 지금 곡의 가사 | 200 · 404 · 502 · 503 |

### queue

| 엔드포인트 | 하는 일 | 응답 |
|---|---|---|
| `GET /queue` | 지금 큐 | 200 |
| `DELETE /queue` | 큐를 비운다 | 204 · 403 · 503 |
| `POST /queue/items` | 큐에 곡을 넣는다 | 201 · 400 · 404 · 503 |
| `PATCH /queue/items/{queueItemId}` | 순서를 바꾼다 | 200 · 400 · 404 · 409 |
| `DELETE /queue/items/{queueItemId}` | 큐에서 뺀다 | 204 · 404 · 409 |

### turns

| 엔드포인트 | 하는 일 | 응답 |
|---|---|---|
| `POST /turns` | 자연어 요청을 낸다 | 202 · 400 · 409 |
| `GET /turns/usage` | 이 기기가 지금까지 쓴 것 | 200 |
| `GET /turns/{turnId}` | 요청 하나의 진행과 결과 | 200 · 404 |
| `POST /turns/{turnId}/cancel` | 진행 중인 요청을 물린다 | 200 · 404 · 409 |
| `GET /messages` | 로그 띠에 그릴 줄들 | 200 · 400 |

### plays

| 엔드포인트 | 하는 일 | 응답 |
|---|---|---|
| `POST /plays` | 곡 하나가 끝났다 | 201 · 400 · 404 |

## 5-4. 에러 코드 13종

화면(W8)이 하는 말과 하나씩 대응한다.

| `code` | HTTP | 화면이 하는 말 |
|---|---|---|
| `VALIDATION_FAILED` | 400 | 요청 형식이 잘못되었다 |
| `NOT_FOUND` | 404 | 대상을 찾을 수 없다 |
| `AUTOMATION_PERMISSION_DENIED` | 403 | `PERMISSION` — 자동화 권한을 허용해 주세요 |
| `MUSIC_APP_NOT_RUNNING` | 503 | `MUSIC APP` — Music 이 실행되어 있지 않습니다 |
| `SYNC_IN_PROGRESS` | 409 | 이미 읽는 중입니다 |
| `AI_NOT_CONFIGURED` | 409 | `AI is off · /ai <key>` |
| `CATALOG_NOT_CONFIGURED` | 409 | `/setup <team ID>` 가 필요합니다 |
| `CATALOG_UNAVAILABLE` | 502 | 카탈로그에 닿지 못했습니다 |
| `MODEL_UNAVAILABLE` | 502 → 잡 | 모델이 답하지 않습니다 |
| `NO_MATCHING_TRACKS` | 잡 | 조건에 맞는 곡이 없습니다 |
| `TURN_ALREADY_RUNNING` | 409 | 이미 진행 중인 요청이 있습니다 |
| `TRACK_NOT_AVAILABLE` | 409 | 이 곡은 지금 틀 수 없습니다 |
| `LYRICS_NOT_FOUND` | 404 | 가사를 찾지 못했습니다 |

### 공통 에러 스키마

모든 오류가 같은 모양으로 온다. 6개의 공통 응답(`BadRequest` · `Forbidden` ·
`NotFound` · `Conflict` · `BadGateway` · `ServiceUnavailable`)을 정의하고
`$ref` 로 재사용한다.

```yaml
Error:
  type: object
  required: [code, message]
  properties:
    code:    { $ref: '#/components/schemas/ErrorCode' }
    message: { type: string }   # 화면에 그대로 띄울 수 있는 문장
    hint:    { type: string, nullable: true }
```

**`hint` 가 있는 이유:** 화면은 "왜 안 되는지"가 아니라 "무엇을 하면 되는지"를
말해야 한다. `MUSIC_APP_NOT_RUNNING` 의 `hint` 는 `open -a Music` 이다.

## 5-5. 화면 ↔ API 대응

| 화면 | 부르는 API |
|---|---|
| W1 홈 | `GET /status` — 홈 전체가 이 응답 하나로 그려진다 |
| W2 지금 재생 | `GET /playback` · `/playback/artwork` · `/playback/lyrics` |
| W2 Recently Added | `GET /tracks?sort=addedAt&order=desc` |
| W3 Artists · 파고들기 | `GET /artists` · `GET /artists/{id}/tracks` |
| W4 Queue | `GET /queue` · `POST` `PATCH` `DELETE /queue/items` |
| **W5 Unplayed** | `GET /tracks?unplayed=true` |
| W6 Catalog | `GET /catalog/tracks?q=` · `POST /library/tracks` |
| W7 팔레트 | 클라이언트가 갖고 있다 (API 없음) |
| W8 관문 | 모든 응답의 에러 코드 |
| 대화 띠 | `GET /messages` · `POST /turns` · `GET /turns/{id}` |
| 상태줄 누적 비용 | `GET /turns/usage` |

**`GET /tracks` 하나가 목록 구역 셋을 담당한다** — Recently Added · Songs ·
Unplayed 가 정렬과 필터만 다르다. 검색(`shift+tab`)도 같은 엔드포인트에
`q` 를 붙인다.

## 5-6. 페이지네이션

라이브러리는 1만 곡대가 될 수 있으므로 커서 기반으로 나눠 준다.

```yaml
PageMeta:
  hasMore:    boolean
  nextCursor: string | null
```

목록 응답은 전부 `{ items: [...], meta: PageMeta }` 모양이며, `limit` 은
기본 50 · 최대 200 이다.
