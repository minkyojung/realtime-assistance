import AppKit

/// 패널 미리보기 실행기. 제품 앱이 되면 여기에 세션 시작·전역 단축키가 붙는다.
@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    private var window: PanelWindow?

    func applicationDidFinishLaunching(_ notification: Notification) {
        let window = PanelWindow()
        window.show()
        self.window = window
    }
}

let app = NSApplication.shared
app.setActivationPolicy(.regular)
let delegate = AppDelegate()
app.delegate = delegate
app.run()
