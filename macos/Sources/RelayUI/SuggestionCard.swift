import SwiftUI
import RelayCore

/// 제안 카드. 대화 흐름 안에, 질문 말풍선 바로 아래에 선다.
///
/// **말풍선처럼 그리지 않는다.** 버블은 "누군가 이 말을 했다"는 뜻인데 제안은
/// 아무도 하지 않은 말이다 — 앱이 나한테만 건네는 것이다. 그래서 문법을 알림 배너
/// 쪽에서 빌린다: 유리, 떠 있음, 꼬리 없음, 오른쪽으로 살짝 들여쓰기.
/// 카드가 유리라서 뒤의 말풍선이 실제로 굴절돼 비친다 — "다른 층"이라는 게 설명 없이
/// 읽히므로 웹에서 쓰던 점선 테두리(`border-dashed`)는 필요 없다.
///
/// **색이 글보다 먼저 온다.** 미팅 중엔 "말해도 되나?"가 "뭐라고 말하지?"보다 급하다.
/// 서버도 그 순서로 보낸다 — `question.verdict`(판정) → `question.delta`(문구).
/// 카드는 그 순서를 시각적으로 지킨다: 유리가 먼저 물들고, 문장은 나중에 차오른다.
///
/// 원본: `app/src/components/panel/suggestion-card.tsx`
public struct SuggestionCard: View {
    private let item: SuggestionItem
    /// 어떤 말에 대한 제안인지. iMessage 의 답장 인용과 같은 역할이다.
    private let replyingTo: String?
    /// 스트리밍 중 커서 깜빡임. `done` 이면 돌지 않는다.
    @State private var cursorOn = true

