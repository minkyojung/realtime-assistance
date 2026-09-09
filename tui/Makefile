# tui 는 Go 지만 카메라는 아니다.
#
# macOS 카메라는 AVFoundation 이고, cgo 로 끌어오는 대신 헬퍼 프로세스로
# 나눴다. 그래서 빌드가 두 갈래다 — 둘 다 bin/ 에 들어가고, 앱은 자기
# 실행 파일 옆에서 헬퍼를 찾는다.

CAMERAD_SRC := internal/cameraapp/camerad/main.swift
CAMERAD_PLIST := internal/cameraapp/camerad/Info.plist

all: bin/tui bin/camerad

bin/tui: $(shell find . -name '*.go' -not -name '*_test.go')
	go build -o bin/tui .

# NSCameraUsageDescription 을 바이너리에 박아 넣는다. 번들이 아니어서
# Info.plist 를 파일로 둘 수 없다.
bin/camerad: $(CAMERAD_SRC) $(CAMERAD_PLIST)
	@mkdir -p bin
	swiftc -O $(CAMERAD_SRC) -o bin/camerad \
		-Xlinker -sectcreate -Xlinker __TEXT -Xlinker __info_plist -Xlinker $(CAMERAD_PLIST)

run: all
	./bin/tui

test:
	go test ./...

clean:
	rm -rf bin

.PHONY: all run test clean
