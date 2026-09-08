import SwiftUI
import RelayCore

/// 발화 하나. 고객은 왼쪽 반투명, 나는 오른쪽 강조색.
///
/// 강조색은 시스템 설정(일반 → 강조 색상)을 따른다.
///
/// 꼬리는 iMessage 처럼 그리지 않고 **모서리 반경으로 표현**한다.
/// 연속 발화 중 마지막 것만 화자 쪽 아래 모서리가 각져서 방향이 읽힌다.
public struct UtteranceBubble: View {
    private let item: UtteranceItem
    private let isLastInRun: Bool
    /// 이 말이 질문일 때의 답. **같은 말풍선 안에서 이어진다.**
    ///
    /// 따로 뜨는 카드가 아닌 이유 — 카드는 화면에 상자를 하나 더 만들고, 그게 어느
    /// 말에 대한 것인지 화살표와 인용으로 다시 가리켜야 했다. 말 안에 들어오면
    /// 그 설명이 전부 필요 없어진다.
    private let answer: SuggestionItem?
    private let askState: AskState
    /// 답을 찾아 달라는 트리거. 아직 요청하지 않았을 때만 온다.
    private let onAsk: (() -> Void)?
    private let onRetry: () -> Void

    public init(
        _ item: UtteranceItem, isLastInRun: Bool = true,
        answer: SuggestionItem? = nil, askState: AskState = .idle,
        onAsk: (() -> Void)? = nil, onRetry: @escaping () -> Void = {}
    ) {
        self.item = item
        self.isLastInRun = isLastInRun
        self.answer = answer
        self.askState = askState
        self.onAsk = onAsk
        self.onRetry = onRetry
    }

    private var isCounterpart: Bool { item.role == .counterpart }

    /// 요청하기 전에는 답이 화면에 없다. 누른 뒤에야 말풍선이 아래로 자란다.
    private var shownAnswer: SuggestionItem? {
        guard let answer, !answer.script.isEmpty || askState != .idle else { return nil }
        return answer
    }

    public var body: some View {
        HStack(alignment: .center, spacing: 4) {
            bubble
            if let onAsk { askButton(onAsk) }
        }
        // 폭 제한을 말풍선이 아니라 여기에 건다. 말풍선에 걸면 글이 짧아도 프레임은
        // 320pt 를 차지해서, 버튼이 글 옆이 아니라 저 멀리 오른쪽 끝에 선다.
        // HStack 은 자식 크기만큼만 차지하므로 버튼이 말풍선에 붙어 따라온다.
        .frame(maxWidth: shownAnswer == nil ? 320 : .infinity,
               alignment: isCounterpart ? .leading : .trailing)
        .frame(maxWidth: .infinity, alignment: isCounterpart ? .leading : .trailing)
    }

    private var bubble: some View {
        VStack(alignment: .leading, spacing: 0) {
            Text(item.text)
                .font(.system(size: 14))
                .foregroundStyle(textColor)
                .animation(.easeOut(duration: 0.15), value: item.text)
            if let shownAnswer {
                SuggestionAnswer(item: shownAnswer, state: askState, onRetry: onRetry)
            }
        }
        // 답이 붙으면 문단이 들어오므로 말풍선이 폭을 다 쓴다. 짧은 말 하나일 때는
        // 제한을 걸지 않아야 글자만큼만 차지한다(nil = 제한 없음).
        .frame(maxWidth: shownAnswer == nil ? nil : .infinity, alignment: .leading)
        .fixedSize(horizontal: false, vertical: true)
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
        .background(background)
    }

    /// 말풍선에 붙는 원. 무채색이라 대화를 방해하지 않고, 세로는 말풍선 중앙에 온다 —
    /// 두 줄짜리 말이든 한 줄짜리 말이든 눈이 같은 자리에서 찾는다.
    private func askButton(_ action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Image(systemName: "arrow.right.circle.fill")
                .font(.system(size: 20))
                .foregroundStyle(.secondary)
                .symbolRenderingMode(.hierarchical)
        }
        .buttonStyle(.plain)
        .accessibilityLabel("Look up an answer")
    }

    /// 고객은 반투명(유리가 비침), 나는 강조색 단색.
    private var background: some View {
        bubbleShape.fill(
            isCounterpart ? Color.primary.opacity(0.07) : Color(nsColor: .controlAccentColor))
    }

    /// 강조색 위에서는 흰 글자. macOS 의 강조색 버튼과 같은 처리다.
    private var textColor: Color {
        if isCounterpart {
            item.isFinal ? .primary : .secondary
        } else {
            .white.opacity(item.isFinal ? 1 : 0.65)
        }
    }

    /// 확정 전에는 꼬리를 달지 않는다 — 아직 그 화자의 마지막 말이 아닐 수 있다.
    private var hasTail: Bool { isLastInRun && item.isFinal }

    private var bubbleShape: UnevenRoundedRectangle {
        let round: CGFloat = 14
        let tail: CGFloat = 4
        return UnevenRoundedRectangle(
            topLeadingRadius: round,
            bottomLeadingRadius: hasTail && isCounterpart ? tail : round,
            bottomTrailingRadius: hasTail && !isCounterpart ? tail : round,
            topTrailingRadius: round
        )
    }
}