    public init(_ item: SuggestionItem, replyingTo: String? = nil) {
        self.item = item
        self.replyingTo = replyingTo
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 7) {
            if let replyingTo, !replyingTo.isEmpty { quoted(replyingTo) }
            header
            headline
            script
            if let condition = item.condition, !condition.isEmpty { conditionRow(condition) }
            footer
        }
        .padding(12)
        .frame(maxWidth: 420, alignment: .leading)
        // 판정 색은 유리 전체의 틴트로 들어간다. 웹은 왼쪽 4px 바였는데(`border-l-4`),
        // 곁눈질로 읽혀야 하는 정보라 면적을 키웠다.
        //
        // 모서리는 숫자로 박지 않고 ConcentricRectangle 로 부모(창 30pt)와 동심을
        // 맞춘다. 다만 카드가 창 가장자리에서 멀면 반경이 지나치게 작아지므로 하한을 둔다.
        .glassEffect(glass, in: ConcentricRectangle(corners: .concentric(minimum: .fixed(12))))
        .frame(maxWidth: .infinity, alignment: .trailing)
        // 나에게 주는 제안이라 내 말풍선과 같은 쪽에 선다. 살짝 들여써서 대화보다 앞에 뜬 층임을 보탠다.
        .padding(.leading, 24)
        .task(id: item.done) {
            guard !item.done else { return cursorOn = false }
            while !Task.isCancelled {
                try? await Task.sleep(for: .milliseconds(500))
                cursorOn.toggle()
            }
        }
    }

    // MARK: - 판정

    private struct Mode {
        let label: String
        let color: Color
    }

    /// 판정 전(`nil`)은 색을 쓰지 않는다 — 아직 아무것도 정해지지 않았다는 뜻이다.
    private var mode: Mode? {
        switch item.mode {
        case .direct:      Mode(label: "Safe to answer", color: .green)
        case .conditional: Mode(label: "Answer with condition", color: .orange)
        case .escalate:    Mode(label: "Verify first", color: .red)
        case nil:          nil
        }
    }

    private var glass: Glass {
        guard let mode else { return .regular }
        return .regular.tint(mode.color)
    }

    // MARK: - 조각

    /// 색만으로 판정을 알리지 않는다. 점 옆의 글자가 같은 정보를 문자로 반복한다.
    private var header: some View {
        HStack(spacing: 6) {
            Circle()
                .fill(mode?.color ?? Color.secondary)
                .frame(width: 6, height: 6)
            Text(mode?.label ?? "Assessing")
                .font(.system(size: 11, weight: .medium))
                .foregroundStyle(.secondary)
            Spacer(minLength: 0)
        }
        .accessibilityElement(children: .combine)
        .accessibilityLabel(mode.map { "\($0.label) suggestion" } ?? "Waiting for verdict")
    }

    /// 어떤 말에 대한 제안인지 — iMessage 의 답장 인용과 같은 자리, 같은 문법이다.
    /// 카드가 대화에서 떨어져 있어도 무엇에 답하는지가 즉시 읽힌다.
    /// 원문은 바로 위 말풍선에 그대로 있으므로 여기서는 한 줄로 줄인다.
    private func quoted(_ text: String) -> some View {
        HStack(spacing: 6) {
            Capsule()
                .fill(.tertiary)
                .frame(width: 2)
            Text(text)
                .font(.system(size: 10))
                .foregroundStyle(.tertiary)
                .lineLimit(1)
                .truncationMode(.tail)
        }
        .fixedSize(horizontal: false, vertical: true)
        .frame(height: 14)
        .accessibilityLabel("Replying to: \(text)")
    }

    private var headline: some View {
        // 판정 전에는 자리만 잡아 둔다. 뷰를 갈아끼우지 않고 `.redacted` 로 가리므로
        // 판정이 도착해 걷힐 때 레이아웃이 튀지 않는다.
        Text(item.headline.isEmpty ? "Waiting for the verdict" : item.headline)
            .font(.system(size: 13, weight: .semibold))
            .fixedSize(horizontal: false, vertical: true)
            .redacted(reason: item.headline.isEmpty ? .placeholder : [])
    }

    private var script: some View {
        Text(streamed)
            .font(.system(size: 12))
            .foregroundStyle(.primary.opacity(0.9))
            .textSelection(.enabled)
            .fixedSize(horizontal: false, vertical: true)
            .redacted(reason: item.script.isEmpty ? .placeholder : [])
            // 20ms 마다 오는 delta 에 기본 텍스트 전환(크로스페이드)이 걸리면
            // 조합 중인 한글이 계속 흔들린다. 전환을 끄고 글자만 늘어나게 둔다.
            .contentTransition(.identity)
            .animation(nil, value: item.script)
    }

    /// 문구는 `String` 을 통째로 갈아끼우지 않고 이어 붙인다.
    private var streamed: AttributedString {
        guard !item.script.isEmpty else { return AttributedString("Preparing the suggested reply") }

        var text = AttributedString(item.script)
        if !item.done, cursorOn {
            var cursor = AttributedString("▋")
            cursor.foregroundColor = .secondary
            text += cursor
        }
        return text
    }

    /// 조건부 판정의 단서. 이걸 못 보고 말하면 판정이 없느니만 못하다.
    private func conditionRow(_ condition: String) -> some View {
        HStack(alignment: .top, spacing: 5) {
            Image(systemName: "exclamationmark.triangle.fill")
                .font(.system(size: 9))
            Text(condition)
                .fixedSize(horizontal: false, vertical: true)
        }
        .font(.system(size: 11))
        .foregroundStyle(.secondary)
        .padding(.horizontal, 8)
        .padding(.vertical, 5)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(.quaternary, in: .rect(cornerRadius: 7))
    }

    private var footer: some View {
        HStack(spacing: 6) {
            SourceBadge(evidence: item.evidence)
            Spacer(minLength: 8)
            if let latency = item.latencyMs {
                Text("\(latency)ms")
                    .font(.system(size: 9))
                    .foregroundStyle(.tertiary)
                    .monospacedDigit()
            }
        }
    }
}
