import SwiftUI
import RelayCore

/// 출처 한 줄.
///
/// 미팅 중에 필요한 것은 근거를 '읽는' 것이 아니라 '확인'하는 것이다.
/// 펼침은 스크롤을 밀어 대화 흐름을 놓치게 하므로 쓰지 않는다. 출처 종류와 시점을
/// 항상 노출하고, 더 보고 싶으면 원문으로 이동한다.
/// 원본: `app/src/components/panel/source-badge.tsx`
struct SourceBadge: View {
    let evidence: [Evidence]

    /// DB 의 `source_type` 을 사람 말로. 모르는 값은 원문 그대로 보여준다.
    private static let label = [
        "release": "Release",
        "changelog": "Changelog",
        "merged_pr": "PR",
        "open_issue": "Issue",
        "milestone": "Milestone",
    ]

    var body: some View {
        if let top = evidence.first {
            Link(destination: URL(string: top.sourceURL) ?? URL(string: "about:blank")!) {
                HStack(spacing: 5) {
                    Text("\(Self.label[top.sourceType] ?? top.sourceType) \(top.externalRef)")
                        .padding(.horizontal, 6)
                        .padding(.vertical, 2)
                        .background(.quaternary, in: .capsule)

                    // 언제 기준의 정보인지. 오래된 근거를 그대로 말하는 게 가장 위험하다.
                    Text(top.validFrom.prefix(7))
                    if evidence.count > 1 { Text("· +\(evidence.count - 1) more") }
                    Image(systemName: "arrow.up.forward")
                }
                .font(.system(size: 10))
                .foregroundStyle(.secondary)
            }
            .buttonStyle(.plain)
            .pointerStyle(.link)
        }
    }
}
