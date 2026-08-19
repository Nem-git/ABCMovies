GO        ?= go
BUF       ?= buf
GOLANGCI  ?= golangci-lint
NPM       ?= npm
GITLEAKS  ?= gitleaks
GOBIN     := $(CURDIR)/bin
GOMODCACHE := $(CURDIR)/.gomodcache
GOCACHE   := $(CURDIR)/.gocache
GOPATHBIN := $(shell $(GO) env GOPATH)/bin
MISESHIMS := $(HOME)/.local/share/mise/shims
export PATH := $(GOBIN):$(GOPATHBIN):$(MISESHIMS):$(PATH)
export GOBIN GOMODCACHE GOCACHE

.PHONY: deps proto fmt secret-scan lint build test-unit fixtures vuln check run

deps:
	mkdir -p $(GOBIN)
	cd core && GOBIN=$(GOBIN) $(GO) install tool
	$(NPM) ci

proto:
	$(BUF) generate
	$(BUF) format -w proto

fmt:
	$(BUF) format -w proto
	cd core && $(GOLANGCI) fmt
	npx --no-install prettier . --write

# Scans the committed tree (not git history): `gitleaks detect` alone would
# scan every commit, and the repo's pre-scaffold history predates ABCMovies.
# Extracting HEAD means the gate is "the tree that would ship has no secrets".
secret-scan:
	rm -rf /tmp/abcmovies-secret-scan && mkdir -p /tmp/abcmovies-secret-scan
	git archive HEAD | tar -x -C /tmp/abcmovies-secret-scan
	$(GITLEAKS) detect --no-git --source /tmp/abcmovies-secret-scan --redact; status=$$?; rm -rf /tmp/abcmovies-secret-scan; exit $$status

lint:
	$(BUF) lint
	git diff --exit-code -- proto
# 	$(BUF) breaking --against .git#branch=dev,ref=0a716ba
	cd core && $(GOLANGCI) run ./cmd/... ./internal/...
	npx --no-install prettier . --check
	npx --no-install markdownlint-cli2
	$(MAKE) secret-scan

build:
	cd core && $(GO) build ./...

test-unit:
	cd core && $(GO) test -race ./...

vuln:
	cd core && $(GO) tool govulncheck ./...

fixtures: proto
	cd core && $(GO) build -o $(GOBIN)/fixture-runner ./cmd/fixture-runner
	$(GOBIN)/fixture-runner fixtures

check: deps proto lint build test-unit fixtures vuln

run:
	cd core && $(GO) run ./cmd/abcmovies
