SHELL := /bin/sh

# Load .env if present so `make run-*` uses the same values as .env.example
-include .env
export

BUF_VERSION ?= 1.75.0
BUF_DIR     := bin/tools
BUF         := $(BUF_DIR)/buf-$(BUF_VERSION)
BUF_ASSET   = buf-$(shell uname -s)-$(shell uname -m)

.PHONY: check test build proto proto-check proto-lint proto-generate \
        run-core run-api run-web run-all

# Standard recipe (CI runs this; any change here changes CI too).
check:
	go vet ./...
	go test -race ./...

# Convenience alias for test-only iteration.
test:
	go test ./...

# Pinned buf (no system install). Download URL/asset names match buf v1.x releases.
$(BUF):
	@mkdir -p $(BUF_DIR)
	@echo "downloading buf v$(BUF_VERSION) ($(BUF_ASSET))"
	@curl -fsSL "https://github.com/bufbuild/buf/releases/download/v$(BUF_VERSION)/$(BUF_ASSET)" -o "$@.tmp"
	@chmod +x "$@.tmp"
	@mv "$@.tmp" "$@"

proto-lint: $(BUF)
	$(BUF) lint

# Regenerates into gen/go (buf.gen.yaml: clean+out) and fails if tracked output differs.
proto-generate: $(BUF)
	$(BUF) generate
	git diff --exit-code -- gen/go

proto-check: proto-lint proto-generate
proto: proto-check

# Build the three deployable binaries into bin/ (gitignored).
build:
	@mkdir -p bin
	go build -o bin/abcmovies-core ./cmd/abcmovies-core
	go build -o bin/abcmovies-api  ./cmd/abcmovies-api
	go build -o bin/abcmovies-web  ./cmd/abcmovies-web

# Run one unit (uses env per .env.example: CORE_* / API_* / WEB_*).
run-core:
	go run ./cmd/abcmovies-core

run-api:
	go run ./cmd/abcmovies-api

run-web:
	go run ./cmd/abcmovies-web

# Runs core, api, web in this one command. Ctrl-C stops all three.
# Logs are interleaved (no per-process prefixing).
# Loads .env automatically if present; no manual export needed.
run-all:
	@pids=''; \
		trap 'kill $$pids 2>/dev/null' INT TERM EXIT; \
		$(MAKE) --no-print-directory run-core & pids="$$pids $$!"; \
		$(MAKE) --no-print-directory run-api & pids="$$pids $$!"; \
		$(MAKE) --no-print-directory run-web & pids="$$pids $$!"; \
		wait
