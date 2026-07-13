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
.PHONY: build install uninstall test cover vet lint fmt tidy clean wasm extension extension-package obsidian npm-package worker help

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

## wasm: build the browser engine and its JS glue into docs/assets
wasm:
	GOOS=js GOARCH=wasm $(GO) build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o docs/assets/slop-chop.wasm ./wasm
	cp "$(shell $(GO) env GOROOT)/lib/wasm/wasm_exec.js" docs/assets/wasm_exec.js

## extension: build the wasm engine and stage it into the browser extension
extension: wasm
	mkdir -p extension/engine
	cp docs/assets/slop-chop.wasm extension/engine/slop-chop.wasm
	cp docs/assets/wasm_exec.js extension/engine/wasm_exec.js

## extension-package: zip the built extension for a store upload
extension-package: extension
	rm -f slop-chop-extension.zip
	cd extension && zip -qr ../slop-chop-extension.zip . -x '.*'

## obsidian: build the wasm engine and stage it into the Obsidian plugin
obsidian: wasm
	mkdir -p obsidian/engine
	cp docs/assets/slop-chop.wasm obsidian/engine/slop-chop.wasm
	cp docs/assets/wasm_exec.js obsidian/engine/wasm_exec.js

## npm-package: build the wasm engine and stage it into the npm package
npm-package: wasm
	mkdir -p npm/engine
	cp docs/assets/slop-chop.wasm npm/engine/slop-chop.wasm
	cp docs/assets/wasm_exec.js npm/engine/wasm_exec.js

## worker: build the wasm engine and stage it into the hosted API worker
worker: wasm
	mkdir -p worker/engine
	cp docs/assets/slop-chop.wasm worker/engine/slop-chop.wasm
	cp docs/assets/wasm_exec.js worker/engine/wasm_exec.js

## obsidian-dist: build a self-contained plugin main.js with the engine inlined as base64,
## the form Obsidian's community installer needs since it only downloads main.js
obsidian-dist: wasm
	mkdir -p obsidian/dist
	cp obsidian/manifest.json obsidian/versions.json obsidian/dist/ 2>/dev/null || cp obsidian/manifest.json obsidian/dist/
	cat docs/assets/wasm_exec.js > obsidian/dist/main.js
	printf 'globalThis.SLOP_WASM_B64=%s;\n' "\"$$(base64 < docs/assets/slop-chop.wasm | tr -d '\n')\"" >> obsidian/dist/main.js
	cat obsidian/main.js >> obsidian/dist/main.js

## clean: remove the built binary, wasm artifacts, and coverage profile
clean:
	rm -f $(BINARY) coverage.out docs/assets/slop-chop.wasm docs/assets/wasm_exec.js \
		extension/engine/slop-chop.wasm extension/engine/wasm_exec.js

## help: list available targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## //' | awk -F': ' '{printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'
