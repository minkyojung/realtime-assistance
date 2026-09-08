import Foundation
import Testing
@testable import RelayCore

/// **실패를 주입하는** 스텁. 정상 재생은 `FixtureReplay` 가 맡으므로
/// 여기서는 그걸로 만들 수 없는 상황만 만든다 — 세션 생성 실패, 서버 `error` 이벤트.
///
/// `FixtureProtocol`(본문 하나를 그대로 흘림)로도 안 된다. POST 와 GET 에
/// **다르게** 답해야 하기 때문이다 (생성은 201 JSON, 스트림은 주입한 본문).
final class FailureStub: URLProtocol, @unchecked Sendable {
    nonisolated(unsafe) static var createStatus = 201
    nonisolated(unsafe) static var streamBody = Data()

    /// 실패 경로만 보므로 세션 본문은 고정이다. 값이 중요한 검증은 `FixtureReplay` 쪽에 있다.
    static let sessionJSON = Data("""
        {"session":{"id":"s1","counterpart_org":"Acme Corp","domain":"sales",
                    "context":{"nda_signed":false,"stage":"discovery"}}}
        """.utf8)

    static func makeSession() -> URLSession {
        let config = URLSessionConfiguration.ephemeral
        config.protocolClasses = [FailureStub.self]
        return URLSession(configuration: config)
    }

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        let isCreate = request.httpMethod == "POST"
        let status = isCreate ? Self.createStatus : 200
        let body = isCreate ? Self.sessionJSON : Self.streamBody
        let response = HTTPURLResponse(
            url: request.url!, statusCode: status, httpVersion: "HTTP/1.1",
            headerFields: ["Content-Type": isCreate ? "application/json" : "text/event-stream"])!
        client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: body)
        client?.urlProtocolDidFinishLoading(self)
    }

    override func stopLoading() {}
}

@MainActor
func makeController(_ urlSession: URLSession) -> SessionController {
    SessionController(api: RelayAPI(session: urlSession), client: SSEClient(session: urlSession))
}

extension SessionController {
    @MainActor
    var suggestions: [SuggestionItem] {
        feed.compactMap { if case let .suggestion(s) = $0 { s } else { nil } }
    }
}

/// 카드가 답을 받고 **요청까지 정리될** 때까지 기다린다. `questionID` 를 주면 그 하나만 본다.
///
/// `done` 만 보면 이르다. 마지막 이벤트가 도착한 뒤 스트림이 닫히고 태스크가 스스로를
/// 비우기까지 한 틱이 더 있어서, 그 사이에는 `askState` 가 아직 `.waiting` 이다.
@MainActor
func waitUntilAnswered(
    _ c: SessionController, questionID: String? = nil, timeout: Duration = .seconds(10)
) async {
    let deadline = ContinuousClock.now + timeout
    while ContinuousClock.now < deadline {
        let suggestions = c.suggestions.filter { questionID == nil || $0.id == questionID }
        if !suggestions.isEmpty,
           suggestions.allSatisfy({ $0.done && c.askState(for: $0.id) == .idle }) { return }
        try? await Task.sleep(for: .milliseconds(20))
    }
}

/// `running` 이 내려갈 때까지 기다린다. 스트림이 끝나면 컨트롤러가 스스로 내린다.
@MainActor
func waitUntilStopped(_ c: SessionController, timeout: Duration = .seconds(10)) async {
    let deadline = ContinuousClock.now + timeout
    while c.running, ContinuousClock.now < deadline {
        try? await Task.sleep(for: .milliseconds(20))
    }
}

/// `FailureStub` 이 전역 상태를 쓰므로 직렬로 돈다.
@Suite("SessionController — 코어를 화면 상태로 묶는다", .serialized)
struct SessionControllerTests {

    /// 재생본을 끝까지 돌려도 **카드는 판정까지만 찬다.** 답은 버튼을 눌러야 온다.
    @Test("재생본을 끝까지 돌리면 카드가 답을 기다린다",
          arguments: [(Domain.sales, "sales-demo", "Acme Corp"),
                      (.recruiting, "recruiting-demo", "지원자 김OO")])
    @MainActor
    func replayRoundTrip(domain: Domain, fixture: String, org: String) async throws {
        let c = makeController(FixtureReplay.urlSession(speed: 500))

        c.start(domain: domain)
        #expect(c.running)
        await waitUntilStopped(c)

        #expect(!c.running)
        #expect(c.errorMessage == nil)
        #expect(c.session?.counterpartOrg == org)
        #expect(c.feed.count == (try Fixture.expectedFeed(fixture)).count)

        let suggestions = c.suggestions
        #expect(!suggestions.isEmpty)
        #expect(suggestions.allSatisfy { $0.mode != nil && $0.script.isEmpty && !$0.done })
        #expect(suggestions.allSatisfy { c.askState(for: $0.id) == .idle })
    }

    /// 버튼을 누르는 경로. 여기까지 와야 녹화본과 같은 화면이 된다.
    @Test("ask 를 부르면 답이 채워지고 기대값 피드가 된다",
          arguments: [(Domain.sales, "sales-demo"), (.recruiting, "recruiting-demo")])
    @MainActor
    func askFillsAnswers(domain: Domain, fixture: String) async throws {
        let c = makeController(FixtureReplay.urlSession(speed: 500))
        c.start(domain: domain)
        await waitUntilStopped(c)

        for suggestion in c.suggestions { c.ask(questionID: suggestion.id) }
        await waitUntilAnswered(c)

        #expect(c.feed == (try Fixture.expectedFeed(fixture)))
        #expect(c.suggestions.allSatisfy { c.askState(for: $0.id) == .idle })
    }

