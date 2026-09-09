bin/tui: $(shell find . -name '*.go' -not -name '*_test.go')
	go build -o bin/tui .

run: bin/tui
	./bin/tui

test:
	go test ./...

clean:
	rm -rf bin

.PHONY: run test clean
