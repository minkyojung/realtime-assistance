import SwiftUI
import RelayCore

/// 화면 1 패널. 떠 있는 상단 pill + 대화 흐름.
///
/// 재질 결정 (2026-09-08):
///   - 창       반투명 (`PanelWindow` 의 NSVisualEffectView)
///   - pill     유리 `clear` (뒤가 비치는 창 위에서 림 하이라이트가 산다)
///   - 고객 버블 반투명
///   - 내 버블   강조색 단색
///
/// 원본: `app/src/components/panel/panel-view.tsx`. 상태는 `SessionController` 가 들고
/// 여기서는 그리기만 한다.
///
/// 발화와 제안이 **한 흐름에 시간순으로** 섞인다. 피드가 이미 그 순서라서
/// (`question.detected` 가 질문 발화 직후에 온다) 카드는 질문 말풍선 바로 아래에 선다.
public struct PanelView: View {
    private let controller: SessionController

    /// 지금 바닥에 붙어 있는가. 붙어 있을 때만 새 내용을 따라 내려간다.
    @State private var pinnedToBottom = true
    /// 유리들을 한 좌표계로 묶는다. 카드가 이 안에서 신원을 갖는다.
    @Namespace private var glass

    public init(controller: SessionController) {
        self.controller = controller
    }

