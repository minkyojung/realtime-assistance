#!/bin/sh
# 배포용 바이너리를 만든다.
#
# 배포물에 p8 을 넣지 않는다. 넣으면 누구나 뽑아서 우리 명의로 API 를 쓴다.
# 대신 그 키로 **미리 서명해 둔 개발자 토큰**을 박는다 — 유효기간이 최대
# 여섯 달이고, 새더라도 만료되면 끝이다.
#
#   scripts/build.sh                  토큰을 박아 배포물을 만든다
#   scripts/build.sh --dev            토큰 없이. 각자 자기 설정으로 돈다
#   scripts/build.sh -o /tmp/yarrr    나갈 자리를 정한다
#   scripts/build.sh --version 0.1.0  첫 화면에 적힐 판
#
# 받는 사람이 해야 하는 것은 둘뿐이다.
#   · Music.app 자동화 권한 허용 (앱이 안내한다)
#   · /ai <key> 로 AI 켜기 (건너뛰면 검색·재생만)
set -eu

cd "$(dirname "$0")/.."

out="dist/yarrr"
dev=""
version=""
while [ $# -gt 0 ]; do
	case "$1" in
	--dev) dev=1 ;;
	-o) shift; out="$1" ;;
	--version) shift; version="$1" ;;
	*) echo "usage: $0 [--dev] [--version X.Y.Z] [-o path]" >&2; exit 2 ;;
	esac
	shift
done

mkdir -p "$(dirname "$out")"
# go build 는 tui/ 안에서 도므로 나갈 자리를 절대 경로로 바꿔 둔다.
case "$out" in
/*) ;;
*) out="$(cd "$(dirname "$out")" && pwd)/$(basename "$out")" ;;
esac

ldflags=""

if [ -z "$dev" ]; then
	echo "개발자 토큰을 서명하는 중…"
	# amtoken 은 설정에서 p8·Key ID·Team ID 를 찾는다. 없으면 무엇이
	# 없는지 자기가 말한다 — 여기서 그 문장을 가로채지 않는다.
	token=$(cd tui && go run ./cmd/amtoken) || {
		echo "" >&2
		echo "토큰을 못 만들었다. 배포물 없이 만들려면 --dev 를 쓴다." >&2
		exit 1
	}
	ldflags="-X amcli/tui/internal/applemusic.EmbeddedToken=$token"
fi

# 판을 박는다. 안 박으면 첫 화면이 "v0.1.0-dev" 라고 말한다 —
# 받는 사람에게 개발 중인 물건을 준 셈이 된다.
if [ -n "$version" ]; then
	ldflags="$ldflags -X amcli/tui/internal/host.Version=$version"
fi

echo "빌드 중… → $out"
( cd tui && go build -trimpath -ldflags "$ldflags" -o "$out" . )

echo ""
echo "$out"
ls -lh "$out" | awk '{print "  " $5}'
if [ -z "$dev" ]; then
	echo "  개발자 토큰이 박혀 있다 — 받는 사람은 /setup 없이 애플 뮤직 검색이 된다"
else
	echo "  개발 빌드 — 각자 ~/.config/amcli 설정으로 돈다"
fi
echo ""
echo "다음: 서명하지 않으면 Gatekeeper 가 막는다."
echo "  codesign --deep --force --options runtime --sign \"Developer ID Application: …\" $out"
