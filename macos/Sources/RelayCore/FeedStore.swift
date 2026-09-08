import Foundation
import Observation

/// SSE 이벤트 → 대화 피드 상태.
///
/// 서버 이벤트를 그대로 반영하며 클라이언트에서 판정을 다시 계산하지 않는다.
/// 원본: `app/src/components/panel/reduce-feed.ts` — 여기가 달라지면 화면이 웹과 갈린다.
@MainActor
@Observable
public final class FeedStore {
    public private(set) var feed: [FeedItem] = []
    public private(set) var running = false

    public init() {}

    public func start() {
        feed = []
        running = true
    }

    public func apply(_ event: Event) {
        feed = Self.reduce(feed, event)
        if case .scriptDone = event { running = false }
    }

    public func stop() {
        running = false
    }

    /// 이벤트 하나를 피드에 반영한다. 순수 함수 — 테스트가 UI 없이 이걸 직접 돌린다.
    ///
    /// `gap.created` 는 무시한다 (2026-09-08 결정, 근거는 `Event.swift` 주석).
    nonisolated public static func reduce(_ prev: [FeedItem], _ event: Event) -> [FeedItem] {
        var next = prev

        switch event {
        // ① 말하는 동안 자막이 채워진다
        case let .utterancePartial(id, role, text):
            let item = FeedItem.utterance(
                UtteranceItem(id: id, role: role, text: text, isFinal: false))
            if let i = next.firstIndex(where: { $0.utteranceID == id }) { next[i] = item }
            else { next.append(item) }

        case let .utteranceFinal(tempId, utterance):
            let item = FeedItem.utterance(
                UtteranceItem(id: utterance.id, role: utterance.role,
                              text: utterance.text, isFinal: true))
            if let i = next.firstIndex(where: { $0.utteranceID == tempId }) { next[i] = item }
            else { next.append(item) }

        // ② 질문 감지 → 스켈레톤 카드
        case let .questionDetected(questionId, _, intent):
            next.append(.suggestion(SuggestionItem(
                id: questionId, intent: intent, mode: nil, headline: "", script: "",
                condition: nil, evidence: [], latencyMs: nil, done: false)))

        // ③ 판정 도착 → 색과 headline 즉시 표시
        case let .questionVerdict(v):
            guard let i = next.firstIndex(where: { $0.suggestionID == v.questionId }),
                  case var .suggestion(cur) = next[i] else { break }
            cur.mode = v.responseMode
            cur.headline = v.headline
            cur.condition = v.condition
            cur.evidence = v.evidence
            cur.latencyMs = v.latencyMs
            next[i] = .suggestion(cur)

        // ④ 제안 문구가 채워진다
        case let .questionDelta(questionId, delta):
            guard let i = next.firstIndex(where: { $0.suggestionID == questionId }),
                  case var .suggestion(cur) = next[i] else { break }
            cur.script += delta
            next[i] = .suggestion(cur)

        case let .questionDone(questionId, script, _):
            guard let i = next.firstIndex(where: { $0.suggestionID == questionId }),
                  case var .suggestion(cur) = next[i] else { break }
            cur.script = script
            cur.done = true
            next[i] = .suggestion(cur)

        case .gapCreated, .scriptDone, .error, .unknown:
            break
        }

        return next
    }
}
