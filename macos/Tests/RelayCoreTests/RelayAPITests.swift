import Foundation
import Testing
@testable import RelayCore

@Suite("RelayAPI")
struct RelayAPITests {

    @Test("스트림 URL 을 만든다")
    func buildsStreamURL() {
        let api = RelayAPI()
        #expect(api.streamURL(sessionID: "abc").absoluteString
                == "http://localhost:3000/api/sessions/abc/stream")
    }

    /// 실서버가 필요하다. `pnpm dev` 를 띄운 뒤 `RELAY_LIVE=1 swift test` 로 켠다.
    /// DB 에 세션 row 가 남으므로 끝나면 `pnpm tsx scripts/reset-demo.ts` 로 정리한다.
    @Test("실서버에서 세션을 만든다 (RELAY_LIVE=1)",
          .enabled(if: ProcessInfo.processInfo.environment["RELAY_LIVE"] == "1"),
          arguments: [Domain.sales, .recruiting])
    func createsSessionOnLiveServer(domain: Domain) async throws {
        let session = try await RelayAPI().createSession(domain: domain)

        #expect(!session.id.isEmpty)
        #expect(session.domain == domain.rawValue)
        switch domain {
        case .sales:
            #expect(session.counterpartOrg == "Acme Corp")
            #expect(session.context.ndaSigned == false)
            #expect(session.context.stage == "discovery")
        case .recruiting:
            #expect(session.counterpartOrg == "지원자 김OO")
            #expect(session.context.ndaSigned == false)  // 키가 없으면 false
            #expect(session.context.stage == nil)
        }
    }
}

/// 스텁이 아니라 진짜 Next.js 스트림에 붙어 본다. LLM 출력은 매번 다르므로
/// 내용이 아니라 **구조**만 확인한다 — 픽스처 대조는 `SSEClientTests` 가 맡는다.
@Suite("실서버 스트림 (RELAY_LIVE=1)",
       .enabled(if: ProcessInfo.processInfo.environment["RELAY_LIVE"] == "1"),
       .serialized)
struct LiveStreamTests {

    @Test("세션 생성 → 스트림 구독 → 판정이 붙은 카드가 나온다", .timeLimit(.minutes(2)))
    func liveRoundTrip() async throws {
        let api = RelayAPI()
        let session = try await api.createSession(domain: .sales)

        let store = await FeedStore()
        await store.start()
        var events = 0
        var sawUnknown = [String]()

        for try await event in SSEClient().events(from: api.streamURL(sessionID: session.id)) {
            events += 1
            if case let .unknown(type) = event { sawUnknown.append(type) }
            await store.apply(event)
        }

        #expect(events > 100)
        #expect(sawUnknown.isEmpty, "계약에 없는 이벤트: \(Set(sawUnknown))")
        #expect(await !store.running, "script.done 을 못 받았다")

        let feed = await store.feed
        let utterances = feed.compactMap { if case let .utterance(u) = $0 { u } else { nil } }
        let suggestions = feed.compactMap { if case let .suggestion(s) = $0 { s } else { nil } }

        #expect(utterances.count == 9)
        #expect(utterances.allSatisfy { $0.isFinal }, "임시 버블이 남았다")
        #expect(suggestions.count == 4)
        #expect(suggestions.allSatisfy { $0.done && $0.mode != nil && !$0.script.isEmpty })
        #expect(suggestions.allSatisfy { $0.evidence.count == 3 })
    }
}
