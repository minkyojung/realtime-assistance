# shazamd 를 .app 번들로 만든다.
#
# 프로파일이 있을 때만 엔타이틀먼트를 붙인다. ad-hoc 서명에 제한
# 엔타이틀먼트를 붙이면 AMFI 가 실행 즉시 죽인다(SIGKILL) — 그러면
# "왜 안 되는지"조차 화면에 못 띄운다. 프로파일이 없으면 서명 없이 두고,
# 카탈로그 조회에서 catalog-refused 로 정직하게 실패한다.
#
#   make                      서명 없이 빌드 — 마이크·시그니처까지 확인된다
#   make PROFILE=~/x.provisionprofile IDENTITY="Developer ID Application: ..."
#                             엔타이틀먼트까지 붙인 진짜 빌드

BUNDLE_ID ?= com.minkyojung.amcli.shazamd
APP       := build/Shazamd.app
BIN       := $(APP)/Contents/MacOS/shazamd
PROFILE   ?=
IDENTITY  ?=

$(BIN): shazamd/main.swift Info.plist
	@mkdir -p $(APP)/Contents/MacOS
	swiftc -O -target arm64-apple-macos14 shazamd/main.swift -o $(BIN)
	sed 's/__BUNDLE_ID__/$(BUNDLE_ID)/' Info.plist > $(APP)/Contents/Info.plist
ifneq ($(PROFILE),)
	cp $(PROFILE) $(APP)/Contents/embedded.provisionprofile
	codesign -f -s $(if $(IDENTITY),"$(IDENTITY)",-) \
		--entitlements shazamd.entitlements -o runtime $(APP)
else
ifneq ($(IDENTITY),)
	codesign -f -s "$(IDENTITY)" -o runtime $(APP)
endif
endif

build: $(BIN)

# 실제로 소리를 들려주고 확인하는 자리. 스피커로 노래를 틀어 두고 부른다.
listen: $(BIN)
	$(BIN) -seconds 8

clean:
	rm -rf build

.PHONY: build listen clean
