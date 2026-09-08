import Foundation

public enum RelayError: Error, Sendable, Equatable {
    case badStatus(Int)
    case badResponse
}

/// `GET /api/sessions/{id}/stream` 구독.
///
/// 서버는 한 이벤트를 `data: <json>\n\n` 한 줄로 보낸다. 이벤트 이름(`event:`)이나
/// 다중 줄 `data:` 는 쓰지 않으므로 그만큼만 파싱한다.
public struct SSEClient: Sendable {
    private let session: URLSession

    public init(session: URLSession = .shared) {
        self.session = session
    }

    /// 스트림이 끝나거나(`script.done`) 소비자가 중단하면 종료한다.
    /// 소비자가 루프를 벗어나면 `onTermination` 이 URLSession 태스크를 취소한다.
    public func events(from url: URL) -> AsyncThrowingStream<Event, any Error> {
        AsyncThrowingStream { continuation in
            let task = Task {
                do {
                    var request = URLRequest(url: url)
                    request.setValue("text/event-stream", forHTTPHeaderField: "Accept")
                    request.timeoutInterval = 3600

                    let (bytes, response) = try await session.bytes(for: request)
                    guard let http = response as? HTTPURLResponse else {
                        throw RelayError.badResponse
                    }
                    guard http.statusCode == 200 else {
                        throw RelayError.badStatus(http.statusCode)
                    }

                    let decoder = JSONDecoder()
                    for try await line in bytes.lines {
                        try Task.checkCancellation()
                        guard line.hasPrefix("data: ") else { continue }

                        // 깨진 줄 하나 때문에 미팅 중 스트림이 끊기지 않게 한다.
                        // 모르는 `type` 은 예외가 아니라 `.unknown` 으로 내려온다.
                        guard let event = try? decoder.decode(
                            Event.self, from: Data(line.dropFirst(6).utf8)) else { continue }

                        continuation.yield(event)
                        if case .scriptDone = event { break }
                    }
                    continuation.finish()
                } catch {
                    continuation.finish(throwing: error)
                }
            }
            continuation.onTermination = { _ in task.cancel() }
        }
    }
}
