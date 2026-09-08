import SwiftUI
import RelayCore

/// 지금 상태를 보여주는 떠 있는 pill. 세션 정보가 아니라 **상태**를 담는다.
///
/// 정보는 3개 이하로 유지한다 — 한눈에 안 들어오면 pill 의 의미가 없다.
/// `NDA 미체결` 은 표시가 아니라 안전 스위치다. 이 값이 판정을 바꾼다.
/// 원본: `app/src/app/panel/session-header.tsx`
public struct SessionPill: View {
    /// nil = 아직 세션이 없다. 상대·NDA 자리를 비워 둔다.
    private let session: Session?
    private let elapsed: Int
    /// 캡처가 살아 있는가. 점 색이 이걸 따른다.
    private let running: Bool

    public init(session: Session?, elapsed: Int, running: Bool) {
        self.session = session
        self.elapsed = elapsed
        self.running = running
    }

    public var body: some View {
        HStack(spacing: 8) {
            // 색만으로 상태를 알리지 않도록 접근성 라벨을 함께 둔다.
            Circle()
                .fill(running ? Color.red : Color.secondary.opacity(0.5))
                .frame(width: 7, height: 7)
                .accessibilityLabel(running ? "녹음 중" : "녹음 정지")

            Text(Self.formatElapsed(elapsed))
                .monospacedDigit()

            if let session {
                Text("·").foregroundStyle(.tertiary)
                Text(session.counterpartOrg)

                Text(session.context.ndaSigned ? "NDA 체결" : "NDA 미체결")
                    .font(.caption)
                    .padding(.horizontal, 7)
                    .padding(.vertical, 2)
                    .background(.quaternary, in: .capsule)
            }
        }
        .font(.system(size: 12))
        .padding(.horizontal, 14)
        .padding(.vertical, 9)
        // 유리는 뷰 자신에 입힌다. `.background { Color.clear.glassEffect() }` 로 감싸면
        // 배경 레이어가 따로 그려져 굴절 대신 평평한 틴트만 남는다.
        // 뒤가 비치는 창(`PanelWindow` 의 NSVisualEffectView) 위에서는 `.clear` 가
        // `.regular` 보다 림 하이라이트가 살아서 유리로 읽힌다.
        .glassEffect(.clear, in: .capsule)
    }

    /// mm:ss, 한 시간이 넘으면 h:mm:ss. 웹 `formatElapsed` 와 같다.
    static func formatElapsed(_ total: Int) -> String {
        let hours = total / 3600
        let minutes = total / 60 % 60
        let seconds = total % 60
        return hours > 0
            ? String(format: "%d:%02d:%02d", hours, minutes, seconds)
            : String(format: "%02d:%02d", minutes, seconds)
    }
}
