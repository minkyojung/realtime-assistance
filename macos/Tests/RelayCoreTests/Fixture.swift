import Foundation
import Testing
@testable import RelayCore

/// 픽스처를 테스트에서 찾을 수 있게 한다.
///
/// `Fixtures/` 는 SwiftPM 리소스가 아니라 저장소의 일반 폴더라 번들에 들어가지 않는다.
/// `#filePath` 기준으로 거슬러 올라가 찾는다.
enum Fixture {
    static let directory = URL(fileURLWithPath: #filePath)
        .deletingLastPathComponent()  // RelayCoreTests
        .deletingLastPathComponent()  // Tests
        .deletingLastPathComponent()  // macos
        .appendingPathComponent("Fixtures")

    static func sse(_ name: String) throws -> Data {
        try Data(contentsOf: directory.appendingPathComponent("\(name).sse"))
    }

    /// `data: ` 로 시작하는 줄의 JSON 본문만 뽑는다.
    static func lines(_ name: String) throws -> [String] {
        let text = try String(contentsOf: directory.appendingPathComponent("\(name).sse"), encoding: .utf8)
        return text.split(separator: "\n").compactMap {
            $0.hasPrefix("data: ") ? String($0.dropFirst(6)) : nil
        }
    }

    static func events(_ name: String) throws -> [Event] {
        let decoder = JSONDecoder()
        return try lines(name).map { try decoder.decode(Event.self, from: Data($0.utf8)) }
    }

    /// TS 리듀서가 만든 기대값.
    static func expectedFeed(_ name: String) throws -> [FeedItem] {
        let data = try Data(contentsOf: directory.appendingPathComponent("\(name).feed.json"))
        return try JSONDecoder().decode([FeedItem].self, from: data)
    }
}
