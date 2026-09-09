BIN := camera-ascii
PLIST := -Xlinker -sectcreate -Xlinker __TEXT -Xlinker __info_plist -Xlinker Info.plist

$(BIN): render.swift main.swift Info.plist
	swiftc -O render.swift main.swift -o $(BIN) $(PLIST)

run: $(BIN)
	./$(BIN)

test: rendertest.bin
	./rendertest.bin
	./rendertest.bin --mono

rendertest.bin: render.swift rendertest/main.swift
	swiftc -O render.swift rendertest/main.swift -o rendertest.bin

clean:
	rm -f $(BIN) rendertest.bin

.PHONY: run test clean
