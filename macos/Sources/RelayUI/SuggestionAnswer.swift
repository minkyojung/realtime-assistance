import SwiftUI
import RelayCore

/// 질문에 대한 답. **질문 말풍선 안에서 이어진다.**
///
/// 따로 떠 있는 카드였다가 안으로 들어왔다. 카드였을 때는 화면에 상자가 하나 더
/// 생기고, 그 상자가 어느 말에 대한 것인지 화살표와 인용으로 다시 가리켜야 했다.
/// 같은 말풍선 안에 있으면 그 설명이 전부 필요 없어진다 — 위치가 곧 관계다.
///
/// 다만 안에 있다고 해서 질문과 섞여서는 안 된다. 말풍선 아랫단에 옅은 색 한 겹을
/// 깔아 **자기 구역**을 준다. 색과 모서리는 말풍선(`UtteranceBubble`)이 그린다 —
/// 여기는 그 구역 안의 내용만 맡는다.
///
/// 지연 시간(`latencyMs`)은 보여주지 않는다. 미팅 중에 쓸 일이 없는 개발 정보다.
struct SuggestionAnswer: View {
    let item: SuggestionItem
    /// 답을 요청한 상태. 답 내용은 `item` 이 들고 여기엔 진행 상태만 온다.
    let state: AskState
    /// 실패했을 때 다시 시도. 최초 요청은 말풍선의 버튼이 맡는다.
    let onRetry: () -> Void

    /// 스트리밍 중 커서 깜빡임. `done` 이면 돌지 않는다.
    @State private var cursorOn = true

    var body: some View {
        VStack(alignment: .leading, spacing: 5) {
            headline
            body_
            if let condition = item.condition, !condition.isEmpty { conditionLine(condition) }
            if !item.script.isEmpty {
                SourceBadge(evidence: item.evidence)
                    .padding(.top, 1)
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(verdictLabel)
        .task(id: item.done) {
            guard !item.done else { return cursorOn = false }
            while !Task.isCancelled {
                try? await Task.sleep(for: .milliseconds(500))
                cursorOn.toggle()
            }
        }
    }

    private var headline: some View {
        HStack(alignment: .firstTextBaseline, spacing: 6) {
            replyBadge
            Text(item.headline.isEmpty ? "Waiting for the verdict" : item.headline)
                .font(.system(size: 13, weight: .semibold))
                .fixedSize(horizontal: false, vertical: true)
                .redacted(reason: item.headline.isEmpty ? .placeholder : [])
        }
    }

    /// 답장 표식과 판정을 **한 기호로 합친다.**
    ///
    /// 화살표는 이게 위 질문에 대한 답이라고 말하고, 원의 배경색이 판정을 말한다.
    /// 따로 두면 말풍선 안에 표식이 둘(점 + 화살표)이 되어 시끄러워진다.
    /// 색을 화살표 자체에 칠하지 않고 배경에 두는 이유는, 얇은 획에 색을 주면
    /// 초록·주황이 회색과 잘 구분되지 않기 때문이다.
    private var replyBadge: some View {
        ZStack {
            Circle().fill(verdictColor)
            Image(systemName: "arrow.turn.down.right")
                .font(.system(size: 8, weight: .bold))
                .foregroundStyle(.white)
        }
        .frame(width: 15, height: 15)
        // 첫 줄 텍스트의 baseline 에 맞추면 원이 글자보다 아래로 내려간다.
        .offset(y: -3)
    }

    /// 본문. 답이 오기 전에는 기다리는 이유가, 실패했으면 다시 시도가 그 자리에 온다.
    @ViewBuilder
    private var body_: some View {
        if !item.script.isEmpty {
            script
        } else if case let .failed(message) = state {
            failure(message)
        } else {
            waitingLine
        }
    }

    private var script: some View {
        Text(streamed)
            .font(.system(size: 12))
            .foregroundStyle(.secondary)
            .textSelection(.enabled)
            .fixedSize(horizontal: false, vertical: true)
            // 20ms 마다 오는 delta 에 기본 텍스트 전환(크로스페이드)이 걸리면
            // 조합 중인 한글이 계속 흔들린다. 전환을 끄고 글자만 늘어나게 둔다.
            .contentTransition(.identity)
            .animation(nil, value: item.script)
    }

    /// 문구는 `String` 을 통째로 갈아끼우지 않고 이어 붙인다.
    private var streamed: AttributedString {
        var text = AttributedString(item.script)
        if !item.done, cursorOn {
            var cursor = AttributedString("▋")
            cursor.foregroundColor = .secondary
            text += cursor
        }
        return text
    }

    /// 기다리는 몇 초가 빈 시간이 되면 안 된다.
    ///
    /// 승인된 답이 없는 질문(escalate)에서는 지금 입으로 낼 문장을 먼저 준다 —
    /// 모델을 기다리지 않는, 정해진 한 줄이다. 나머지 판정은 승인 문구가 곧바로
    /// 나오므로 "확인해서 알려주겠다"고 말하면 사실이 아니게 된다.
    private var waitingLine: some View {
        HStack(spacing: 6) {
            ProgressView()
                .controlSize(.small)
                .scaleEffect(0.55)
                .frame(width: 12, height: 12)
            Text(item.mode == .escalate
                 ? "\u{201C}I'll check and get right back to you.\u{201D}"
                 : "Looking it up\u{2026}")
        }
        .font(.system(size: 12))
        .foregroundStyle(.secondary)
    }

    private func failure(_ message: String) -> some View {
        HStack(spacing: 6) {
            Text("Couldn't fetch the answer")
            Button("Retry", action: onRetry)
                .buttonStyle(.borderless)
                .controlSize(.small)
        }
        .font(.system(size: 11))
        .foregroundStyle(.secondary)
        .accessibilityLabel("Couldn't fetch the answer: \(message)")
    }

    /// 조건부 판정의 단서. 이걸 못 보고 말하면 판정이 없느니만 못하다.
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

    /// 판정 전에는 무채색이다 — 아직 색으로 말할 것이 없다.
    private var verdictColor: Color {
        switch item.mode {
        case .direct:      .green
        case .conditional: .orange
        case .escalate:    .red
        case nil:          .secondary
        }
    }

    /// 화면에서는 판정을 라벨로 말하지 않으므로(색 하나뿐이다) 이름은 접근성 쪽에 남긴다.
    private var verdictLabel: String {
        switch item.mode {
        case .direct:      "Safe to answer suggestion"
        case .conditional: "Conditional suggestion"
        case .escalate:    "Needs verification suggestion"
        case nil:          "Waiting for verdict"
        }
    }
}
