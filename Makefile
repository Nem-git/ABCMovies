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

.PHONY: deps proto web-build fmt secret-scan lint build test-unit tidy-check milestone fixtures vuln check run run-web

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
	mkdir -p frontends/web/serving/dist
	cp frontends/web/src/index.html frontends/web/serving/dist/index.html
	cp frontends/web/dist/bundle.js frontends/web/serving/dist/bundle.js

fmt:
	$(BUF) format -w proto
	$(GOLANGCI) fmt ./core/... ./adapters/... ./frontends/web/...
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
	# Breaking-change detection is disabled until the first release
	# (TECHNICAL-DECISIONS.md §1.24): contracts may evolve breaking-ly while
	# every consumer is in-repo. Re-enable before any contract is published.
	# $(BUF) breaking --against '.git#ref=refs/remotes/origin/main'
	$(GOLANGCI) run ./core/... ./adapters/... ./frontends/web/...
	npx --no-install prettier . --check
	npx --no-install markdownlint-cli2
	$(MAKE) secret-scan

build: web-build
	$(GO) build ./core/... ./adapters/... ./frontends/web/...

test-unit: web-build
	$(GO) test -race ./core/... ./adapters/... ./frontends/web/...

vuln: web-build
	$(GO) tool govulncheck ./core/... ./adapters/... ./frontends/web/...

# Fails when `go mod tidy` would change anything. `-diff` prints the change
# and exits non-zero without writing, so the check never mutates the tree; a
# drift here means go.mod/go.sum were committed by hand and the module is not
# the source of truth for its own dependency graph.
tidy-check:
	$(GO) mod tidy -diff

# The milestone whose acceptance criteria gate the next image tag (CI-CD.md
# §5). The value lives exactly once, at the repo root; CI re-reads the same
# file at release time. It must look like m<N> and be backed by a matching
# core/tests/m<N> acceptance tree, so a bump cannot point at thin air.
milestone:
	@m=$$(cat MILESTONE); \
	case "$$m" in m[0-9]*) ;; *) echo "MILESTONE must be of shape m<N>, got '$$m'" >&2; exit 1;; esac; \
	test -d core/tests/$$m || { echo "no core/tests/$$m acceptance tree for MILESTONE=$$m" >&2; exit 1; }; \
	echo "milestone=$$m"

fixtures: proto web-build
	$(GO) build -o $(GOBIN)/fixture-runner ./core/cmd/fixture-runner
	$(GOBIN)/fixture-runner fixtures

check: deps proto lint tidy-check milestone build test-unit fixtures vuln

run:
	$(GO) run ./core/cmd/abcmovies

run-web: web-build
	$(GO) run ./frontends/web
