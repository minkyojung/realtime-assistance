import SwiftUI
import RelayCore

/// 제안. 대화 흐름 안에, 질문 말풍선 바로 아래에 선다.
///
/// **말풍선과 같은 언어를 쓰되 같은 것으로 보이지는 않게 한다.**
/// 폭·모서리·여백은 말풍선을 따르고, 위치는 내 쪽(오른쪽)이다 — 나에게 주는 말이니까.
/// 다만 강조색 단색(내가 실제로 한 말)이 아니라 **중립 유리**다. 대화 위에 떠 있는
/// 다른 층이라는 것이 재질로만 구분되고, 꼬리는 달지 않는다 — 아무도 하지 않은 말이다.
///
/// **색을 쓰지 않는다.** Apple 은 색을 상태(경고·오류·진행)에만 쓰고, "이 내용이 어떤
/// 종류인가"를 색으로 칠하지 않는다 — Mail 의 요약도 Writing Tools 의 제안도 무채색
/// 평문이다. 판정 3색을 배경에 깔아 봤더니 카드가 화면을 지배하고 대화가 밀렸다(실측).
/// 판정은 headline 과 조건 문구가 글로 말한다.
///
/// 지연 시간(`latencyMs`)도 보여주지 않는다. 미팅 중에 쓸 일이 없는 개발 정보다.
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
        VStack(alignment: .leading, spacing: 5) {
            if let replyingTo, !replyingTo.isEmpty { quoted(replyingTo) }
            headline
            script
            if let condition = item.condition, !condition.isEmpty { conditionLine(condition) }
            SourceBadge(evidence: item.evidence)
                .padding(.top, 1)
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 9)
        .frame(maxWidth: 340, alignment: .leading)
        .glassEffect(.regular, in: bubbleShape)
        .accessibilityElement(children: .contain)
        .accessibilityLabel(verdictLabel)
        .frame(maxWidth: .infinity, alignment: .trailing)
        .task(id: item.done) {
            guard !item.done else { return cursorOn = false }
            while !Task.isCancelled {
                try? await Task.sleep(for: .milliseconds(500))
                cursorOn.toggle()
            }
        }
    }

    /// 말풍선과 같은 반경. 다만 꼬리는 없다 — 발화가 아니기 때문이다.
    private var bubbleShape: RoundedRectangle { RoundedRectangle(cornerRadius: 14) }

    /// 화면에서는 판정을 색이나 라벨로 말하지 않으므로, 이름은 접근성 쪽에 남긴다.
    private var verdictLabel: String {
        switch item.mode {
        case .direct:      "Safe to answer suggestion"
        case .conditional: "Conditional suggestion"
        case .escalate:    "Needs verification suggestion"
        case nil:          "Waiting for verdict"
        }
    }

    // MARK: - 조각

    /// 어떤 말에 대한 제안인지 — iMessage 의 답장 인용과 같은 자리, 같은 문법이다.
    /// 원문은 바로 위 말풍선에 그대로 있으므로 여기서는 한 줄로 줄인다.
    private func quoted(_ text: String) -> some View {
        HStack(spacing: 5) {
            // 위 발화에서 돌아 나와 이 카드로 들어온다는 뜻. 머리가 오른쪽을 가리킨다.
            Image(systemName: "arrow.uturn.right")
                .font(.system(size: 9, weight: .semibold))
            Text(text)
                .lineLimit(1)
                .truncationMode(.tail)
        }
        .font(.system(size: 10))
        .foregroundStyle(.tertiary)
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
            .foregroundStyle(.secondary)
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
    /// 상자로 감싸지 않는다 — 카드 안에 또 상자를 넣으면 그때부터 시끄러워진다.
    private func conditionLine(_ condition: String) -> some View {
        HStack(alignment: .top, spacing: 4) {
            Image(systemName: "exclamationmark.triangle")
                .font(.system(size: 9))
            Text(condition)
                .fixedSize(horizontal: false, vertical: true)
        }
        .font(.system(size: 11))
        .foregroundStyle(.secondary)
    }
}
