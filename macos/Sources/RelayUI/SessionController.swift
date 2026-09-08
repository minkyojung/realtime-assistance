import Foundation
import Observation
import RelayCore

/// 세션 하나의 수명. 코어 세 조각(`RelayAPI` · `SSEClient` · `FeedStore`)을 묶어
/// 화면이 볼 상태 하나로 만든다. 원본: `app/src/components/panel/use-session-stream.ts`
///
/// 웹과 같은 순서로 동작한다 — 피드 비움 → 세션 생성 → 스트림 구독 → `script.done` 에 종료.
@MainActor
@Observable
public final class SessionController {
    public private(set) var session: Session?
    /// 세션 생성 실패·스트림 오류·서버 `error` 이벤트. 다음 `start` 에서 지워진다.
    public private(set) var errorMessage: String?
    /// 세션 시작부터 흐른 초. `running` 인 동안만 1초마다 오른다.
    public private(set) var elapsed = 0

    public let store = FeedStore()
    public var feed: [FeedItem] { store.feed }
    public var running: Bool { store.running }

    private let api: RelayAPI
    private let client: SSEClient
    private var streamTask: Task<Void, Never>?
    private var clockTask: Task<Void, Never>?

    public init(api: RelayAPI = RelayAPI(), client: SSEClient = SSEClient()) {
        self.api = api
        self.client = client
    }

    public func start(domain: Domain) {
        stop()
        errorMessage = nil
        session = nil
        elapsed = 0
        store.start()

        let startedAt = Date()
        clockTask = Task { [weak self] in
            while !Task.isCancelled {
                try? await Task.sleep(for: .seconds(1))
                guard let self, !Task.isCancelled else { return }
                self.elapsed = Int(Date().timeIntervalSince(startedAt))
            }
        }

        streamTask = Task { [weak self] in
            guard let self else { return }
            do {
                let session = try await api.createSession(domain: domain)
                self.session = session
                for try await event in client.events(from: api.streamURL(sessionID: session.id)) {
                    if case let .error(message) = event { self.errorMessage = message }
                    self.store.apply(event)
                }
            } catch is CancellationError {
                return
            } catch {
                self.errorMessage = String(describing: error)
            }
            self.finish()
        }
    }

    public func stop() {
        streamTask?.cancel()
        streamTask = nil
        finish()
    }

    /// 스트림이 끝난 뒤(정상·오류·중단 모두) 시계를 멈추고 `running` 을 내린다.
    private func finish() {
        clockTask?.cancel()
        clockTask = nil
        store.stop()
    }
}
