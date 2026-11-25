SHELL := /usr/bin/env bash
GO ?= $(shell command -v go 2>/dev/null || echo go)
BINARY := agy-usage
OUTPUT_DIR := _output
INSTALL_DIR ?= $(HOME)/.local/bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo v0.1.0)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w \
	-X github.com/LiangNing7/agy-usage/internal/version.GitVersion=$(VERSION) \
	-X github.com/LiangNing7/agy-usage/internal/version.GitCommit=$(GIT_COMMIT) \
	-X github.com/LiangNing7/agy-usage/internal/version.BuildDate=$(BUILD_DATE)

PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

.PHONY: all build install test fmt vet clean cross-build

all: fmt vet test build

build:
	@mkdir -p $(OUTPUT_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(OUTPUT_DIR)/$(BINARY) ./cmd/agy-usage

install: build
	@mkdir -p $(INSTALL_DIR)
	install -m 0755 $(OUTPUT_DIR)/$(BINARY) $(INSTALL_DIR)/$(BINARY)

test:
	@if command -v gcc >/dev/null 2>&1 || command -v clang >/dev/null 2>&1; then \
		$(GO) test -race -count=1 ./...; \
	else \
		CGO_ENABLED=0 $(GO) test -count=1 ./...; \
	fi

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

cross-build:
	@mkdir -p $(OUTPUT_DIR)/dist
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		out="$(OUTPUT_DIR)/dist/$(BINARY)-$$os-$$arch$$ext"; \
		echo "Building $$out ..."; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o "$$out" ./cmd/agy-usage || exit 1; \
	done

clean:
	rm -rf $(OUTPUT_DIR) dist
