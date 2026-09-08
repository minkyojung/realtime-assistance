# Relay — 화면 1 Swift 마이그레이션 계획

> 2026-09-08 작성. **과제 제출(09-10 14:00) 이후** 착수 전제.
> 범위: 실시간 어시스트 패널(화면 1)만 SwiftUI로 옮긴다. 백엔드·화면 2·3은 그대로.
> 견적: SwiftUI 숙련자 기준 **3.5일** (버퍼 포함 4일).

---

## 0. 왜 옮기는가, 왜 패널만인가

| 이유 | 설명 |
|---|---|
| Liquid Glass | Electron은 `NSVisualEffectView`까지만 노출. 굴절·주변광 반응은 `NSGlassEffectView`/`.glassEffect()` 전용 |
| 패널이 효과를 가장 크게 받음 | 항상 떠 있는 유리창이라 재질 차이가 상시 노출됨. 화면 2·3은 책상 앞 일반 창이라 이득이 작음 |
| 시스템 오디오 | `ScreenCaptureKit`이 네이티브. `03-기술스택.md`에서 Tauri를 기각한 사유가 Swift에선 반대로 장점 |
| 화면 2·3은 웹 유지 | 브라우저에서도 써야 하고, 옮겨도 시각적 이득이 없음. 두 벌 유지 비용만 생김 |

---

## 1. 범위 — 옮기는 것 / 건드리지 않는 것

`git ls-files` 실측 (2026-09-08).

| 영역 | 줄 수 | 처리 |
|---|---|---|
| `app/src/lib` 파이프라인 (감지·검색·판정·생성) | 1,230 | **그대로** |
| `app/src/app/api` REST + SSE | 270 | **그대로** |
| `app/scripts`, `db/schema.sql`, docker | 1,005 | **그대로** |
| `app/src/components/gaps` 화면 2 | 433 | **그대로** (웹) |
| `app/src/components/panel` 화면 1 | 421 | **→ SwiftUI** |
| `app/src/app/panel*`, `session-header.tsx` | 116 | **→ SwiftUI** |
| `app/electron/main.js` | 73 | **→ NSPanel** (Phase 3 완료 후 삭제) |
| `app/src/components/ui` shadcn | 1,118 | 안 옮김. SwiftUI 기본 컴포넌트 |

**백엔드 수정 0줄.** 컴포넌트가 `fetch`/`EventSource`로만 통신하며 서버 액션·직접 DB 접근이 없음을 확인함.

---

## 2. 목표 아키텍처

```
┌──────────────────────────────┐        ┌──────────────────────────────┐
│  Relay.app (Swift)            │  HTTP  │  Next.js  (localhost:3000)    │
│                               │ ─────▶ │                               │
│  NSPanel(.nonactivating)      │  SSE   │  /api/sessions  (기존)         │
│   └ NSHostingView             │ ◀───── │  /api/sessions/{id}/stream    │
│      └ PanelView (SwiftUI)    │        │  /api/deferrals (화면 2가 씀)  │
│         .glassEffect()        │        │                               │
│                               │        │  lib/pipeline → Postgres      │
│  RelayCore (SwiftPM)          │        │               → OpenAI        │
│   ├ SSEClient                 │        └──────────────────────────────┘
│   ├ Event (Codable enum)      │                    ▲
│   └ FeedStore (@Observable)   │        브라우저 ── /gaps (화면 2, 그대로)
└──────────────────────────────┘
```

Electron과 Swift 앱은 **같은 서버에 동시에 붙을 수 있다.** 전환 기간 동안 둘을 나란히 띄워 비교한다.

---

## 3. 계약 고정 — 옮기기 전에 얼려 두는 것

### 3-1. 패널이 쓰는 API (2개)

| 호출 | 요청 | 응답 |
|---|---|---|
| `POST /api/sessions` | `{ domain, counterpartOrg, context }` | `201 { session: {id, counterpart_org, domain, context, …} }` |
| `GET /api/sessions/{id}/stream` | — | `text/event-stream`, `data: <json>\n\n` |

