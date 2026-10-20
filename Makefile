# Сборка и проверка ttd (Go).

GO      ?= go
BIN     ?= ttd
PKG     ?= ./...

.PHONY: all build test vet fmt clean run

all: build

build:
	$(GO) build -trimpath -ldflags "-s -w" -o $(BIN) ./cmd/ttd

test:
	$(GO) test $(PKG)

vet:
	$(GO) vet $(PKG)

fmt:
	$(GO) fmt $(PKG)

run: build
	./$(BIN) --help

clean:
	rm -f $(BIN)
