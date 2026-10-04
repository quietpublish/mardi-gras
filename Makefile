BINARY := mg
BUILD_DIR := .
GO := go
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build run run-sample test clean dev dev-gt dev-gc dev-bd dev-jev contract-bd contract-jev screenshot screenshots-gc screenshot-light demo-gif tidy fmt lint gc-client

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

# dev-bd runs mg in CLI mode (the bd list refresh loop) against a fake bd
# (testdata/fake-bd/bin/bd) that serves testdata/sample.jsonl. The .beads/
# directory is created here because .gitignore excludes every .beads/.
# Set MG_FAKE_BD_LOG=<file> to log each bd invocation.
dev-bd: build
	mkdir -p testdata/fake-bd/.beads
	cd testdata/fake-bd && PATH="$(CURDIR)/testdata/fake-bd/bin:$(PATH)" $(CURDIR)/$(BINARY)

# contract-bd runs mg's bd events journal client against a real bd binary, in a
# throwaway workspace with its own HOME (never your real one: bd migrates a
# workspace's schema on first use). Usage: make contract-bd BD=/path/to/bd
contract-bd:
	@test -n "$(BD)" || { echo "usage: make contract-bd BD=/path/to/bd"; exit 2; }
	MG_BD_CONTRACT_BIN="$(BD)" $(GO) test -tags bdcontract -run TestBdContract -count=1 -v ./internal/data

# dev-gc runs mg against a fake Gas City supervisor (testdata/fakegc) — the
# HTTP analogue of dev-gt. Press ctrl+g for the Gas City panel. Good for
# demos and screenshots without a real `gc` install.
dev-gc: build
	./testdata/dev-gc.sh

# dev-jev runs mg against a fake Jev (System One) server (testdata/fakejev),
# so the Jev plumbing can be exercised without a TypeSafe key. The fake logs
# what mg sends to /tmp/mg-fakejev.log. FAKEJEV_FLAGS="-status 401" shows Jev
# disabling itself; "-fail-every 2" trips the circuit breaker.
dev-jev: build
	./testdata/dev-jev.sh

# contract-jev runs mg's Jev client against a real System One endpoint. It
# costs a fraction of a cent and needs network, so it is never part of
# `make test`. Usage: make contract-jev KEY=$$TYPESAFE_API_KEY
# (MG_JEV_CONTRACT_URL=http://host:port points it at a self-hosted server.)
contract-jev:
	@test -n "$(KEY)" || { echo "usage: make contract-jev KEY=<api key>"; exit 2; }
	MG_JEV_CONTRACT_KEY="$(KEY)" $(GO) test -tags jevcontract -run TestJevContract -count=1 -v ./internal/jev

screenshot: build
	@echo "Launching mg with screenshot dataset..."
	@echo "Tip: resize terminal to ~120x38 for best results"
	./$(BINARY) --path testdata/screenshot.jsonl

# screenshots-gc regenerates the Gas City screenshots via vhs + the fake
# supervisor (no `gc` install needed). Requires vhs (brew install vhs ffmpeg ttyd).
screenshots-gc:
	./testdata/screenshots-gc.sh

# demo-gif regenerates the animated README hero GIF (docs/screenshots/demo.gif)
# via vhs + the fake supervisor. Requires vhs (brew install vhs ffmpeg ttyd).
demo-gif:
	./testdata/demo-gif.sh

# screenshot-light regenerates the light-theme screenshots via vhs + the fake
# supervisor (no `gc` install needed). Requires vhs (brew install vhs ffmpeg ttyd).
screenshot-light:
	./testdata/screenshots-light.sh

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