> **답변 오류 신고는 이 목록에 없다.**
> `POST /questions/{id}/reject` 는 API 명세에 선언돼 있으나 미구현이며,
> 동작하지 않는 버튼을 옮기지 않기 위해 화면에서도 제거했다(2026-09-08).
> 구현 시 이 표에 3번째 행으로 추가하고 카드 푸터에 버튼을 되살린다.

### 3-2. SSE 이벤트 (10종)

| type | 페이로드 | 피드 반영 |
|---|---|---|
| `utterance.partial` | `id, role, text` | 같은 `id` 버블 텍스트 갱신 (없으면 추가) |
| `utterance.final` | `tempId, utterance{…row, role}` | `tempId` 버블을 확정 row로 교체 |
| `question.detected` | `questionId, normalized, intent` | 스켈레톤 제안 카드 추가 (`mode: nil`) |
| `question.verdict` | `questionId, responseMode, headline, condition, evidence[], latencyMs` | 카드 판정 채움 |
| `question.delta` | `questionId, delta` | `script += delta` |
| `question.done` | `questionId, script, totalMs` | `done = true`, 최종 script로 덮어씀 |
| `gap.created` | `questionId, gapId, reason` | 카드에 공백 표시 |
| `script.done` | — | 스트림 종료, `running = false` |
| `error` | `message` | 에러 표시 |
| `utterance` | (서버 내부, 스트림에는 안 나감) | 무시 |

### 3-3. 함정: 케이싱이 섞여 있다

- 이벤트 최상위 필드는 **camelCase** (`questionId`, `responseMode`, `latencyMs`)
- `utterance.final.utterance` 와 `session` 은 **snake_case** DB row (`session_id`, `is_final`, `has_question_mark`, `counterpart_org`)

→ Swift에서 전역 `keyDecodingStrategy = .convertFromSnakeCase` 를 쓰면 camelCase 필드가 깨진다.
**타입별 `CodingKeys` 로 명시**하고, 전역 전략은 쓰지 않는다.

### 3-4. 픽스처 녹화

옮기기 전에 실제 스트림을 파일로 떠 둔다.

```
curl -N localhost:3000/api/sessions/{id}/stream > macos/Fixtures/sales-demo.sse
```

이 파일이 Swift 파서·리듀서 테스트의 입력이 된다. TS `use-session-stream.ts`가 만드는 피드와
**항목 수·순서·최종 텍스트가 동일**해야 통과.

비교 대상인 TS 피드도 파일로 떠 둔다. 리듀서만 떼어내 픽스처를 먹이고 결과를 JSON 으로 출력하는
스크립트가 필요하다 (`app/scripts/dump-feed.ts`, 약 20줄). 이게 없으면 "동일하다"를 검증할 기준이 없다.

```
Fixtures/sales-demo.sse        입력 (양쪽 공통)
Fixtures/sales-demo.feed.json  TS 리듀서 출력 = Swift 테스트의 기대값
```

---

## 4. 단계별 계획

각 단계는 **완료 기준**이 통과해야 다음으로 간다.

### Phase 0 — 준비 (0.5일)

| 작업 | 완료 기준 |
|---|---|
| `macos/` 디렉터리, SwiftPM 패키지 `RelayCore` + 앱 타겟 `Relay` 생성 | `swift build` 통과 |
| 세션 2종(sales / recruiting) SSE 픽스처 녹화 | `Fixtures/*.sse` 2개 존재 |
| 이 문서의 3절 표를 `RelayCore/Event.swift` 주석으로 옮김 | — |

### Phase 1 — 코어 (1일) · UI 없음

| 작업 | 완료 기준 |
|---|---|
| `Event` — `enum` + `Codable`, 타입별 `CodingKeys` | 픽스처 전 줄 디코딩 성공, 알 수 없는 `type`은 무시 (crash 금지) |
| `SSEClient` — `URLSession.bytes(for:)` → `lines` → `data:` 파싱 → `AsyncStream<Event>` | 픽스처를 로컬 HTTP로 서빙해 끝까지 수신, 취소 시 누수 없음 |
| `FeedStore` — `@MainActor @Observable`, TS reducer 이식 | 픽스처 재생 결과가 TS 피드와 동일 (항목 수·순서·최종 텍스트) |
| `RelayAPI` — `createSession` | 실서버 대상 201 수신 |

