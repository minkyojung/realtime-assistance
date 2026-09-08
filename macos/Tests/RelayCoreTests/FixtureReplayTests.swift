import Foundation
import Testing
@testable import RelayCore

/// 재생 모드가 **실서버와 구분되지 않는지** 본다.
/// 구분되는 순간 미리보기로 다듬은 화면이 실제와 갈라지므로, 여기가 그 경계다.
///
/// `ReplayProtocol.speed` 가 전역이라 직렬로 돈다.
@Suite("FixtureReplay — 녹화본을 서버인 척 되돌려준다", .serialized)
struct FixtureReplayTests {
    /// 간격은 눈으로 보라고 넣은 것이라 테스트에서는 걷어낸다.
    static func session() -> URLSession { FixtureReplay.urlSession(speed: 500) }

    @Test("보낸 본문이 세션으로 그대로 돌아온다", arguments: [Domain.sales, .recruiting])
    func createSessionEchoes(domain: Domain) async throws {
        let api = RelayAPI(session: Self.session())
        let session = try await api.createSession(domain: domain)

        #expect(session.domain == domain.rawValue)
        #expect(!session.counterpartOrg.isEmpty, "RelayAPI 가 보낸 상대가 유실됐다")
        if domain == .sales {
            #expect(session.counterpartOrg == "Acme Corp")
            #expect(session.context.ndaSigned == false)
            #expect(session.context.stage == "discovery")
        }
    }

    @Test("생성 → 스트림 재생 결과가 TS 기대값 피드와 같다",
          arguments: [(Domain.sales, "sales-demo"), (.recruiting, "recruiting-demo")])
    func replayMatchesExpectedFeed(domain: Domain, fixture: String) async throws {
        let urlSession = Self.session()
        let api = RelayAPI(session: urlSession)
        let client = SSEClient(session: urlSession)

        let session = try await api.createSession(domain: domain)
        let store = await FeedStore()
        await store.start()
        for try await event in client.events(from: api.streamURL(sessionID: session.id)) {
            await store.apply(event)
        }

        #expect(await store.feed == (try Fixture.expectedFeed(fixture)))
        #expect(await !store.running)
    }

    @Test("이벤트가 한꺼번에 오지 않고 시간에 따라 흐른다")
    func replayIsPaced() async throws {
        let urlSession = FixtureReplay.urlSession(speed: 20)
        let api = RelayAPI(session: urlSession)
        let client = SSEClient(session: urlSession)
        let session = try await api.createSession(domain: .sales)

        // 첫 이벤트와 열 번째 이벤트 사이에 실제로 시간이 흘러야 한다.
        // 한꺼번에 쏟아지면 스켈레톤·스트리밍 같은 중간 상태를 볼 수 없다.
        var stamps = [ContinuousClock.Instant]()
        for try await _ in client.events(from: api.streamURL(sessionID: session.id)) {
            stamps.append(.now)
            if stamps.count == 10 { break }
        }

        #expect(stamps.count == 10)
        #expect(stamps[9] - stamps[0] > .milliseconds(10), "간격 없이 한꺼번에 도착했다")
    }

    @Test("녹화본이 없는 도메인은 오류가 된다")
    func missingFixtureFails() async throws {
        let client = SSEClient(session: Self.session())
        let url = URL(string: "http://localhost:3000/api/sessions/fixture-nope/stream")!

        await #expect(throws: (any Error).self) {
            for try await _ in client.events(from: url) {}
        }
    }
}
