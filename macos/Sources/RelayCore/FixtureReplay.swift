import Foundation

/// 녹화해 둔 스트림(`Fixtures/*.sse`)을 서버인 척 되돌려주는 재생 모드.
///
/// UI 를 그리는 일은 "고치고 → 보고 → 다시 고치고"의 반복인데, 그 한 사이클마다
/// docker(Postgres) + `pnpm dev` + OpenAI 호출 대기가 필요하면 작업이 성립하지 않는다.
/// 여기서는 실제 포트를 열지 않고 `URLProtocol` 로 요청을 가로채 파일을 흘려보낸다.
/// 테스트의 `FixtureProtocol` 과 같은 기법이고(VCR · MSW · WireMock 계열),
/// 다르게 쓰는 점 하나는 **시간에 따라 흐르게** 한다는 것이다 — 스켈레톤 → 판정 →
/// script 차오름 같은 움직이는 상태를 눈으로 봐야 하기 때문이다.
///
/// 가로채는 요청은 실서버와 같은 2개다. 응답도 실서버 형태를 그대로 따르므로
/// `RelayAPI` · `SSEClient` · `SessionController` 는 한 줄도 달라지지 않는다.
///
///     let session = FixtureReplay.urlSession()
///     SessionController(api: RelayAPI(session: session), client: SSEClient(session: session))
///
/// 재생본은 **녹화 시점의 판정 결과**다. 파이프라인을 고쳐도 여기 결과는 안 변한다 —
/// 파이프라인을 검증하려면 실서버로 봐야 한다.
public enum FixtureReplay {
    /// `speed` 배속. 2 면 두 배 빠르게 재생한다.
    ///
    /// 배속을 전역에 두지 않고 **세션 헤더로 실어 보낸다.** 전역이면 재생 세션이
    /// 둘 이상일 때(예: 병렬로 도는 테스트 스위트) 서로의 배속을 덮어쓴다.
    /// `httpAdditionalHeaders` 는 그 세션의 모든 요청에 붙으므로 `RelayAPI` ·
    /// `SSEClient` 는 이 사실을 몰라도 된다.
    public static func urlSession(speed: Double = 1) -> URLSession {
        let config = URLSessionConfiguration.ephemeral
        config.protocolClasses = [ReplayProtocol.self]
        config.httpAdditionalHeaders = [ReplayProtocol.speedHeader: String(max(speed, 0.01))]
        return URLSession(configuration: config)
    }

    /// `Fixtures/` 위치. 번들 안(`Relay.app/Contents/Resources`)을 먼저 보고,
    /// 없으면 소스 트리로 떨어진다 — `swift run` 처럼 번들이 아닐 때의 경로다.
    static var directory: URL {
        if let bundled = Bundle.main.resourceURL?.appendingPathComponent("Fixtures"),
           FileManager.default.fileExists(atPath: bundled.path) {
            return bundled
        }
        return URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()  // RelayCore
            .deletingLastPathComponent()  // Sources
            .deletingLastPathComponent()  // macos
            .appendingPathComponent("Fixtures")
    }
}

/// 재생 전용 세션(`FixtureReplay.urlSession()`)에만 설치되므로 다른 통신에 영향이 없다.
///
/// 재생 루프에 `Task` 대신 `DispatchQueue` 를 쓴다. `URLProtocol` 은 시스템이
/// `startLoading`/`stopLoading` 을 불러 주는 콜백 객체이지 Sendable 값이 아니라서,
/// `Task` 로 넘기려면 region 격리 검사를 우회해야 한다. 직렬 큐를 쓰면 그 우회 자체가
/// 필요 없어지고 이벤트 순서도 큐가 보장한다.
final class ReplayProtocol: URLProtocol, @unchecked Sendable {
    /// 배속을 싣고 오는 헤더. `FixtureReplay.urlSession(speed:)` 가 붙인다.
    static let speedHeader = "X-Relay-Replay-Speed"

    private var speed: Double {
        Double(request.value(forHTTPHeaderField: Self.speedHeader) ?? "") ?? 1
    }

    /// 이벤트 사이 간격(초). 픽스처에는 타임스탬프가 없어서 **합성한 값**이다.
    /// script 가 흐르는 느낌만 재현하면 되므로 delta 만 촘촘하게 둔다.
    private static let deltaGap = 0.02
    private static let eventGap = 0.15