    @Test("같은 질문을 두 번 눌러도 요청은 한 번만 나간다")
    @MainActor
    func askIsIdempotentWhileInFlight() async throws {
        let c = makeController(FixtureReplay.urlSession(speed: 20))
        c.start(domain: .sales)
        await waitUntilStopped(c)

        let id = try #require(c.suggestions.first?.id)
        c.ask(questionID: id)
        #expect(c.askState(for: id) == .waiting)
        c.ask(questionID: id)   // 무시돼야 한다
        await waitUntilAnswered(c, questionID: id)

        let answered = try #require(c.suggestions.first)
        let expected = try Fixture.expectedFeed("sales-demo")
            .compactMap { if case let .suggestion(s) = $0 { s } else { nil } }
        #expect(answered.script == expected.first?.script, "두 번 받아 문구가 겹쳤다")
    }

    @Test("세션을 다시 시작하면 진행 중이던 답변 요청도 멈춘다")
    @MainActor
    func restartCancelsAsk() async throws {
        let c = makeController(FixtureReplay.urlSession(speed: 8))
        c.start(domain: .sales)
        // 판정이 하나라도 올 때까지만 기다린다 — 스트림이 끝나기 전에 끊어야 하는 검증이다.
        let deadline = ContinuousClock.now + .seconds(20)
        while c.suggestions.first?.mode == nil, ContinuousClock.now < deadline {
            try? await Task.sleep(for: .milliseconds(50))
        }
        let id = try #require(c.suggestions.first?.id)

        c.ask(questionID: id)
        #expect(c.askState(for: id) == .waiting)

        c.stop()
        #expect(c.askState(for: id) == .idle, "요청이 남아 있으면 다음 세션으로 답이 새어 든다")
        #expect(!c.running)
    }

    @Test("세션 생성이 실패하면 오류를 남기고 running 을 내린다")
    @MainActor
    func createFails() async throws {
        FailureStub.createStatus = 500
        let c = makeController(FailureStub.makeSession())

        c.start(domain: .sales)
        await waitUntilStopped(c)

        #expect(!c.running)
        #expect(c.session == nil)
        #expect(c.errorMessage != nil)
        #expect(c.feed.isEmpty)
    }

    @Test("서버 error 이벤트는 화면 오류가 된다")
    @MainActor
    func serverErrorEvent() async throws {
        FailureStub.createStatus = 201
        FailureStub.streamBody = Data("data: {\"type\":\"error\",\"message\":\"boom\"}\n\n".utf8)
        let c = makeController(FailureStub.makeSession())

        c.start(domain: .sales)
        await waitUntilStopped(c)

        #expect(c.errorMessage == "boom")
        #expect(!c.running)
    }

    @Test("다시 시작하면 이전 상태가 지워진다")
    @MainActor
    func restartClears() async throws {
        FailureStub.createStatus = 201
        FailureStub.streamBody = Data("data: {\"type\":\"error\",\"message\":\"boom\"}\n\n".utf8)
        let c = makeController(FailureStub.makeSession())
        c.start(domain: .sales)
        await waitUntilStopped(c)
        #expect(c.errorMessage == "boom")

        FailureStub.streamBody = try Fixture.sse("recruiting-demo")
        c.start(domain: .recruiting)
        #expect(c.errorMessage == nil)
        #expect(c.feed.isEmpty)
        await waitUntilStopped(c)

        #expect(c.errorMessage == nil)
        #expect(c.feed == (try Fixture.expectedFeed("recruiting-demo")))
    }
}

/// 실서버가 필요하다. `pnpm dev` 를 띄운 뒤 `RELAY_LIVE=1 swift test` 로 켠다.
/// 화면이 보는 상태 그대로 — 세션·타이머·피드·종료 — 를 한 번에 확인한다.
/// 재생 모드가 검증하지 못하는 것, 즉 **파이프라인 자체**가 여기서만 걸린다.
@Suite("SessionController 실서버 (RELAY_LIVE=1)",
       .enabled(if: ProcessInfo.processInfo.environment["RELAY_LIVE"] == "1"))
struct SessionControllerLiveTests {

    @Test("시작 → 타이머가 돌고 → 끝나면 멈춘다", .timeLimit(.minutes(2)))
    @MainActor
    func liveRoundTrip() async throws {
        let c = SessionController()
        c.start(domain: .sales)

        try await Task.sleep(for: .milliseconds(1500))
        #expect(c.running)
        #expect(c.elapsed >= 1, "타이머가 안 돈다")
        #expect(c.session?.counterpartOrg == "Acme Corp")

        await waitUntilStopped(c, timeout: .seconds(110))
        #expect(!c.running)
        #expect(c.errorMessage == nil)

        let utterances = c.feed.compactMap { if case let .utterance(u) = $0 { u } else { nil } }
        let suggestions = c.feed.compactMap { if case let .suggestion(s) = $0 { s } else { nil } }
        #expect(utterances.count == 9)
        #expect(utterances.allSatisfy { $0.isFinal })
        #expect(suggestions.count == 4)
        #expect(suggestions.allSatisfy { $0.done && $0.mode != nil })

        let frozen = c.elapsed
        try await Task.sleep(for: .milliseconds(1500))
        #expect(c.elapsed == frozen, "끝난 뒤에도 타이머가 돈다")
    }
}
