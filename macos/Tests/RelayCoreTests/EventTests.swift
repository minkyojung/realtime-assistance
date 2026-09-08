import Foundation
import Testing
@testable import RelayCore

@Suite("Event — 픽스처 전 줄 디코딩")
struct EventTests {

    @Test("픽스처의 모든 줄이 디코딩되고 unknown 이 하나도 없다",
          arguments: [("sales-demo", 280), ("recruiting-demo", 145)])
    func everyLineDecodes(name: String, count: Int) throws {
        let events = try Fixture.events(name)
        #expect(events.count == count)
        #expect(events.last == .scriptDone)

        let unknown = events.compactMap { if case let .unknown(type) = $0 { type } else { nil } }
        #expect(unknown.isEmpty, "모르는 type: \(Set(unknown))")
    }

    @Test("모르는 type 은 crash 없이 unknown 이 된다")
    func unknownTypeIsIgnored() throws {
        let json = #"{"type":"something.new","whatever":1}"#
        let event = try JSONDecoder().decode(Event.self, from: Data(json.utf8))
        #expect(event == .unknown(type: "something.new"))
        #expect(FeedStore.reduce([], event).isEmpty)
    }

    /// 3-3절의 함정. 전역 `convertFromSnakeCase` 를 켰다면 여기가 깨진다.
    @Test("camelCase 최상위와 snake_case 근거가 한 이벤트에서 함께 디코딩된다")
    func mixedCasingDecodes() throws {
        let verdicts = try Fixture.events("sales-demo").compactMap {
            if case let .questionVerdict(v) = $0 { v } else { nil }
        }
        #expect(verdicts.count == 4)

        let v = try #require(verdicts.first)
        #expect(!v.questionId.isEmpty)      // camelCase
        #expect(v.latencyMs > 0)            // camelCase
        #expect(v.evidence.count == 3)
        #expect(v.evidence[0].sourceURL.hasPrefix("https://"))   // snake_case
        #expect(!v.evidence[0].externalRef.isEmpty)              // snake_case
        #expect(!v.evidence[0].validFrom.isEmpty)                // snake_case
    }
}