    /// 재생을 태우는 직렬 큐. `cancelled` 도 이 큐 위에서만 읽고 쓴다.
    private let queue = DispatchQueue(label: "relay.fixture-replay")
    private var cancelled = false

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        guard request.httpMethod == "POST" else { return streamEvents() }
        if request.url?.lastPathComponent == "answer" { answerEvents() } else { createSession() }
    }

    override func stopLoading() {
        queue.async { self.cancelled = true }
    }

    // MARK: - POST /api/sessions

    /// 보낸 본문을 그대로 되돌려준다. 상대·컨텍스트를 여기서 또 적으면
    /// `RelayAPI` 의 것과 갈라지므로, 받은 값에 id 만 붙여 echo 한다.
    private func createSession() {
        let sent = (try? JSONSerialization.jsonObject(with: requestBody)) as? [String: Any] ?? [:]
        let domain = sent["domain"] as? String ?? Domain.sales.rawValue

        var session: [String: Any] = ["id": "\(Self.idPrefix)\(domain)", "domain": domain]
        session["counterpart_org"] = sent["counterpartOrg"] ?? ""
        session["context"] = sent["context"] ?? [:]

        guard let body = try? JSONSerialization.data(withJSONObject: ["session": session]) else {
            client?.urlProtocol(self, didFailWithError: RelayError.badResponse)
            return
        }
        send(status: 201, contentType: "application/json")
        client?.urlProtocol(self, didLoad: body)
        client?.urlProtocolDidFinishLoading(self)
    }

    /// `URLSession` 이 본문을 스트림으로 바꿔 두는 경우가 있어 양쪽을 다 본다.
    private var requestBody: Data {
        if let body = request.httpBody { return body }
        guard let stream = request.httpBodyStream else { return Data() }
        stream.open()
        defer { stream.close() }

        var data = Data()
        var buffer = [UInt8](repeating: 0, count: 4096)
        while stream.hasBytesAvailable {
            let read = stream.read(&buffer, maxLength: buffer.count)
            guard read > 0 else { break }
            data.append(buffer, count: read)
        }
        return data
    }

    // MARK: - GET /api/sessions/{id}/stream

    private func streamEvents() {
        guard let text = try? String(contentsOf: fixtureURL, encoding: .utf8) else {
            client?.urlProtocol(self, didFailWithError: RelayError.badStatus(404))
            return
        }
        send(status: 200, contentType: "text/event-stream")

        // 답변 이벤트는 세션 스트림에서 빼고 `POST .../answer` 로만 내보낸다.
        // 서버도 같은 형태로 갈 예정이라, 여기서 빼 두어야 재생본이 실제와 어긋나지 않는다.
        let lines = text.split(separator: "\n", omittingEmptySubsequences: true)
            .map(String.init)
            .filter { Self.answerQuestionID($0) == nil }
        queue.async { self.emit(lines, from: 0) }
    }

    // MARK: - POST /api/questions/{id}/answer

    /// 서버가 검색·생성에 쓰는 시간. 버튼을 누른 뒤 곧장 글자가 쏟아지면
    /// 기다리는 동안의 화면(스피너·시간 벌기 문구)을 눈으로 확인할 수 없다.
    private static let thinkDelay = 1.2

    /// 녹화본에서 그 질문의 답변 이벤트만 뽑아 다시 흘려보낸다.
    /// 실제 서버는 여기서 HyDE·재검색·생성을 돌지만, 나가는 이벤트는 같다.
    private func answerEvents() {
        let questionID = request.url?.pathComponents.dropLast().last ?? ""
        let lines = Self.answerLines(for: questionID)
        guard !lines.isEmpty else {
            send(status: 404, contentType: "text/event-stream")
            client?.urlProtocolDidFinishLoading(self)
            return
        }
        send(status: 200, contentType: "text/event-stream")
        queue.asyncAfter(deadline: .now() + Self.thinkDelay / speed) {
            self.emit(lines, from: 0)
        }
    }

    /// 어느 녹화본에 있는지 모르므로 전부 훑는다. 픽스처는 두어 개뿐이다.
    private static func answerLines(for questionID: String) -> [String] {
        guard !questionID.isEmpty,
              let names = try? FileManager.default.contentsOfDirectory(
                  atPath: FixtureReplay.directory.path)
        else { return [] }

        for name in names.sorted() where name.hasSuffix(".sse") {
            guard let text = try? String(
                contentsOf: FixtureReplay.directory.appendingPathComponent(name), encoding: .utf8)
            else { continue }

            let lines = text.split(separator: "\n", omittingEmptySubsequences: true)
                .map(String.init)
                .filter { answerQuestionID($0) == questionID }
            if !lines.isEmpty { return lines }
        }
        return []
    }

    /// 답변 이벤트(`question.delta` · `question.done`)면 그 questionId, 아니면 nil.
    private static func answerQuestionID(_ line: String) -> String? {
        switch decode(line) {
        case let .questionDelta(questionId, _): questionId
        case let .questionDone(questionId, _, _): questionId
        default: nil
        }
    }

    /// 한 줄 보내고, 간격만큼 쉬었다가 다음 줄로.
    private func emit(_ lines: [String], from index: Int) {
        guard !cancelled else { return }
        guard index < lines.count else {
            client?.urlProtocolDidFinishLoading(self)
            return
        }

        let line = lines[index]
        client?.urlProtocol(self, didLoad: Data("\(line)\n\n".utf8))

        queue.asyncAfter(deadline: .now() + Self.gap(after: line) / speed) {
            self.emit(lines, from: index + 1)
        }
    }

    /// 줄 자체는 원본 그대로 보낸다. 디코딩은 **간격과 분류에만** 쓴다 —
    /// 깨진 줄이 섞여 있어도 재생이 멈추지 않아야 한다.
    private static func gap(after line: String) -> Double {
        if case .questionDelta = decode(line) { deltaGap } else { eventGap }
    }

    private static func decode(_ line: String) -> Event? {
        guard line.hasPrefix("data: ") else { return nil }
        return try? JSONDecoder().decode(Event.self, from: Data(line.dropFirst(6).utf8))
    }

    /// 세션 id 에 도메인을 실어 뒀다(`createSession`). 스트림 요청에는 그것만 오기 때문이다.
    private static let idPrefix = "fixture-"

    private var fixtureURL: URL {
        let id = request.url?.pathComponents.dropLast().last ?? ""
        let domain = id.hasPrefix(Self.idPrefix) ? String(id.dropFirst(Self.idPrefix.count)) : id
        return FixtureReplay.directory.appendingPathComponent("\(domain)-demo.sse")
    }

    // MARK: -

    private func send(status: Int, contentType: String) {
        let response = HTTPURLResponse(
            url: request.url!, statusCode: status, httpVersion: "HTTP/1.1",
            headerFields: ["Content-Type": contentType])!
        client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
    }
}
