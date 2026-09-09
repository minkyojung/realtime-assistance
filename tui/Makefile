BIN := bin/yarrr

$(BIN): $(shell find . -name '*.go' -not -name '*_test.go')
	go build -o $(BIN) .

run: $(BIN)
	./$(BIN)

# 배포 빌드 — p8 로 토큰을 서명해 바이너리에 박는다.
# p8 은 여기까지만 오고 배포물에는 서명된 토큰만 들어간다.
release:
	go build -ldflags "-X amcli/tui/internal/applemusic.EmbeddedToken=$$(go run ./cmd/amtoken)" -o $(BIN) .

test:
	go test ./...

clean:
	rm -rf bin

.PHONY: run release test clean
