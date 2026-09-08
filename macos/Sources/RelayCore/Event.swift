//  Event.swift
//  스트림 계약. 구현은 Phase 1.
//
//  ─────────────────────────────────────────────────────────────────────────
//  패널이 쓰는 API (2개)
//
//    POST /api/sessions
//      요청  { domain, counterpartOrg, context }
//      응답  201 { session: { id, counterpart_org, domain, context, … } }
//
//    GET  /api/sessions/{id}/stream
//      응답  text/event-stream, 한 줄이 `data: <json>\n\n`
//
//  ─────────────────────────────────────────────────────────────────────────
//  SSE 이벤트
//
//    type                페이로드                                          피드 반영
//    ──────────────────  ────────────────────────────────────────────────  ─────────────────────────────────
//    utterance.partial   id, role, text                                    같은 id 버블 텍스트 갱신 (없으면 추가)
//    utterance.final     tempId, utterance{ …row, role }                   tempId 버블을 확정 row 로 교체
//    question.detected   questionId, normalized, intent                    스켈레톤 제안 카드 추가 (mode: nil)
//    question.verdict    questionId, responseMode, headline, condition,    카드 판정 채움
//                        evidence[], latencyMs
//    question.delta      questionId, delta                                 script += delta
//    question.done       questionId, script, totalMs                       done = true, 최종 script 로 덮어씀
//    gap.created         questionId, gapId, reason                         무시 ↓
//    script.done         —                                                 스트림 종료, running = false
//    error               message                                           에러 표시
//
//  gap.created 는 무시한다 (2026-09-08 결정).
//  화면 2(지식 공백 큐)가 같은 정보를 보여주므로 패널에서는 중복이다.
//  웹 리듀서(`app/src/components/panel/reduce-feed.ts`)도 동일하게 무시하며,
//  기대값 `Fixtures/*.feed.json` 이 그 동작으로 생성돼 있다. 표시하기로 바꾸려면
//  웹·Swift·기대값 세 곳을 함께 고쳐야 한다.
//
//  설계 문서 표에 있는 `utterance` 는 서버 내부 이벤트이며 스트림에 나오지 않는다
//  (`stream/route.ts` 가 걸러냄). 픽스처에도 0건이다.
//
//  ─────────────────────────────────────────────────────────────────────────
//  함정: 케이싱이 섞여 있다
//
//    최상위 필드            camelCase   questionId, responseMode, latencyMs
//    utterance / session   snake_case  session_id, is_final, has_question_mark, counterpart_org
//
//  전역 `keyDecodingStrategy = .convertFromSnakeCase` 를 쓰면 camelCase 가 깨진다.
//  타입별 CodingKeys 로 명시하고 전역 전략은 쓰지 않는다.

import Foundation

/// 스트림이 내려보내는 이벤트 하나.
///
/// 전역 `keyDecodingStrategy` 를 쓰지 않는다. 페이로드마다 `CodingKeys` 를 명시한다.
public enum Event: Sendable, Equatable {
    case utterancePartial(id: String, role: Role, text: String)
    case utteranceFinal(tempId: String, utterance: FinalUtterance)
    case questionDetected(questionId: String, normalized: String, intent: String)
    case questionVerdict(Verdict)
    case questionDelta(questionId: String, delta: String)
    case questionDone(questionId: String, script: String, totalMs: Int)
    case gapCreated(questionId: String, gapId: String, reason: String)
    case scriptDone
    case error(message: String)
    /// 모르는 `type`. 무시하고 계속 간다 — 서버에 이벤트가 추가돼도 패널이 죽지 않게.
    case unknown(type: String)
}

/// `utterance.final` 안의 확정 발화. DB row 라 snake_case 필드가 섞여 있지만
/// 패널이 읽는 셋(`id`, `role`, `text`)은 마침 케이싱이 같다.
public struct FinalUtterance: Decodable, Sendable, Equatable {
    public let id: String
    public let role: Role
    public let text: String

    enum CodingKeys: String, CodingKey { case id, role, text }
}

/// `question.verdict` 의 페이로드. **최상위는 camelCase.**
public struct Verdict: Decodable, Sendable, Equatable {
    public let questionId: String
    public let responseMode: ResponseMode
    public let headline: String
    public let condition: String?
    public let evidence: [Evidence]
    public let latencyMs: Int

    enum CodingKeys: String, CodingKey {
        case questionId, responseMode, headline, condition, evidence, latencyMs
    }
}

extension Event: Decodable {
    private enum TypeKey: String, CodingKey { case type }

    private struct Partial: Decodable {
        let id: String, role: Role, text: String
        enum CodingKeys: String, CodingKey { case id, role, text }
    }
    private struct Final: Decodable {
        let tempId: String, utterance: FinalUtterance
        enum CodingKeys: String, CodingKey { case tempId, utterance }
    }
    private struct Detected: Decodable {
        let questionId: String, normalized: String, intent: String
        enum CodingKeys: String, CodingKey { case questionId, normalized, intent }
    }
    private struct Delta: Decodable {
        let questionId: String, delta: String
        enum CodingKeys: String, CodingKey { case questionId, delta }
    }
    private struct Done: Decodable {
        let questionId: String, script: String, totalMs: Int
        enum CodingKeys: String, CodingKey { case questionId, script, totalMs }
    }
    private struct Gap: Decodable {
        let questionId: String, gapId: String, reason: String
        enum CodingKeys: String, CodingKey { case questionId, gapId, reason }
    }
    private struct Failure: Decodable {
        let message: String
        enum CodingKeys: String, CodingKey { case message }
    }

    public init(from decoder: any Decoder) throws {
        let type = try decoder.container(keyedBy: TypeKey.self).decode(String.self, forKey: .type)
        switch type {
        case "utterance.partial":
            let p = try Partial(from: decoder)
            self = .utterancePartial(id: p.id, role: p.role, text: p.text)
        case "utterance.final":
            let p = try Final(from: decoder)
            self = .utteranceFinal(tempId: p.tempId, utterance: p.utterance)
        case "question.detected":
            let p = try Detected(from: decoder)
            self = .questionDetected(questionId: p.questionId, normalized: p.normalized, intent: p.intent)
        case "question.verdict":
            self = .questionVerdict(try Verdict(from: decoder))
        case "question.delta":
            let p = try Delta(from: decoder)
            self = .questionDelta(questionId: p.questionId, delta: p.delta)
        case "question.done":
            let p = try Done(from: decoder)
            self = .questionDone(questionId: p.questionId, script: p.script, totalMs: p.totalMs)
        case "gap.created":
            let p = try Gap(from: decoder)
            self = .gapCreated(questionId: p.questionId, gapId: p.gapId, reason: p.reason)
        case "script.done":
            self = .scriptDone
        case "error":
            self = .error(message: try Failure(from: decoder).message)
        default:
            self = .unknown(type: type)
        }
    }
}
