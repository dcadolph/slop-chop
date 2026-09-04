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
.PHONY: build install uninstall test cover vet lint fmt tidy clean wasm obsidian npm-package worker site site-deploy help

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

## obsidian: build the self-contained Obsidian plugin into obsidian/dist. The engine is
## gzipped and inlined as base64, since the community installer only downloads main.js
## and Obsidian Sync caps a plugin file at 5 MB, and the JS glue is minified. The plugin
## decodes the payload in memory, so the shipped bundle needs no filesystem access.
obsidian: wasm
	mkdir -p obsidian/dist
	cp obsidian/manifest.json obsidian/versions.json obsidian/dist/
	printf '/* wasm_exec.js: Copyright 2018 The Go Authors, BSD-style license, https://go.dev/LICENSE */\n' > obsidian/dist/main.js
	npx -y $(ESBUILD) docs/assets/wasm_exec.js --minify >> obsidian/dist/main.js
	printf 'globalThis.SLOP_WASM_B64_GZ=%s;\n' "\"$$(gzip -9 -n -c docs/assets/slop-chop.wasm | base64 | tr -d '\n')\"" >> obsidian/dist/main.js
	npx -y $(ESBUILD) obsidian/main.js --minify >> obsidian/dist/main.js

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

## site: build the documentation site into site/ with a freshly built engine
site: wasm
	mkdocs build --strict

## site-deploy: build the site and publish it to the Worker that serves slop-chop.com
site-deploy: site
	npx -y wrangler@4 deploy --config wrangler.site.jsonc

## clean: remove the built binary, wasm artifacts, and coverage profile
clean:
	rm -f $(BINARY) coverage.out docs/assets/slop-chop.wasm docs/assets/wasm_exec.js
	rm -rf obsidian/dist obsidian/engine

## help: list available targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## //' | awk -F': ' '{printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'
