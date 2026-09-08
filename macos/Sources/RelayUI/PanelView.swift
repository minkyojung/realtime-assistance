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
/// 여기서는 그리기만 한다. 제안 카드(`SuggestionCard`)는 아직 없어서 피드에 있어도 건너뛴다.
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
                ForEach(utterances, id: \.item.id) { entry in
                    UtteranceBubble(entry.item, isLastInRun: entry.last)
                        .padding(.top, entry.first ? 12 : 3)
                }
            }
            .frame(maxWidth: .infinity)
            .padding(.horizontal, 14)
            .padding(.top, 56)    // pill 자리
            .padding(.bottom, 16)
        }
        .scrollContentBackground(.hidden)
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

    private struct Entry {
        let item: UtteranceItem
        /// 연속 발화의 첫 줄인가 — 위 간격을 벌린다.
        let first: Bool
        /// 연속 발화의 마지막 줄인가 — 꼬리를 단다.
        let last: Bool
    }

    /// 피드에서 발화만 뽑고, 같은 화자의 연속 발화를 한 묶음으로 본다.
    private var utterances: [Entry] {
        let items = controller.feed.compactMap { item -> UtteranceItem? in
            if case let .utterance(u) = item { u } else { nil }
        }
        return items.indices.map { i in
            Entry(
                item: items[i],
                first: i == 0 || items[i - 1].role != items[i].role,
                last: i == items.count - 1 || items[i + 1].role != items[i].role
            )
        }
    }
}
