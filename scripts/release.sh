#!/bin/sh
# 배포물을 만든다 — 빌드 · 서명 · 공증 · 검역 검증까지 한 번에.
#
#   scripts/release.sh          0.1.0 으로
#   scripts/release.sh 0.2.0    판을 정해서
#
# 네 단계를 손으로 이으면 다음에 배포할 때 순서를 다시 기억해야 하고,
# 하나를 빠뜨려도 **받는 사람 기계에서만 티가 난다.** 그래서 묶는다.
#
# 준비물이 없으면 무엇을 하면 되는지 말하고 멈춘다. 반쯤 만들어진 배포물을
# 남기지 않는다 — 서명만 되고 공증이 안 된 파일은 겉으로 멀쩡해 보인다.
set -eu

cd "$(dirname "$0")/.."

profile="amcli"        # notarytool 자격 증명 프로필 (키체인)
out="dist/yarrr"
zip="dist/yarrr.zip"

# 판. 태그와 같은 값이어야 한다 — 첫 화면이 이것을 적는다.
version="${1:-0.1.0}"

say() { printf '\n\033[1m%s\033[0m\n' "$1"; }
die() { printf '\n%s\n' "$1" >&2; exit 1; }

# ── 준비물 ──────────────────────────────────────────────────────────
# 빌드부터 하고 마지막에 서명이 없다고 하면 몇 분을 버린다. 먼저 본다.

identity=$(security find-identity -v -p codesigning 2>/dev/null |
	sed -n 's/.*"\(Developer ID Application: [^"]*\)".*/\1/p' | head -1)
[ -n "$identity" ] || die "Developer ID Application 인증서가 없다.
  Xcode › Settings › Accounts › Manage Certificates 에서 만든다."

xcrun notarytool history --keychain-profile "$profile" >/dev/null 2>&1 || die \
"공증 자격 증명이 없다. 한 번만 넣어 두면 그 뒤로는 이 스크립트가 알아서 한다.

  xcrun notarytool store-credentials $profile \\
    --apple-id <애플 ID> --team-id <팀 ID> --password <앱 암호>

앱 암호는 appleid.apple.com › 로그인 및 보안 › 앱 암호 에서 만든다."

# ── 빌드 ────────────────────────────────────────────────────────────
say "1/4  빌드 — 개발자 토큰과 판(v$version)을 박는다"
./scripts/build.sh --version "$version" -o "$out"

# ── 서명 ────────────────────────────────────────────────────────────
say "2/4  서명 — $identity"
# hardened runtime 없이 보내면 공증이 거절한다.
codesign --force --options runtime --timestamp --sign "$identity" "$out"
codesign --verify --strict "$out" || die "서명 검증 실패"

# ── 공증 ────────────────────────────────────────────────────────────
say "3/4  공증 — 애플에 보내고 결과를 기다린다 (보통 1~5분)"
rm -f "$zip"
# ditto 를 쓴다. zip(1) 은 확장 속성을 흘려서 서명이 깨질 수 있다.
# --keepParent 는 쓰지 않는다. 그것을 쓰면 압축 안에 dist/ 가 딸려 들어가서
# 받는 사람이 yarrr 가 아니라 dist/yarrr 를 얻는다.
ditto -c -k "$out" "$zip"
xcrun notarytool submit "$zip" --keychain-profile "$profile" --wait ||
	die "공증 거절. 로그를 본다:
  xcrun notarytool log <submission id> --keychain-profile $profile"

# ── 검증 ────────────────────────────────────────────────────────────
# 여기까지 왔다고 끝이 아니다. **받는 사람 상태로 직접 확인한다.**
# 브라우저로 받으면 검역 표시가 붙는데, 그 상태에서 통과하는지가 진짜 질문이다.
say "4/4  검역 검증 — 브라우저로 받은 상태를 흉내낸다"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
ditto -x -k "$zip" "$tmp"
bin="$tmp/$(basename "$out")"
[ -f "$bin" ] || die "압축 안의 자리가 예상과 다르다: $(ditto -x -k "$zip" "$tmp" 2>&1; find "$tmp" -type f)"
xattr -w com.apple.quarantine "0083;00000000;Safari;" "$bin"
spctl -a -vvv -t install "$bin" 2>&1 | sed 's/^/  /'
spctl -a -t install "$bin" 2>/dev/null || die "검역이 붙으면 막힌다 — 공증이 안 먹었다"

say "됐다"
echo "  $zip  $(ls -lh "$zip" | awk '{print $5}')  ·  v$version"
echo ""
echo "  이 파일을 그대로 보내면 된다. 받는 사람이 할 일은 둘뿐이다."
echo "    · 실행하고 '음악 제어' 권한 승인"
echo "    · (원하면) /ai <key> 로 AI 켜기"
