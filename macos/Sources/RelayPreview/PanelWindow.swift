import AppKit
import SwiftUI
import RelayCore
import RelayUI

/// 화면 1 패널 창.
///
/// **창 배경을 절대 투명하게 만들지 않는다.** `isOpaque = false` + `backgroundColor = .clear`
/// 를 주면 시스템이 창 그리기를 멈추고, 그때부터 모서리·그림자를 직접 그려야 한다.
/// macOS 26 에는 창 모서리 반경을 정하는 API 가 없다 — 시스템이 그린 모서리 *안쪽으로*
/// 배치하는 API(`NSViewLayoutRegion`, `safeAreaBar`, `ConcentricRectangle`)만 있다.
/// 투명하게 만드는 순간 그 API 들이 전부 무의미해진다.
///
/// Liquid Glass 는 창 배경이 아니라 **콘텐츠 위에 떠 있는 레이어**의 것이다.
/// (Apple: "Liquid Glass applies to the topmost layer of the interface, where you
/// define your navigation.") 창 전면의 유리도 그 규칙을 그대로 따른다 — 유리는
/// 항상 뒤가 비치는 무언가 **위에** 있어야 한다.
///
/// 다만 배경을 시스템에 맡기면(contentView = hosting) 창이 **불투명 단색**으로 그려진다.
/// 유리는 뒤 픽셀을 굴절시켜 보이는 재질이라 단색 위에서는 아무것도 안 보인다 — 실제로
/// pill 이 통째로 사라졌다. 그래서 `NSVisualEffectView` 로 뒤가 비치게 만든다.
/// contentView 로 놓으면 AppKit 이 창 합성만 조정하고 프레임은 그대로 둔다 —
/// `isOpaque = false` 없이 반투명을 얻는 유일한 방법이다.
@MainActor
final class PanelWindow {
    let panel: NSPanel
    /// 세션 상태. 창과 수명이 같다.
    let controller: SessionController

    /// 기본은 **픽스처 재생**이다. 미리보기 실행기의 일은 화면을 보는 것이고,
    /// 그러자고 매번 docker + `pnpm dev` + OpenAI 를 띄우게 하면 그 일이 안 된다.
    /// 실서버를 봐야 할 때만 `RELAY_LIVE=1` 로 켠다 (테스트의 실서버 스위트와 같은 이름).
    static var isLive: Bool { ProcessInfo.processInfo.environment["RELAY_LIVE"] == "1" }

    /// 어느 쪽 가장자리에 붙일지. 미팅 앱을 반대쪽에 두는 사람도 있어서 한 줄로 뒤집는다.
    enum Edge { case left, right }
    static let edge: Edge = .right

    /// 가장자리 여백. 창 그림자가 화면 밖으로 잘리지 않을 만큼만 띄운다.
    private static let margin: CGFloat = 16

