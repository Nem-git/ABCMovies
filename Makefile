SHELL := /bin/sh

.PHONY: check build test run-core run-api run-web run-all

# Standard recipe (CI runs this; do not change its behavior).
check:
	go vet ./...
	go test ./...

# Convenience alias for test-only iteration.
test:
	go test ./...

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
# Export env vars first if needed, e.g.: set -a; . .env; set +a; make run-all
run-all:
	@pids=''; \
		trap 'kill $$pids 2>/dev/null' INT TERM EXIT; \
		$(MAKE) --no-print-directory run-core & pids="$$pids $$!"; \
		$(MAKE) --no-print-directory run-api & pids="$$pids $$!"; \
		$(MAKE) --no-print-directory run-web & pids="$$pids $$!"; \
		wait
