// swift-tools-version: 6.2
import PackageDescription

/// Relay 화면 1(실시간 어시스트 패널)의 코어.
///
/// UI 없이 SSE 수신과 피드 상태만 담당한다. Xcode 없이 `swift test` 로 검증하기 위해
/// 앱 타겟과 분리했다. 앱 타겟(`Relay`)은 Phase 2 에서 추가한다.
let package = Package(
    name: "RelayCore",
    // Liquid Glass(.glassEffect)·NSGlassEffectView 가 macOS 26 전용이다.
    platforms: [.macOS(.v26)],
    products: [
        .library(name: "RelayCore", targets: ["RelayCore"]),
        .library(name: "RelayUI", targets: ["RelayUI"]),
    ],
    targets: [
        .target(name: "RelayCore", swiftSettings: [.swiftLanguageMode(.v6)]),
        .target(
            name: "RelayUI",
            dependencies: ["RelayCore"],
            swiftSettings: [.swiftLanguageMode(.v6)],
        ),
        // `swift run RelayPreview` — 만든 컴포넌트를 창에 띄워 눈으로 본다.
        .executableTarget(
            name: "RelayPreview",
            dependencies: ["RelayUI", "RelayCore"],
            swiftSettings: [.swiftLanguageMode(.v6)],
        ),
        .testTarget(
            name: "RelayCoreTests",
            dependencies: ["RelayCore"],
            swiftSettings: [.swiftLanguageMode(.v6)],
        ),
        .testTarget(
            name: "RelayUITests",
            dependencies: ["RelayUI", "RelayCore"],
            swiftSettings: [.swiftLanguageMode(.v6)],
        ),
    ]
)