> 이 단계가 전체의 절반이다. 여기서 TS 리듀서와 1:1로 맞춰 두면 UI는 순수 SwiftUI 작업이 된다.

### Phase 2 — 창 + UI (1.5일)

| 작업 | 완료 기준 |
|---|---|
| `PanelWindow` — `NSPanel(styleMask: [.nonactivatingPanel, .fullSizeContentView, …])`, `level = .floating`, `collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary]`, `NSHostingView` | 패널 클릭 시 뒤 앱(브라우저)이 활성 상태 유지 |
| `SessionHeader` — 녹음 점·경과·상대·단계 (현 `session-header.tsx` 이식) | 타이머가 캡처 중에만 돈다 |
| `UtteranceBubble` — partial/final 구분 | partial 중 텍스트 갱신이 깜빡이지 않음 |
| `SuggestionCard` — 스켈레톤 → 판정 → 스트리밍 script → done | `question.delta` 수신 중 한글 조합이 튀지 않음 |
| `SourceBadge`, 근거 펼침 | — |
| 자동 스크롤 (`ScrollViewReader`) | 새 항목 추가 시 하단 고정, 사용자가 위로 올리면 멈춤 |
| `.glassEffect()` 적용, 라이트/다크 | 시스템 외관 전환 시 즉시 반영 |
| **데모 E2E** | `pnpm dev` 띄운 상태에서 sales 스크립트 9발화 → 카드 4장, 판정 🟢2 🟡1 🔴1 (`08-MVP-Phase2-결과.md`와 동일) |
| **캡처 확보** ★ | 판정 3색 + 대기 상태 스크린샷을 `wireframes/out/` 에 저장. **Phase 3 의 `sharingType = .none` 을 켜기 전에 반드시 끝낸다** |

### Phase 3 — 폴리시 + 전환 (0.5일)

