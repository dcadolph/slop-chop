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
.PHONY: build install uninstall test cover vet lint fmt tidy clean wasm obsidian npm-package worker site site-deploy check-versions help

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

# ESBUILD pins the minifier so plugin builds reproduce across machines and CI.
ESBUILD := esbuild@0.25.5

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

## check-versions: every shipped surface must name one version. The release fails its
## README pin check when these drift, and OpenVSX looks for a vsix named from the tag, so
## a surface left behind silently misses its publish. The Obsidian manifest and the npm
## package are rewritten from the tag during the release and are checked here anyway, so
## the committed tree never disagrees with itself.
check-versions:
	@set -eu; \
	pin=$$(grep -oE 'dcadolph/slop-chop@v[0-9]+\.[0-9]+\.[0-9]+' README.md | head -1 | sed 's/.*@v//'); \
	if [ -z "$$pin" ]; then echo "README.md names no action version"; exit 1; fi; \
	fail=0; \
	pins=$$(grep -oE 'dcadolph/slop-chop@v[0-9]+\.[0-9]+\.[0-9]+' README.md | sort -u | wc -l | tr -d ' '); \
	if [ "$$pins" != "1" ]; then echo "README.md pins more than one version"; fail=1; fi; \
	for f in npm/package.json vscode/package.json obsidian/manifest.json; do \
		v=$$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' $$f | head -1); \
		if [ "$$v" != "$$pin" ]; then echo "$$f is $$v, README pins $$pin"; fail=1; fi; \
	done; \
	v=$$(sed -n 's/^version[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' jetbrains/build.gradle.kts | head -1); \
	if [ "$$v" != "$$pin" ]; then echo "jetbrains/build.gradle.kts is $$v, README pins $$pin"; fail=1; fi; \
	if ! grep -q "\"$$pin\"[[:space:]]*:" obsidian/versions.json; then \
		echo "obsidian/versions.json has no entry for $$pin"; fail=1; fi; \
	if [ "$$fail" != "0" ]; then \
		echo "every shipped surface must name $$pin before the tag is cut"; exit 1; fi; \
	echo "every shipped surface names $$pin"

## clean: remove the built binary, wasm artifacts, and coverage profile
clean:
	rm -f $(BINARY) coverage.out docs/assets/slop-chop.wasm docs/assets/wasm_exec.js
	rm -rf obsidian/dist obsidian/engine

## help: list available targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## //' | awk -F': ' '{printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'
