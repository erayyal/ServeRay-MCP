GO ?= go
GOLANGCI_LINT ?= golangci-lint
GOVULNCHECK ?= govulncheck
GORELEASER ?= goreleaser
SERVER ?=
OUT_DIR ?= .bin
PREFIX ?= $(HOME)/.local/bin

.PHONY: build build-server install-server fmt fmt-check lint test vet vuln verify script-check release-check package

build:
	$(GO) build ./cmd/...

build-server:
	test -n "$(SERVER)"
	mkdir -p "$(OUT_DIR)"
	$(GO) build -o "$(OUT_DIR)/$(SERVER)" "./cmd/$(SERVER)"

install-server:
	test -n "$(SERVER)"
	mkdir -p "$(PREFIX)"
	$(GO) build -o "$(PREFIX)/$(SERVER)" "./cmd/$(SERVER)"

fmt:
	gofmt -w cmd internal

fmt-check:
	test -z "$$(gofmt -l cmd internal)"

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

lint:
	$(GOLANGCI_LINT) run

vuln:
	$(GOVULNCHECK) ./...

script-check:
	sh -n scripts/install.sh scripts/build-release.sh

release-check:
	$(GORELEASER) check

verify: fmt-check vet test build script-check

package:
	./scripts/build-release.sh $(SERVER)
