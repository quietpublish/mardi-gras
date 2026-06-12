BINARY := mg
BUILD_DIR := .
GO := go
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build run run-sample test clean dev dev-gt screenshot tidy fmt lint gc-client

# GCDIR is the generated Gas City client package.
GCDIR := internal/gastown/gcclient

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

dev-gt: build
	PATH="$(CURDIR)/testdata:$(PATH)" ./$(BINARY) --path testdata/sample.jsonl

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

# gc-client regenerates the Gas City Supervisor API client from the pinned
# spec. The committed openapi.json is the authentic 3.1 contract; oapi-codegen
# does not yet fully support 3.1, so downgrade.jq rewrites it to 3.0 first.
# Bump openapi.json to a new gascity tag, then run this.
gc-client:
	jq -f $(GCDIR)/downgrade.jq $(GCDIR)/openapi.json > $(GCDIR)/.openapi-3.0.json
	cd $(GCDIR) && oapi-codegen --config config.yaml .openapi-3.0.json
	rm -f $(GCDIR)/.openapi-3.0.json
	$(GO) build ./$(GCDIR)/
