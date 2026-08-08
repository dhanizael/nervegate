# NerveGate Makefile
# The Sub-Millisecond Intelligent Gateway & Model Orchestrator for Linux

BINARY_NAME=nervegate
BUILD_DIR=bin
MAIN_PATH=./cmd/nervegate
VERSION?=0.1.0-alpha
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE?=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildDate=$(BUILD_DATE) -s -w"

.PHONY: all build test test-race lint bench clean install help

all: lint test build

## build: Compiles the binary into bin/nervegate
build:
	@echo "==> Building $(BINARY_NAME) v$(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)

## test: Runs unit tests
test:
	@echo "==> Running unit tests..."
	go test -v ./pkg/classifier ./pkg/rotator ./pkg/trimmer ./pkg/ingress

## test-race: Runs tests with Go race detector enabled
test-race:
	@echo "==> Running race detector tests..."
	go test -race -v ./pkg/classifier ./pkg/rotator ./pkg/trimmer ./pkg/ingress

## bench: Runs microsecond latency benchmarks
bench:
	@echo "==> Running performance benchmarks..."
	go test -bench=. -benchmem ./pkg/classifier ./pkg/trimmer

## lint: Runs golangci-lint
lint:
	@echo "==> Checking code quality..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed, running go vet..." && go vet ./...)
	@which golangci-lint > /dev/null && golangci-lint run ./... || true

## clean: Removes build artifacts
clean:
	@echo "==> Cleaning build output..."
	rm -rf $(BUILD_DIR)

## install: Installs nervegate binary to /usr/local/bin
install: build
	@echo "==> Installing $(BINARY_NAME) to /usr/local/bin..."
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
