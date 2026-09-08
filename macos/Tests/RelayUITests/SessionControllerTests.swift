import Foundation
import Testing
import RelayCore
@testable import RelayUI

/// `POST /api/sessions` 와 `GET …/stream` 두 요청을 가로채는 스텁.
/// 실제 포트를 열지 않고 `URLProtocol` 로 답한다. 전역 상태라 테스트는 직렬로 돈다.
final class RelayStub: URLProtocol, @unchecked Sendable {
    nonisolated(unsafe) static var createStatus = 201
    nonisolated(unsafe) static var streamBody = Data()

    static let sessionJSON = Data("""
        {"session":{"id":"s1","counterpart_org":"Acme Corp","domain":"sales",
                    "context":{"nda_signed":false,"stage":"discovery"}}}
        """.utf8)

    static func makeSession() -> URLSession {
        let config = URLSessionConfiguration.ephemeral
        config.protocolClasses = [RelayStub.self]
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

enum UIFixture {
    static let directory = URL(fileURLWithPath: #filePath)
        .deletingLastPathComponent()  // RelayUITests
        .deletingLastPathComponent()  // Tests
        .deletingLastPathComponent()  // macos
        .appendingPathComponent("Fixtures")

    static func data(_ file: String) throws -> Data {
        try Data(contentsOf: directory.appendingPathComponent(file))
    }
}

@MainActor
func makeController() -> SessionController {
    let session = RelayStub.makeSession()
    return SessionController(api: RelayAPI(session: session), client: SSEClient(session: session))
}

/// `running` 이 내려갈 때까지 기다린다. 스트림이 끝나면 컨트롤러가 스스로 내린다.
@MainActor
func waitUntilStopped(_ c: SessionController, timeout: Duration = .seconds(10)) async {
    let deadline = ContinuousClock.now + timeout
    while c.running, ContinuousClock.now < deadline {
        try? await Task.sleep(for: .milliseconds(20))
    }
}

@Suite("SessionController — 코어를 화면 상태로 묶는다", .serialized)
struct SessionControllerTests {

    @Test("세션 생성 → 스트림 수신 → 피드가 TS 기대값과 같다")
    @MainActor
    func fixtureRoundTrip() async throws {
        RelayStub.createStatus = 201
        RelayStub.streamBody = try UIFixture.data("sales-demo.sse")
        let c = makeController()

        c.start(domain: .sales)
        #expect(c.running)
        await waitUntilStopped(c)

        #expect(!c.running)
        #expect(c.errorMessage == nil)
        #expect(c.session?.counterpartOrg == "Acme Corp")
        #expect(c.session?.context.ndaSigned == false)
        let expected = try JSONDecoder().decode([FeedItem].self, from: UIFixture.data("sales-demo.feed.json"))
        #expect(c.feed == expected)
    }

    @Test("세션 생성이 실패하면 오류를 남기고 running 을 내린다")
    @MainActor
    func createFails() async throws {
        RelayStub.createStatus = 500
        let c = makeController()

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
        RelayStub.createStatus = 201
        RelayStub.streamBody = Data("data: {\"type\":\"error\",\"message\":\"boom\"}\n\n".utf8)
        let c = makeController()

        c.start(domain: .sales)
        await waitUntilStopped(c)

        #expect(c.errorMessage == "boom")
        #expect(!c.running)
    }

    @Test("다시 시작하면 이전 상태가 지워진다")
    @MainActor
    func restartClears() async throws {
        RelayStub.createStatus = 201
        RelayStub.streamBody = Data("data: {\"type\":\"error\",\"message\":\"boom\"}\n\n".utf8)
        let c = makeController()
        c.start(domain: .sales)
        await waitUntilStopped(c)
        #expect(c.errorMessage == "boom")

        RelayStub.streamBody = try UIFixture.data("recruiting-demo.sse")
        c.start(domain: .recruiting)
        #expect(c.errorMessage == nil)
        #expect(c.feed.isEmpty)
        await waitUntilStopped(c)

        #expect(c.errorMessage == nil)
        let expected = try JSONDecoder().decode([FeedItem].self, from: UIFixture.data("recruiting-demo.feed.json"))
        #expect(c.feed == expected)
    }
}

/// 실서버가 필요하다. `pnpm dev` 를 띄운 뒤 `RELAY_LIVE=1 swift test` 로 켠다.
/// 화면이 보는 상태 그대로 — 세션·타이머·피드·종료 — 를 한 번에 확인한다.
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
