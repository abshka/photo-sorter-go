# Makefile for PhotoSorter Go project

BINARY_NAME=photo-sorter
BINARY_UNIX=$(BINARY_NAME)_unix
BINARY_WINDOWS=$(BINARY_NAME).exe
BINARY_DARWIN=$(BINARY_NAME)_darwin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d %H:%M:%S UTC')
LDFLAGS=-ldflags "-X 'main.version=$(VERSION)' -X 'main.buildTime=$(BUILD_TIME)'"

GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

MAIN_PACKAGE=./cmd/photo-sorter

.PHONY: all
all: test build

.PHONY: build
build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) -v $(MAIN_PACKAGE)

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

.PHONY: build-arm
build-arm: build-linux-arm64 build-darwin-arm64

.PHONY: build-linux-arm64
build-linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)_linux_arm64 -v $(MAIN_PACKAGE)

.PHONY: build-darwin-arm64
build-darwin-arm64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)_darwin_arm64 -v $(MAIN_PACKAGE)

.PHONY: test
test:
	$(GOTEST) -v ./...

.PHONY: test-coverage
test-coverage:
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

.PHONY: test-race
test-race:
	$(GOTEST) -race -v ./...

.PHONY: bench
bench:
	$(GOTEST) -bench=. -benchmem ./...

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

.PHONY: deps
deps:
	$(GOMOD) download
	$(GOMOD) tidy

.PHONY: fmt
fmt:
	$(GOFMT) ./...

.PHONY: vet
vet:
	$(GOVET) ./...

.PHONY: lint
lint:
	golint ./...

.PHONY: check
check: fmt vet lint test

.PHONY: install
install: build
	cp $(BINARY_NAME) /usr/local/bin/

.PHONY: uninstall
uninstall:
	rm -f /usr/local/bin/$(BINARY_NAME)

.PHONY: run
run:
	$(GOCMD) run $(MAIN_PACKAGE)

.PHONY: run-example
run-example:
	$(GOCMD) run $(MAIN_PACKAGE) --config config.example.yaml --dry-run

.PHONY: release
release: build-all
	mkdir -p release
	tar -czf release/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz $(BINARY_UNIX) README.md config.example.yaml
	zip -j release/$(BINARY_NAME)-$(VERSION)-windows-amd64.zip $(BINARY_WINDOWS) README.md config.example.yaml
	tar -czf release/$(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz $(BINARY_DARWIN) README.md config.example.yaml

.PHONY: dev-setup
dev-setup:
	$(GOGET) -u golang.org/x/lint/golint
	$(GOGET) -u github.com/goreleaser/goreleaser

.PHONY: docker-build
docker-build:
	docker build -t photo-sorter:$(VERSION) .
	docker tag photo-sorter:$(VERSION) photo-sorter:latest

.PHONY: docker-run
docker-run:
	docker run --rm -v $(PWD)/photos:/photos photo-sorter:latest

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

.DEFAULT_GOAL := help