| 작업 | 완료 기준 |
|---|---|
| `window.sharingType = .none` (화면 공유·녹화에서 제외) | QuickTime 화면 녹화에 패널이 안 찍힘 |
| ↑ 단, `RELAY_CAPTURE=1` 환경변수일 때는 `.readOnly` 로 둔다 | 캡처가 필요할 때 재빌드 없이 전환 가능 |
| 전역 단축키 (`⌘\` 숨김/표시) — `NSEvent.addGlobalMonitorForEvents` 또는 `CGEventTap` | 다른 앱 활성 상태에서 동작 |
| Reduce Transparency 대응 (`accessibilityDisplayShouldReduceTransparency`) | 설정 켜면 불투명 배경으로 |
| Electron 셸 제거 — `app/electron/`, `dev:desktop` 스크립트, `electron`·`concurrently`·`wait-on` 의존성 | `pnpm build`·`lint` 통과, `/panel` 라우트는 브라우저 미리보기용으로 유지 |
| 문서 갱신 — `03-기술스택.md` 스택 표, `05-화면-필드.md` 캡처 교체 | **Phase 2 에서 확보한 캡처를 사용한다.** 이 단계에서 새로 찍으려 하면 `sharingType = .none` 때문에 찍히지 않는다 |

### Phase 4 — 실제 오디오 (선택, 2~3일) · 별도 결정

현재 파이프라인은 **서버가 `ScriptedSource`를 재생해 내려보내는 구조**라 클라이언트 → 서버 업링크가 없다.
실제 오디오는 인터페이스 추가가 필요하며, 이건 Swift 전환과 무관하게 Electron에서도 똑같이 필요했던 작업이다.

| 작업 | 비고 |
|---|---|
| `AVAudioEngine` 마이크 + `ScreenCaptureKit` 시스템 오디오 (2트랙) | TCC 권한 2종 (마이크·화면 녹화) |
| OpenAI Realtime WS로 STT — 트랙당 세션 1개, `semantic_vad` | `03-기술스택.md` 결정 그대로 |
| `POST /api/sessions/{id}/utterances` — 확정 발화 업링크 (**명세에는 선언돼 있고 구현만 없음**) | 서버는 `saveUtterance` + `processUtterance` 호출. `stream` 라우트에서 ScriptedSource 분기 제거 |
| `AudioSource` 프로토콜 — `ScriptedSource` / `LiveSource` 교체 가능 | 설계 문서의 인터페이스를 클라이언트로 옮김 |

---

## 5. 리포 구조

```
macos/
├── Package.swift                 # RelayCore (라이브러리) + RelayCoreTests
├── Sources/RelayCore/
│   ├── Event.swift               # SSE 이벤트 enum + CodingKeys
│   ├── SSEClient.swift
│   ├── RelayAPI.swift
│   └── FeedStore.swift
├── Tests/RelayCoreTests/
│   └── FeedStoreTests.swift      # 픽스처 재생 → TS 피드와 비교
├── Fixtures/
│   ├── sales-demo.sse
│   └── recruiting-demo.sse
└── Relay/                        # Xcode 앱 타겟
    ├── RelayApp.swift
    ├── PanelWindow.swift         # NSPanel + NSHostingView
    └── Views/
        ├── PanelView.swift
        ├── SessionHeader.swift
        ├── UtteranceBubble.swift
        ├── SuggestionCard.swift
        └── SourceBadge.swift
```

코어를 SwiftPM 패키지로 분리하는 이유: **Xcode 없이 `swift test`로 리듀서를 검증**할 수 있고,
UI와 무관하게 CI에서 돌릴 수 있다.

---

## 6. 리스크와 대응

| 리스크 | 영향 | 대응 |
|---|---|---|
| Swift 6 엄격 동시성 — SSE 스트림(백그라운드) → `FeedStore`(메인) 경계 | 컴파일 에러 폭주 | `FeedStore`를 `@MainActor`로, `SSEClient`는 `Sendable` 이벤트만 방출. 첫날에 경계를 확정 |
| 케이싱 혼재 (3-3절) | 조용히 `nil` 디코딩 | 전역 전략 금지, 타입별 `CodingKeys`, 픽스처 테스트로 전 필드 검증 |
| 한글 조합 중 스트리밍 텍스트 튐 | UX | `Text`에 `String` 통째로 교체하지 말고 `AttributedString` 누적, 필요 시 `.contentTransition(.identity)` |
| `next dev` 가 항상 떠 있어야 함 | 데모 절차 +1 | 데모 체크리스트에 명시. 장기적으로 앱이 `next start`를 자식 프로세스로 띄우는 옵션 |
| TCC 권한 팝업 (Phase 4) | 데모 중 팝업 | 리허설 때 미리 허용. `Info.plist` 사용 설명 문구 필수 |
| 알 수 없는 이벤트 type 추가 | 크래시 | `Event` 디코딩에 `unknown` 케이스, 무시하고 계속 |
| Electron·Swift 동시 유지 기간 | 두 벌 수정 | Phase 2 완료 즉시 Electron 제거. 병행 기간 최대 1주 |
| **`sharingType = .none` 이 스크린샷도 막는다** | PDF 캡처 불가 | 캡처는 Phase 2 에서 먼저 확보. Phase 3 은 `RELAY_CAPTURE` 로 토글 |
| **미구현 API 를 그대로 이식** | 동작하지 않는 UI 복제 | 옮기기 전에 화면에서 제거하거나 API 를 먼저 구현. 3-1 절 주석 참조 |

---

## 7. 되돌리기

- Phase 2까지는 **Electron을 삭제하지 않는다.** `pnpm dev:desktop`이 계속 동작하므로 언제든 복귀 가능.
- 백엔드를 안 건드리므로 Swift 쪽을 통째로 지워도 웹·API는 영향 없음.
- Phase 3의 Electron 제거는 별도 커밋으로 격리한다 (revert 1회로 복구).

---

## 8. 문서 반영

| 문서 | 수정 |
|---|---|
| `03-기술스택.md` | 데스크톱 클라이언트 행 → `Swift / SwiftUI (macOS 26+)`. "전 계층 TypeScript" 근거에 예외 기재 |
| `03-기술스택.md` 선택 근거 | Tauri 기각 사유(오디오 캡처)가 Swift에선 장점임을 추가 |
| `05-화면-필드.md` 화면 1 | 필드 변경 없음. 캡처만 교체 |
| PDF 6장 고려사항 | "다음 단계: 패널 네이티브 전환" 항목으로 이 문서 요약 |
