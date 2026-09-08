import Foundation

/// 화면 1이 다루는 대화 항목. 발화와 제안이 한 흐름에 섞인다.
/// 원본: `app/src/components/panel/types.ts`

public enum Role: String, Codable, Sendable {
    case host, counterpart
}

public enum ResponseMode: String, Codable, Sendable {
    case direct, conditional, escalate
}

/// 판정의 근거 청크. **이 타입만 snake_case DB row 다.**
/// 스트림 이벤트가 `content`·`distance` 도 함께 보내지만 패널이 쓰지 않아 받지 않는다.
public struct Evidence: Codable, Sendable, Equatable {
    public let id: String
    public let sourceType: String
    public let sourceURL: String
    public let externalRef: String
    public let headingPath: String?
    public let audienceLevel: String
    public let validFrom: String

    enum CodingKeys: String, CodingKey {
        case id
        case sourceType = "source_type"
        case sourceURL = "source_url"
        case externalRef = "external_ref"
        case headingPath = "heading_path"
        case audienceLevel = "audience_level"
        case validFrom = "valid_from"
    }
}

public struct UtteranceItem: Codable, Sendable, Equatable {
    public var id: String
    public var role: Role
    public var text: String
    public var isFinal: Bool

    enum CodingKeys: String, CodingKey { case id, role, text, isFinal }

    public init(id: String, role: Role, text: String, isFinal: Bool) {
        self.id = id
        self.role = role
        self.text = text
        self.isFinal = isFinal
    }
}

public struct SuggestionItem: Codable, Sendable, Equatable {
    /// questionId
    public var id: String
    public var intent: String
    /// nil = 판정 대기 (스켈레톤)
    public var mode: ResponseMode?
    public var headline: String
    public var script: String
    public var condition: String?
    public var evidence: [Evidence]
    public var latencyMs: Int?
    public var done: Bool

    enum CodingKeys: String, CodingKey {
        case id, intent, mode, headline, script, condition, evidence, latencyMs, done
    }

    public init(
        id: String, intent: String, mode: ResponseMode?, headline: String, script: String,
        condition: String?, evidence: [Evidence], latencyMs: Int?, done: Bool
    ) {
        self.id = id
        self.intent = intent
        self.mode = mode
        self.headline = headline
        self.script = script
        self.condition = condition
        self.evidence = evidence
        self.latencyMs = latencyMs
        self.done = done
    }
}

public enum FeedItem: Sendable, Equatable {
    case utterance(UtteranceItem)
    case suggestion(SuggestionItem)
}

extension FeedItem {
    var utteranceID: String? { if case let .utterance(u) = self { u.id } else { nil } }
    var suggestionID: String? { if case let .suggestion(s) = self { s.id } else { nil } }
}

/// `Fixtures/*.feed.json` 을 읽기 위한 디코딩. `kind` 가 판별자다.
extension FeedItem: Decodable {
    private enum Kind: String, Decodable { case utterance, suggestion }
    private enum KindKey: String, CodingKey { case kind }

    public init(from decoder: any Decoder) throws {
        let kind = try decoder.container(keyedBy: KindKey.self).decode(Kind.self, forKey: .kind)
        switch kind {
        case .utterance: self = .utterance(try UtteranceItem(from: decoder))
        case .suggestion: self = .suggestion(try SuggestionItem(from: decoder))
        }
    }
}

public enum Domain: String, Sendable {
    case sales, recruiting
}

/// 패널 헤더가 쓰는 세션 컨텍스트. 서버는 임의 JSON 을 담지만
/// 화면 1이 읽는 건 이 둘뿐이다 (`panel-view.tsx`, `session-header.tsx`).
public struct SessionContext: Decodable, Sendable, Equatable {
    public let ndaSigned: Bool
    public let stage: String?

    enum CodingKeys: String, CodingKey {
        case ndaSigned = "nda_signed"
        case stage
    }

    public init(from decoder: any Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        ndaSigned = try c.decodeIfPresent(Bool.self, forKey: .ndaSigned) ?? false
        stage = try c.decodeIfPresent(String.self, forKey: .stage)
    }
}

/// `POST /api/sessions` 의 응답. **snake_case row 다.**
public struct Session: Decodable, Sendable, Equatable {
    public let id: String
    public let counterpartOrg: String
    public let domain: String
    public let context: SessionContext

    enum CodingKeys: String, CodingKey {
        case id, domain, context
        case counterpartOrg = "counterpart_org"
    }
}
