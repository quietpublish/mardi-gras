BINARY := mg
BUILD_DIR := .
GO := go
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build run run-sample test clean dev screenshot tidy fmt lint

build:
	$(GO) build $(LDFLAGS) -o $(BINARY) ./cmd/mg

run: build
	./$(BINARY)

run-sample: build
	./$(BINARY) --path testdata/sample.jsonl

test:
	$(GO) test ./...

clean:
	rm -f $(BINARY)
	rm -rf dist/

dev: build
	./$(BINARY) --path testdata/sample.jsonl

screenshot: build
	@echo "Launching mg with screenshot dataset..."
	@echo "Tip: resize terminal to ~120x38 for best results"
	./$(BINARY) --path testdata/screenshot.jsonl

tidy:
	$(GO) mod tidy

fmt:
	$(GO) fmt ./...

lint:
	golangci-lint run ./...
