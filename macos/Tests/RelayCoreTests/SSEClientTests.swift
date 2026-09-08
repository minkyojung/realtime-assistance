import Foundation
import Testing
@testable import RelayCore

/// 픽스처를 HTTP 응답으로 흘려보내는 스텁.
///
/// 실제 포트를 열지 않고 `URLProtocol` 로 가로챈다. 본문을 잘게 쪼개 보내므로
/// **이벤트 한 줄이 여러 청크에 걸쳐 도착하는 상황**이 그대로 재현된다 — 파서의 진짜 위험.
final class FixtureProtocol: URLProtocol, @unchecked Sendable {
    nonisolated(unsafe) static var body = Data()
    nonisolated(unsafe) static var chunkSize = 137
    nonisolated(unsafe) static var status = 200
    nonisolated(unsafe) static var didStop = false

    private var feeder: Task<Void, Never>?

    static func reset(body: Data, status: Int = 200) {
        Self.body = body
        Self.status = status
        Self.didStop = false
    }

    static func makeSession() -> URLSession {
        let config = URLSessionConfiguration.ephemeral
        config.protocolClasses = [FixtureProtocol.self]
        return URLSession(configuration: config)
    }

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        let response = HTTPURLResponse(
            url: request.url!, statusCode: Self.status, httpVersion: "HTTP/1.1",
            headerFields: ["Content-Type": "text/event-stream"])!
        client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)

        let body = Self.body
        let size = Self.chunkSize
        feeder = Task {
            var i = body.startIndex
            while i < body.endIndex {
                if Task.isCancelled { return }
                let end = body.index(i, offsetBy: size, limitedBy: body.endIndex) ?? body.endIndex
                self.client?.urlProtocol(self, didLoad: Data(body[i..<end]))
                i = end
                try? await Task.sleep(for: .milliseconds(1))
            }
            self.client?.urlProtocolDidFinishLoading(self)
        }
    }

    override func stopLoading() {
        feeder?.cancel()
        Self.didStop = true
    }
}

/// 스텁이 전역 상태(`FixtureProtocol.body`)를 쓰므로 직렬 실행한다.
@Suite("SSEClient", .serialized)
struct SSEClientTests {
    static let url = URL(string: "http://localhost:3000/api/sessions/x/stream")!

    @Test("청크 경계와 무관하게 끝까지 수신한다", arguments: ["sales-demo", "recruiting-demo"])
    func receivesEveryEvent(name: String) async throws {
        FixtureProtocol.reset(body: try Fixture.sse(name))
        let client = SSEClient(session: FixtureProtocol.makeSession())

        var received = [Event]()
        for try await event in client.events(from: Self.url) { received.append(event) }

        #expect(received == (try Fixture.events(name)))
    }

    @Test("수신한 이벤트를 그대로 재생하면 기대값 피드가 나온다")
    func endToEndFeedMatches() async throws {
        FixtureProtocol.reset(body: try Fixture.sse("sales-demo"))
        let client = SSEClient(session: FixtureProtocol.makeSession())

        let store = await FeedStore()
        await store.start()
        for try await event in client.events(from: Self.url) { await store.apply(event) }

        #expect(await store.feed == (try Fixture.expectedFeed("sales-demo")))
        #expect(await !store.running)
    }

    @Test("중간에 멈추면 연결이 닫힌다")
    func cancellationClosesConnection() async throws {
        FixtureProtocol.reset(body: try Fixture.sse("sales-demo"))
        let client = SSEClient(session: FixtureProtocol.makeSession())

        var received = 0
        for try await _ in client.events(from: Self.url) {
            received += 1
            if received == 5 { break }
        }
        #expect(received == 5)

        // 취소가 URLSession 까지 전파되는 데 한 틱 걸린다.
        for _ in 0..<100 where !FixtureProtocol.didStop {
            try await Task.sleep(for: .milliseconds(10))
        }
        #expect(FixtureProtocol.didStop, "스트림을 벗어났는데 연결이 살아 있다")
    }

    @Test("깨진 줄이 있어도 스트림이 끊기지 않는다")
    func malformedLineIsSkipped() async throws {
        var body = "data: {\"type\":\"utterance.partial\",\"id\":\"a\",\"role\":\"host\",\"text\":\"안녕\"}\n\n"
        body += "data: {이건 JSON 이 아니다\n\n"
        body += "data: {\"type\":\"utterance.partial\",\"id\":\"a\",\"role\":\"host\"}\n\n"  // text 누락
        body += "data: {\"type\":\"script.done\"}\n\n"
        FixtureProtocol.reset(body: Data(body.utf8))
        let client = SSEClient(session: FixtureProtocol.makeSession())

        var received = [Event]()
        for try await event in client.events(from: Self.url) { received.append(event) }

        #expect(received == [
            .utterancePartial(id: "a", role: .host, text: "안녕"),
            .scriptDone,
        ])
    }

    @Test("200 이 아니면 오류를 던진다")
    func nonOKStatusThrows() async throws {
        FixtureProtocol.reset(body: Data(), status: 404)
        let client = SSEClient(session: FixtureProtocol.makeSession())

        await #expect(throws: RelayError.badStatus(404)) {
            for try await _ in client.events(from: Self.url) {}
        }
    }
}
