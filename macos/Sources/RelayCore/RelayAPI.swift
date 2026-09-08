import Foundation

/// Next.js 서버(`localhost:3000`)에 붙는다. 서버는 그대로 두고 클라이언트만 옮기므로
/// 여기서 만드는 요청은 웹 패널(`use-session-stream.ts`)과 완전히 같아야 한다.
public struct RelayAPI: Sendable {
    public let baseURL: URL
    private let session: URLSession

    public init(baseURL: URL = URL(string: "http://localhost:3000")!, session: URLSession = .shared) {
        self.baseURL = baseURL
        self.session = session
    }

    public func createSession(domain: Domain) async throws -> Session {
        var request = URLRequest(url: baseURL.appendingPathComponent("api/sessions"))
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONEncoder().encode(CreateRequest(domain: domain))

        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else { throw RelayError.badResponse }
        guard http.statusCode == 201 else { throw RelayError.badStatus(http.statusCode) }

        return try JSONDecoder().decode(CreateResponse.self, from: data).session
    }

    public func streamURL(sessionID: String) -> URL {
        baseURL.appendingPathComponent("api/sessions/\(sessionID)/stream")
    }
}

/// 웹 패널이 보내는 것과 같은 본문. 도메인별 컨텍스트도 그대로 따른다.
private struct CreateRequest: Encodable {
    let domain: String
    let counterpartOrg: String
    let context: Context

    init(domain: Domain) {
        self.domain = domain.rawValue
        switch domain {
        case .sales:
            counterpartOrg = "Acme Corp"
            context = Context(ndaSigned: false, stage: "discovery")
        case .recruiting:
            counterpartOrg = "지원자 김OO"
            context = Context(interviewRound: 2, positionLevel: "senior")
        }
    }

    /// **snake_case.** 서버가 JSON 을 그대로 DB `context` 컬럼에 넣는다.
    struct Context: Encodable {
        var ndaSigned: Bool?
        var stage: String?
        var interviewRound: Int?
        var positionLevel: String?

        enum CodingKeys: String, CodingKey {
            case ndaSigned = "nda_signed"
            case stage
            case interviewRound = "interview_round"
            case positionLevel = "position_level"
        }
    }

    enum CodingKeys: String, CodingKey { case domain, counterpartOrg, context }
}

private struct CreateResponse: Decodable {
    let session: Session
}
