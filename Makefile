# Makefile for tls-scanner

# Binary names
SCANNER_BINARY=tls-scanner
ANALYZER_BINARY=tls-analyzer

# Build directory
BUILD_DIR=bin

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Version info — injected at build time, no .git/ needed in container
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)

# Default target
.PHONY: all
all: build

# Build both binaries
.PHONY: build
build:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GOBUILD) -mod=readonly -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(SCANNER_BINARY) ./cmd/tls-scanner
	CGO_ENABLED=0 $(GOBUILD) -mod=readonly -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(ANALYZER_BINARY) ./cmd/tls-analyzer

# Clean build artifacts
.PHONY: clean
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

# Download dependencies
.PHONY: deps
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Run tests (skips integration tests that hit live endpoints)
.PHONY: test
test:
	$(GOTEST) -v -short ./...

# Run benchmarks (parsing only, no network)
.PHONY: bench
bench:
	$(GOTEST) -bench=. -benchmem -short ./internal/...

# Run integration benchmarks against live TLS endpoints (requires testssl.sh)
.PHONY: bench-integration
bench-integration:
	$(GOTEST) -v -run Integration -count=1 -timeout=600s ./internal/scanner/

# Vet
.PHONY: vet
vet:
	$(GOCMD) vet ./...

# Install binaries to GOPATH/bin
.PHONY: install
install:
	$(GOCMD) install ./cmd/tls-scanner
	$(GOCMD) install ./cmd/tls-analyzer

# Run the scanner with default parameters
.PHONY: run
run: build
	./$(BUILD_DIR)/$(SCANNER_BINARY)

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build        - Build the binary"
	@echo "  clean        - Clean build artifacts"
	@echo "  deps         - Download and tidy dependencies"
	@echo "  test         - Run tests"
	@echo "  bench        - Run benchmarks (parsing, no network)"
	@echo "  bench-integ  - Run integration benchmarks (requires testssl.sh)"
	@echo "  vet          - Run go vet"
	@echo "  install      - Install binary to GOPATH/bin"
	@echo "  run          - Build and run with default parameters"
	@echo "  help         - Show this help message"
