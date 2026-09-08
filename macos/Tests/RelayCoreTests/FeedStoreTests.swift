import Foundation
import Testing
@testable import RelayCore

/// 픽스처를 재생해 TS 리듀서가 만든 기대값(`*.feed.json`)과 대조한다.
/// 여기가 통과하면 UI 는 순수 SwiftUI 작업이 된다.
@Suite("FeedStore — TS 리듀서와 동일한가")
struct FeedStoreTests {

    static let fixtures = ["sales-demo", "recruiting-demo"]

    @Test("픽스처 재생 결과가 기대값과 완전히 일치한다", arguments: fixtures)
    func replayMatchesExpectedFeed(name: String) throws {
        let events = try Fixture.events(name)
        let feed = events.reduce(into: [FeedItem]()) { $0 = FeedStore.reduce($0, $1) }
        let expected = try Fixture.expectedFeed(name)

        #expect(feed.count == expected.count)
        for (i, (got, want)) in zip(feed, expected).enumerated() {
            #expect(got == want, "\(i)번째 항목이 다르다")
        }
    }

    /// `question.done` 이 script 를 통째로 덮어쓰기 때문에, 델타를 전부 무시해도
    /// 최종 피드는 같아진다. 그 구멍을 막는다 — done 직전 스냅샷을 확인한다.
    @Test("델타 누적만으로 script 가 완성된다", arguments: fixtures)
    func deltasAccumulateToFinalScript(name: String) throws {
        var feed = [FeedItem]()
        var checked = 0

        for event in try Fixture.events(name) {
            if case let .questionDone(questionId, script, _) = event {
                let before = feed.first { $0.suggestionID == questionId }
                guard case let .suggestion(s)? = before else {
                    Issue.record("done 이 왔는데 카드가 없다: \(questionId)")
                    continue
                }
                #expect(s.script == script, "델타 누적이 최종 script 와 다르다")
                checked += 1
            }
            feed = FeedStore.reduce(feed, event)
        }

        #expect(checked > 0, "검사한 question.done 이 없다")
    }

    @Test("gap.created 는 피드를 바꾸지 않는다")
    func gapCreatedIsIgnored() throws {
        let events = try Fixture.events("sales-demo")
        let gaps = events.filter { if case .gapCreated = $0 { true } else { false } }
        #expect(gaps.count == 2, "픽스처에 gap.created 가 있어야 이 테스트가 의미 있다")

        let withGaps = events.reduce(into: [FeedItem]()) { $0 = FeedStore.reduce($0, $1) }
        let withoutGaps = events
            .filter { if case .gapCreated = $0 { false } else { true } }
            .reduce(into: [FeedItem]()) { $0 = FeedStore.reduce($0, $1) }
        #expect(withGaps == withoutGaps)
    }

    @Test("script.done 이 오면 running 이 내려간다")
    @MainActor
    func scriptDoneStopsRunning() throws {
        let store = FeedStore()
        store.start()
        #expect(store.running)

        for event in try Fixture.events("sales-demo") { store.apply(event) }

        #expect(!store.running)
        #expect(store.feed == (try Fixture.expectedFeed("sales-demo")))
    }
}