    /// 폭만 고정한다. 높이는 화면에 맞춰 `positionAtEdge()` 가 늘린다 —
    /// 제안 카드가 들어오면 세로로 길어지고, 그걸 담는 게 이 패널의 형태다.
    init(size: NSSize = NSSize(width: 380, height: 720)) {
        if Self.isLive {
            controller = SessionController()
        } else {
            let session = FixtureReplay.urlSession()
            controller = SessionController(
                api: RelayAPI(session: session), client: SSEClient(session: session))
        }

        panel = NSPanel(
            contentRect: NSRect(origin: .zero, size: size),
            // .nonactivatingPanel — 패널을 눌러도 뒤 앱(브라우저)이 활성 상태를 유지한다.
            styleMask: [.nonactivatingPanel, .titled, .closable, .resizable, .fullSizeContentView],
            backing: .buffered,
            defer: false
        )

        panel.titlebarAppearsTransparent = true
        panel.titleVisibility = .hidden
        panel.isMovableByWindowBackground = true

        // Tahoe 의 큰 모서리 반경(약 30pt)은 **툴바가 있는 창에만** 적용된다.
        // 툴바가 없으면 레거시 반경(약 18pt)이 그려진다 — 실측으로 확인했다.
        // 항목은 없어도 되고, titlebarAppearsTransparent + fullSizeContentView 라
        // 눈에 보이는 바도, 프레임 높이 증가도 없다(contentRect == frame 유지).
        panel.toolbar = NSToolbar()

        // 패널에는 신호등이 없다. 닫기·숨기기는 전역 단축키(⌘\)가 맡는다.
        for button in [NSWindow.ButtonType.closeButton, .miniaturizeButton, .zoomButton] {
            panel.standardWindowButton(button)?.isHidden = true
        }

        panel.isFloatingPanel = true
        panel.becomesKeyOnlyIfNeeded = true
        panel.level = .floating

        // NSPanel 은 앱이 비활성화되면 기본으로 숨는다. 미팅 내내 떠 있어야 하므로 끈다.
        panel.hidesOnDeactivate = false

        // 창 레벨이 normal 이 아니면 collectionBehavior 기본값이 .transient 가 되고,
        // .transient 는 정의상 Exposé(F3)에서 숨겨진다. 명시적으로 덮어쓴다.
        //   .stationary        Exposé 영향 없음 — 계속 보인다
        //   .canJoinAllSpaces  스페이스를 옮겨도 따라온다
        //   .fullScreenAuxiliary  전체화면 앱 위에도 뜬다
        panel.collectionBehavior = [.canJoinAllSpaces, .stationary, .fullScreenAuxiliary]

        let hosting = NSHostingView(rootView: PanelView(controller: controller))
        // 타이틀바를 숨겼으므로 안전 영역으로 콘텐츠를 밀어낼 이유가 없다.
        hosting.safeAreaRegions = []

        // 데스크톱을 통과시키는 건 NSVisualEffectView(.behindWindow) 뿐이다.
        // NSGlassEffectView 는 **창 뒤를 샘플링하지 않는다** — 같은 창 안에서 자기 뒤에
        // 그려진 픽셀만 굴절시킨다.
        //
        // **그래서 여기에 유리를 얹지 않는다.** 헤더가 그대로 말한다 —
        // "A view that embeds its *content view* in a dynamic glass effect."
        // 창 전체를 유리로 덮으면 굴절할 대상이 자기 자식(= 우리 콘텐츠)뿐이라
        // 굴절이 죽고 밋밋한 틴트만 남는다. 게다가 그 위에 올린 pill 의
        // `.glassEffect()` 는 유리 위의 유리가 되어 이중 블러로 뭉갠다.
        //
        // 유리는 배경의 재질이 아니라 **콘텐츠 위에 떠 있는 최상위 레이어**의 재질이다.
        // 창은 여기까지(반투명 재질)만 맡고, 유리는 전부 SwiftUI 쪽에서 개별 요소
        // (pill · 제안 카드 · 컨트롤)에 `.glassEffect()` 로 붙인다.
        let backdrop = NSVisualEffectView()
        backdrop.material = .underWindowBackground
        backdrop.blendingMode = .behindWindow
        backdrop.state = .active

        backdrop.addSubview(hosting)
        hosting.translatesAutoresizingMaskIntoConstraints = false
        NSLayoutConstraint.activate([
            hosting.topAnchor.constraint(equalTo: backdrop.topAnchor),
            hosting.bottomAnchor.constraint(equalTo: backdrop.bottomAnchor),
            hosting.leadingAnchor.constraint(equalTo: backdrop.leadingAnchor),
            hosting.trailingAnchor.constraint(equalTo: backdrop.trailingAnchor),
        ])
        panel.contentView = backdrop
    }

    func show() {
        positionAtEdge()
        panel.orderFrontRegardless()
    }

    /// 화면 한쪽 가장자리에 세로로 붙인다.
    ///
    /// 상단 가로 배치도 검토했지만(카메라 근처라 시선이 덜 튄다), 피드에 제안 카드가
    /// 들어오면 콘텐츠가 세로로 길어져서 가로 밴드에는 애초에 담기지 않는다.
    /// 읽는 시간이 길어지는 이상 시선 이점은 사라지고, 남는 건 담을 높이뿐이다.
    private func positionAtEdge() {
        guard let screen = panel.screen ?? NSScreen.main else { return }
        let visible = screen.visibleFrame
        let margin = Self.margin

        // 화면 세로를 거의 다 쓰되, 기본 높이보다 크게는 늘리지 않는다.
        let height = min(panel.frame.height, visible.height - margin * 2)
        let width = panel.frame.width
        let x = Self.edge == .right ? visible.maxX - width - margin : visible.minX + margin
        let y = visible.maxY - margin - height

        panel.setFrame(
            NSRect(x: x.rounded(), y: y.rounded(), width: width, height: height.rounded()),
            display: false)
    }
}
