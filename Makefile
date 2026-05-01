BIN ?= vibecode-helper
PREFIX ?= /usr/local

.PHONY: build install test fmt

build:
	go mod tidy
	go build -o $(BIN) ./cmd/vibecode-helper

install: build
	install -m 0755 $(BIN) $(PREFIX)/bin/$(BIN)

test:
	go test ./...

fmt:
	gofmt -w cmd/vibecode-helper internal
