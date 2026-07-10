BINARY  := slop-chop
MODULE  := github.com/dcadolph/slop-chop
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X $(MODULE)/cmd.version=$(VERSION)"

GO ?= go

# GOBIN is where "go install" drops the binary. Fall back to GOPATH/bin.
GOBIN := $(shell $(GO) env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell $(GO) env GOPATH)/bin
endif

.DEFAULT_GOAL := help
.PHONY: build install uninstall test cover vet lint fmt tidy clean help

## build: compile the binary into the repo root with the version stamped
build:
	$(GO) build $(LDFLAGS) -o $(BINARY) .

## install: install the binary into GOBIN with the version stamped
install:
	$(GO) install $(LDFLAGS) .

## uninstall: remove the installed binary from GOBIN
uninstall:
	rm -f $(GOBIN)/$(BINARY)

## test: run the full test suite
test:
	$(GO) test ./...

## cover: run tests and write a coverage profile
cover:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out

## vet: run go vet
vet:
	$(GO) vet ./...

## lint: run golangci-lint (must be installed separately)
lint:
	golangci-lint run

## fmt: format all Go source
fmt:
	$(GO) fmt ./...

## tidy: sync go.mod and go.sum
tidy:
	$(GO) mod tidy

## clean: remove the built binary and coverage profile
clean:
	rm -f $(BINARY) coverage.out

## help: list available targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## //' | awk -F': ' '{printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'
