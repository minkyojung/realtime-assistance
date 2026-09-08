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

    public init(controller: SessionController) {
        self.controller = controller
    }

    public var body: some View {
        ZStack(alignment: .top) {
            flow
            SessionPill(
                session: controller.session,
                elapsed: controller.elapsed,
                running: controller.running
            )
            .padding(.top, 12)
        }
        // VStack 으로 쌓지 않고 safeAreaBar 로 붙인다. 시스템이 이 바를 Tahoe 재질로
        // 직접 그리고, 창 아래 모서리 안쪽으로 알아서 배치한다 — 우리가 그리면 그 둘을
        // 전부 손으로 재현해야 한다. 스크롤 콘텐츠도 바 뒤로 흘러 안전 영역만큼 밀린다.
        .safeAreaBar(edge: .bottom) { footer }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    private var flow: some View {
        ScrollView {
            VStack(spacing: 0) {
                if let message = controller.errorMessage {
                    Text(message)
                        .font(.system(size: 12))
                        .foregroundStyle(.red)
                        .padding(.top, 12)
                }
                if controller.feed.isEmpty && controller.errorMessage == nil {
                    Text(controller.running ? "세션을 여는 중…" : "아직 감지된 대화가 없습니다.\n아래에서 미팅을 시작하세요.")
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
                    case let .suggestion(item):
                        SuggestionCard(item)
                            .padding(.top, 8)
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
    }

    /// ScriptedSource 재생 컨트롤. 웹 패널의 footer 와 같다 — 실제 제품에서는 오디오 캡처 토글.
    private var footer: some View {
        HStack(spacing: 8) {
            Button(controller.running ? "진행 중…" : "미팅 시작 (세일즈)") {
                controller.start(domain: .sales)
            }
            .buttonStyle(.borderedProminent)
            Button("채용 도메인") {
                controller.start(domain: .recruiting)
            }
            .buttonStyle(.bordered)
            Spacer()
            Text("ScriptedSource · 오디오 미사용")
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
        case suggestion(SuggestionItem)

        /// 발화 id 와 질문 id 는 다른 공간의 값이라 접두사로 갈라 둔다.
        var id: String {
            switch self {
            case let .utterance(item, _, _): "u-\(item.id)"
            case let .suggestion(item):      "s-\(item.id)"
            }
        }
    }

    /// 피드를 그릴 순서 그대로 훑는다.
    ///
    /// 연속 발화 판정에서 **제안 카드는 묶음을 끊는다.** 사이에 카드가 끼면 화면상
    /// 이어진 말이 아니므로, 꼬리와 간격도 거기서 끊겨야 한다.
    private var rows: [Row] {
        let feed = controller.feed
        func role(at index: Int) -> Role? {
            guard feed.indices.contains(index), case let .utterance(u) = feed[index] else { return nil }
            return u.role
        }

        return feed.indices.map { i in
            switch feed[i] {
            case let .suggestion(item):
                .suggestion(item)
            case let .utterance(item):
                .utterance(
                    item,
                    first: role(at: i - 1) != item.role,
                    last: role(at: i + 1) != item.role)
            }
        }
    }
}
