GO        ?= go
BUF       ?= buf
GOLANGCI  ?= golangci-lint
GOBIN     := $(CURDIR)/bin
GOMODCACHE := $(CURDIR)/.gomodcache
GOCACHE   := $(CURDIR)/.gocache
GOPATHBIN := $(shell $(GO) env GOPATH)/bin
export PATH := $(GOBIN):$(GOPATHBIN):$(PATH)
export GOBIN GOMODCACHE GOCACHE

.PHONY: deps proto lint build test-unit fixtures check run

deps:
	mkdir -p $(GOBIN)
	cd core && GOBIN=$(GOBIN) $(GO) install tool

proto:
	$(BUF) generate
	$(BUF) format -w proto

lint:
	$(BUF) lint
	$(BUF) breaking --against .git#branch=main
	cd core && $(GOLANGCI) run ./cmd/... ./internal/...

build:
	cd core && $(GO) build ./...

test-unit:
	cd core && $(GO) test ./...

fixtures: proto
	cd core && $(GO) build -o $(GOBIN)/fixture-runner ./cmd/fixture-runner
	$(GOBIN)/fixture-runner fixtures

check: deps proto lint build test-unit fixtures

run:
	cd core && $(GO) run ./cmd/abcmovies
