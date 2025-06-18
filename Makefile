# PhotoSorter Go Makefile

# Variables
BINARY_NAME=photo-sorter
BINARY_UNIX=$(BINARY_NAME)_unix
BINARY_WINDOWS=$(BINARY_NAME).exe
BINARY_DARWIN=$(BINARY_NAME)_darwin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d %H:%M:%S UTC')
LDFLAGS=-ldflags "-X 'main.version=$(VERSION)' -X 'main.buildTime=$(BUILD_TIME)'"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Main source directory
MAIN_PACKAGE=./cmd/photo-sorter

# Default target
.PHONY: all
all: test build

# Build the binary
.PHONY: build
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) -v $(MAIN_PACKAGE)

# Build for multiple platforms
.PHONY: build-all
build-all: build-linux build-windows build-darwin

.PHONY: build-linux
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_UNIX) -v $(MAIN_PACKAGE)

.PHONY: build-windows
build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_WINDOWS) -v $(MAIN_PACKAGE)

.PHONY: build-darwin
build-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_DARWIN) -v $(MAIN_PACKAGE)

# Build for ARM architectures
.PHONY: build-arm
build-arm: build-linux-arm64 build-darwin-arm64

.PHONY: build-linux-arm64
build-linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)_linux_arm64 -v $(MAIN_PACKAGE)

.PHONY: build-darwin-arm64
build-darwin-arm64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)_darwin_arm64 -v $(MAIN_PACKAGE)

# Run tests
.PHONY: test
test:
	$(GOTEST) -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Run tests with race detection
.PHONY: test-race
test-race:
	$(GOTEST) -race -v ./...

# Run benchmarks
.PHONY: bench
bench:
	$(GOTEST) -bench=. -benchmem ./...

# Clean build artifacts
.PHONY: clean
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)
	rm -f $(BINARY_WINDOWS)
	rm -f $(BINARY_DARWIN)
	rm -f $(BINARY_NAME)_*
	rm -f coverage.out
	rm -f coverage.html

# Run go mod tidy
.PHONY: deps
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Format code
.PHONY: fmt
fmt:
	$(GOFMT) ./...

# Run go vet
.PHONY: vet
vet:
	$(GOVET) ./...

# Run golint (requires golint to be installed)
.PHONY: lint
lint:
	golint ./...

# Run all quality checks
.PHONY: check
check: fmt vet lint test

# Install the binary
.PHONY: install
install: build
	cp $(BINARY_NAME) /usr/local/bin/

# Uninstall the binary
.PHONY: uninstall
uninstall:
	rm -f /usr/local/bin/$(BINARY_NAME)

# Run the application
.PHONY: run
run:
	$(GOCMD) run $(MAIN_PACKAGE)

# Run with example configuration
.PHONY: run-example
run-example:
	$(GOCMD) run $(MAIN_PACKAGE) --config config.example.yaml --dry-run

# Create release archives
.PHONY: release
release: build-all
	mkdir -p release
	tar -czf release/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz $(BINARY_UNIX) README.md config.example.yaml
	zip -j release/$(BINARY_NAME)-$(VERSION)-windows-amd64.zip $(BINARY_WINDOWS) README.md config.example.yaml
	tar -czf release/$(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz $(BINARY_DARWIN) README.md config.example.yaml

# Development setup
.PHONY: dev-setup
dev-setup:
	$(GOGET) -u golang.org/x/lint/golint
	$(GOGET) -u github.com/goreleaser/goreleaser

# Docker build
.PHONY: docker-build
docker-build:
	docker build -t photo-sorter:$(VERSION) .
	docker tag photo-sorter:$(VERSION) photo-sorter:latest

# Docker run
.PHONY: docker-run
docker-run:
	docker run --rm -v $(PWD)/photos:/photos photo-sorter:latest

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build         - Build the binary"
	@echo "  build-all     - Build for all platforms"
	@echo "  build-linux   - Build for Linux"
	@echo "  build-windows - Build for Windows"
	@echo "  build-darwin  - Build for macOS"
	@echo "  build-arm     - Build for ARM architectures"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  test-race     - Run tests with race detection"
	@echo "  bench         - Run benchmarks"
	@echo "  clean         - Clean build artifacts"
	@echo "  deps          - Download and tidy dependencies"
	@echo "  fmt           - Format code"
	@echo "  vet           - Run go vet"
	@echo "  lint          - Run golint"
	@echo "  check         - Run all quality checks"
	@echo "  install       - Install binary to /usr/local/bin"
	@echo "  uninstall     - Remove binary from /usr/local/bin"
	@echo "  run           - Run the application"
	@echo "  run-example   - Run with example config in dry-run mode"
	@echo "  release       - Create release archives"
	@echo "  dev-setup     - Install development tools"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run in Docker container"
	@echo "  help          - Show this help message"

# Default target when just running 'make'
.DEFAULT_GOAL := help
