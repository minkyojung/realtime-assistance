import Foundation
import Observation

/// 세션 하나의 수명. 코어 세 조각(`RelayAPI` · `SSEClient` · `FeedStore`)을 묶어
/// 화면이 볼 상태 하나로 만든다. 원본: `app/src/components/panel/use-session-stream.ts`
///
/// 웹과 같은 순서로 동작한다 — 피드 비움 → 세션 생성 → 스트림 구독 → `script.done` 에 종료.
/// 카드 하나가 답을 기다리는 중인지. 답 내용(headline·script·근거)은 피드에 있고
/// 여기에는 **요청의 진행 상태만** 둔다 — 그래야 `FeedStore` 가 서버 이벤트만으로
/// 결정되는 상태로 남고, TS 리듀서와의 대조가 계속 성립한다.
public enum AskState: Sendable, Equatable {
    case idle
    case waiting
    case failed(String)
}

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
    /// 답변 요청 하나당 태스크 하나. 같은 질문을 두 번 누르면 두 번째는 무시한다.
    private var askTasks: [String: Task<Void, Never>] = [:]
    private var askErrors: [String: String] = [:]

    public init(api: RelayAPI = RelayAPI(), client: SSEClient = SSEClient()) {
        self.api = api
        self.client = client
    }

    public func start(domain: Domain) {
        stop()
        errorMessage = nil
        askErrors.removeAll()
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

    /// 답 요청 — 사용자가 카드의 버튼을 눌렀을 때만 돈다.
    ///
    /// 질문마다 자동으로 답을 만들지 않는 이유는 시간이다. 자동이면 대화를 끊지
    /// 않으려고 1초 안에 끝내야 해서 검색이 얕아지고, 그 결과가 질문과 무관한
    /// 근거였다. 누른 순간부터는 몇 초를 써도 되므로 서버가 제대로 찾을 수 있다.
    ///
    /// 응답 이벤트는 세션 스트림의 것과 같아서 `store` 가 그대로 받는다.
    public func ask(questionID: String) {
        guard askTasks[questionID] == nil else { return }
        askErrors[questionID] = nil

        askTasks[questionID] = Task { [weak self] in
            guard let self else { return }
            do {
                for try await event in client.events(for: api.answerRequest(questionID: questionID)) {
                    self.store.apply(event)
                }
            } catch is CancellationError {
                // 세션이 끝나 취소된 것이다. 오류로 보여줄 일이 아니다.
            } catch {
                self.askErrors[questionID] = String(describing: error)
            }
            self.askTasks[questionID] = nil
        }
    }

    /// 카드 하나의 요청 상태. 답 내용은 피드가 들고 있으므로 여기엔 없다.
    public func askState(for questionID: String) -> AskState {
        if let message = askErrors[questionID] { return .failed(message) }
        return askTasks[questionID] == nil ? .idle : .waiting
    }

    public func stop() {
        streamTask?.cancel()
        streamTask = nil
        for task in askTasks.values { task.cancel() }
        askTasks.removeAll()
        finish()
    }

    /// 스트림이 끝난 뒤(정상·오류·중단 모두) 시계를 멈추고 `running` 을 내린다.
    private func finish() {
        clockTask?.cancel()
        clockTask = nil
        store.stop()
    }
}
