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

.PHONY: deps proto web-build fmt secret-scan lint build test-unit fixtures vuln check run run-web

deps:
	mkdir -p $(GOBIN)
	GOBIN=$(GOBIN) $(GO) install tool
	$(NPM) ci
	cd frontends/web && $(NPM) ci

proto:
	$(BUF) generate
	$(BUF) generate --template buf.gen.web.yaml
	$(BUF) format -w proto

web-build:
	cd frontends/web && $(NPM) run build
	cp frontends/web/src/index.html frontends/web/serving/dist/index.html
	cp frontends/web/dist/bundle.js frontends/web/serving/dist/bundle.js

fmt:
	$(BUF) format -w proto
	$(GOLANGCI) fmt ./core/... ./frontends/web/...
	npx --no-install prettier . --write

# Scans the committed tree (not git history): `gitleaks detect` alone would
# scan every commit, and the repo's pre-scaffold history predates ABCMovies.
# Extracting HEAD means the gate is "the tree that would ship has no secrets".
secret-scan:
	rm -rf /tmp/abcmovies-secret-scan && mkdir -p /tmp/abcmovies-secret-scan
	git archive HEAD | tar -x -C /tmp/abcmovies-secret-scan
	$(GITLEAKS) detect --no-git --source /tmp/abcmovies-secret-scan --redact; status=$$?; rm -rf /tmp/abcmovies-secret-scan; exit $$status

lint: web-build
	$(BUF) lint
	git diff --exit-code -- proto
# 	$(BUF) breaking --against .git#branch=dev,ref=0a716ba
	$(GOLANGCI) run ./core/... ./frontends/web/...
	npx --no-install prettier . --check
	npx --no-install markdownlint-cli2
	$(MAKE) secret-scan

build: web-build
	$(GO) build ./core/... ./frontends/web/...

test-unit: web-build
	$(GO) test -race ./core/... ./frontends/web/...

vuln: web-build
	$(GO) tool govulncheck ./core/... ./frontends/web/...

fixtures: proto web-build
	$(GO) build -o $(GOBIN)/fixture-runner ./core/cmd/fixture-runner
	$(GOBIN)/fixture-runner fixtures

check: deps proto lint build test-unit fixtures vuln

run:
	$(GO) run ./core/cmd/abcmovies

run-web: web-build
	$(GO) run ./frontends/web
