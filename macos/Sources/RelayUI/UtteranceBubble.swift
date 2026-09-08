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

    public init(_ item: UtteranceItem, isLastInRun: Bool = true) {
        self.item = item
        self.isLastInRun = isLastInRun
    }

    private var isCounterpart: Bool { item.role == .counterpart }

    public var body: some View {
        Text(item.text)
            .font(.system(size: 14))
            .foregroundStyle(textColor)
            .padding(.horizontal, 12)
            .padding(.vertical, 8)
            .background(background)
            .frame(maxWidth: 320, alignment: isCounterpart ? .leading : .trailing)
            .frame(maxWidth: .infinity, alignment: isCounterpart ? .leading : .trailing)
            .animation(.easeOut(duration: 0.15), value: item.text)
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