    public var body: some View {
        // 가까이 있는 유리들을 한 패스로 **병합**해 그린다. pill 과 카드가 서로
        // 다가오면 붙었다 떨어지는 액체 거동이 여기서 나오고, 렌더 패스도 줄어든다.
        // 개별 유리를 각자 그리면 그 거동이 아예 생기지 않는다.
        GlassEffectContainer(spacing: 20) {
            ZStack(alignment: .top) {
                flow
                SessionPill(
                    session: controller.session,
                    elapsed: controller.elapsed,
                    running: controller.running
                )
                .padding(.top, 12)
            }
        }
        // VStack 으로 쌓지 않고 safeAreaBar 로 붙인다. 시스템이 이 바를 Tahoe 재질로
        // 직접 그리고, 창 아래 모서리 안쪽으로 알아서 배치한다 — 우리가 그리면 그 둘을
        // 전부 손으로 재현해야 한다. 스크롤 콘텐츠도 바 뒤로 흘러 안전 영역만큼 밀린다.
        .safeAreaBar(edge: .bottom) { footer }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    private var flow: some View {
        ScrollViewReader { proxy in
            ScrollView {
                // Lazy — 화면에 보이는 행만 만든다. 행 하나하나가 유리라서 눈에 안 보이는
                // 것까지 전부 살려 두면 스크롤 한 프레임의 비용이 대화 길이에 비례해 커진다.
                LazyVStack(spacing: 0) {
                    if let message = controller.errorMessage {
                        Text(message)
                            .font(.system(size: 12))
                            .foregroundStyle(.red)
                            .padding(.top, 12)
                    }
                    if controller.feed.isEmpty && controller.errorMessage == nil {
                        Text(controller.running ? "Opening session…" : "No conversation yet.\nStart a meeting below.")
                            .font(.system(size: 13))
                            .foregroundStyle(.secondary)
                            .multilineTextAlignment(.center)
                            .padding(.top, 96)
                    }
                    ForEach(rows) { row in
                        switch row {
                        case let .utterance(item, first, last):
                            UtteranceBubble(item, isLastInRun: last)
                                .padding(.top, first ? 12 : 3)
                        case let .suggestion(item, replyingTo):
                            SuggestionCard(item, replyingTo: replyingTo, in: glass)
                                .padding(.top, 8)
                                // 새 카드가 유리가 맺히듯 등장한다. 지금은 툭 나타난다.
                                .glassEffectTransition(.materialize)
                        }
                    }
                }
                .frame(maxWidth: .infinity)
                .padding(.horizontal, 14)
                .padding(.top, 56)    // pill 자리
                .padding(.bottom, 16)
            }
            .scrollContentBackground(.hidden)
            // 패널이 작아서 스크롤바가 뜨면 말풍선 위를 덮는다. 흐름은 자동 스크롤로 따라간다.
            .scrollIndicators(.hidden)
            // 위로는 떠 있는 pill, 아래로는 safeAreaBar 가 콘텐츠를 덮는다. 그냥 두면
            // 글이 그 경계에서 딱딱하게 잘린다 — 화면에서 가장 웹 같아 보이던 지점이다.
            // Tahoe 가 이 문제 전용으로 준 API 로, 가장자리에서 콘텐츠가 재질 속으로
            // 흐려지며 사라진다. `.hard` 는 경계를 또렷하게 끊고 `.soft` 는 번지게 둔다 —
            // 유리 아래로 글이 지나가는 그림이라 번지는 쪽이 맞다.
            .scrollEdgeEffectStyle(.soft, for: [.top, .bottom])
            // 대화는 아래가 현재다. 처음 열 때부터 바닥을 본다.
            // (`.initialOffset` 만 지정한다. 전체 역할에 걸면 콘텐츠가 자랄 때마다
            //  시스템이 바닥으로 끌어당겨, 사용자가 위로 올려둔 것을 무시한다.)
            .defaultScrollAnchor(.bottom, for: .initialOffset)
            // 문구가 차오를 때 따라가는 일은 **시스템에 맡긴다.**
            // 직접 scrollTo 를 부르면 delta 가 20ms 마다 오므로 초당 50번 스크롤을
            // 명령하게 되고, 그때마다 레이아웃이 다시 돈다. 앵커는 콘텐츠가 자랄 때
            // 시스템이 알아서 붙잡아 주므로 명령이 0번이 된다.
            // 사용자가 위로 올려 두면 앵커 자체를 뗀다 — 그래야 끌려가지 않는다.
            .defaultScrollAnchor(pinnedToBottom ? .bottom : nil, for: .sizeChanges)
            // 바닥 근처인지 계속 지켜본다. 32pt 는 손으로 살짝 민 정도는 이탈로 보지 않는 여유다.
            .onScrollGeometryChange(for: Bool.self) { geometry in
                geometry.contentOffset.y
                    >= geometry.contentSize.height - geometry.containerSize.height - 32
            } action: { _, isNearBottom in
                pinnedToBottom = isNearBottom
            }
            // 새 **항목**이 붙을 때만 직접 내린다. 글자가 차오르는 건 위 앵커가 맡는다.
            // **사용자가 위로 올려 뒀으면 따라가지 않는다** — 근거를 확인하려고 올려놨는데
            // 새 발화가 왔다고 도로 내려가면 그 순간 이 패널은 쓸 수 없는 물건이 된다.
            .onChange(of: controller.feed.count) {
                guard pinnedToBottom, let last = rows.last else { return }
                withAnimation(.easeOut(duration: 0.2)) {
                    proxy.scrollTo(last.id, anchor: .bottom)
                }
            }
        }
    }

    /// ScriptedSource 재생 컨트롤. 웹 패널의 footer 와 같다 — 실제 제품에서는 오디오 캡처 토글.
    private var footer: some View {
        HStack(spacing: 8) {
            Button(controller.running ? "Running…" : "Start meeting (Sales)") {
                controller.start(domain: .sales)
            }
            .buttonStyle(.borderedProminent)
            Button("Recruiting") {
                controller.start(domain: .recruiting)
            }
            .buttonStyle(.bordered)
            Spacer()
            Text("ScriptedSource · no audio")
                .font(.system(size: 10))
                .foregroundStyle(.tertiary)
        }
        .controlSize(.small)
        .disabled(controller.running)
        .padding(.horizontal, 14)
        .padding(.vertical, 10)
    }

    private enum Row: Identifiable {
        /// `first`/`last` 는 같은 화자의 연속 발화 묶음에서의 위치다.
        /// 첫 줄은 위 간격을 벌리고, 마지막 줄만 꼬리를 단다.
        case utterance(UtteranceItem, first: Bool, last: Bool)
        /// `replyingTo` 는 이 제안이 답하는 말 — 바로 앞 발화다.
        case suggestion(SuggestionItem, replyingTo: String?)

        /// 발화 id 와 질문 id 는 다른 공간의 값이라 접두사로 갈라 둔다.
        var id: String {
            switch self {
            case let .utterance(item, _, _): "u-\(item.id)"
            case let .suggestion(item, _):   "s-\(item.id)"
            }
        }
    }

    /// 피드를 그릴 순서 그대로 훑는다.
    ///
    /// 연속 발화 판정에서 **제안 카드는 묶음을 끊는다.** 사이에 카드가 끼면 화면상
    /// 이어진 말이 아니므로, 꼬리와 간격도 거기서 끊겨야 한다.
    private var rows: [Row] {
        let feed = controller.feed
        func utterance(at index: Int) -> UtteranceItem? {
            guard feed.indices.contains(index), case let .utterance(u) = feed[index] else { return nil }
            return u
        }

        return feed.indices.map { i in
            switch feed[i] {
            case let .suggestion(item):
                // 답하는 대상은 바로 앞 발화다 — 서버가 `question.detected` 를 질문
                // 발화 직후에 보내므로 피드 순서가 곧 답장 관계다.
                .suggestion(item, replyingTo: utterance(at: i - 1)?.text)
            case let .utterance(item):
                .utterance(
                    item,
                    first: utterance(at: i - 1)?.role != item.role,
                    last: utterance(at: i + 1)?.role != item.role)
            }
        }
    }
}
