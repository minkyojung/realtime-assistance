#!/bin/bash
#
# SwiftPM 실행 파일을 Relay.app 번들로 감싼다.
#
# 왜 필요한가 — `swift run RelayPreview` 는 번들이 아닌 맨몸 Mach-O 를 띄운다.
# Info.plist 가 없으면 LaunchServices 가 앱으로 등록하지 않아서
#   · 창 재질·외관이 정상 앱과 다르게 합성되고
#   · TCC 권한(마이크·화면 녹화, Phase 4)을 붙일 주체가 없고
#   · 앱 이름·아이콘·활성화 동작이 제대로 잡히지 않는다.
# UI 를 눈으로 보고 판단하려면 먼저 이걸 정상 앱으로 만들어야 한다.
#
# Xcode 프로젝트를 만들지 않는 이유: 지금 필요한 건 Info.plist 와 번들 껍데기뿐이고,
# `swift build` / `swift test` 를 그대로 유지하는 편이 CI 에도 유리하다.
# 앱 타겟이 실제로 필요해지는 시점(엔타이틀먼트·서명·배포)에 옮긴다.
#
# 사용법:  Scripts/bundle.sh [debug|release]   기본 debug
#         Scripts/bundle.sh --run             빌드 후 실행
set -euo pipefail

cd "$(dirname "$0")/.."

CONFIG=debug
RUN=0
for arg in "$@"; do
    case "$arg" in
        debug|release) CONFIG="$arg" ;;
        --run) RUN=1 ;;
        *) echo "알 수 없는 인자: $arg" >&2; exit 1 ;;
    esac
done

EXECUTABLE=RelayPreview
BUNDLE_ID=com.relay.panel
APP=".build/Relay.app"

swift build -c "$CONFIG" --product "$EXECUTABLE"
BUILT=$(swift build -c "$CONFIG" --show-bin-path)/$EXECUTABLE

# 통째로 지우고 다시 만든다. 이전 실행의 잔여물이 섞이면 서명이 깨진다.
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp "$BUILT" "$APP/Contents/MacOS/$EXECUTABLE"

# 픽스처 재생 모드(RELAY_LIVE=1 이 아닐 때의 기본)가 읽는다.
# 번들에 넣어 두면 앱이 소스 트리 없이도 혼자 돈다.
cp -R Fixtures "$APP/Contents/Resources/"

cat > "$APP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleName</key>                   <string>Relay</string>
    <key>CFBundleDisplayName</key>            <string>Relay</string>
    <key>CFBundleIdentifier</key>             <string>$BUNDLE_ID</string>
    <key>CFBundleExecutable</key>             <string>$EXECUTABLE</string>
    <key>CFBundlePackageType</key>            <string>APPL</string>
    <key>CFBundleInfoDictionaryVersion</key>  <string>6.0</string>
    <key>CFBundleShortVersionString</key>     <string>0.1</string>
    <key>CFBundleVersion</key>                <string>1</string>
    <!-- Liquid Glass·NSGlassEffectView 가 macOS 26 전용이다 (Package.swift 와 같은 하한). -->
    <key>LSMinimumSystemVersion</key>         <string>26.0</string>
    <key>NSHighResolutionCapable</key>        <true/>
</dict>
</plist>
PLIST

# 애드혹 서명. 배포용은 아니지만, 서명이 있어야 번들 신원이 고정돼
# 실행할 때마다 TCC 권한을 다시 묻지 않는다 (Phase 4 에서 필요해진다).
codesign --force --sign - --timestamp=none "$APP" >/dev/null 2>&1

echo "→ $(cd "$(dirname "$APP")" && pwd)/$(basename "$APP")"
[ "$RUN" = 1 ] && open "$APP"
exit 0
