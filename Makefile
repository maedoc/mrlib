# Go Build Script

.PHONY: all build test clean install

# Binary name
BINARY = mrlib

# Version (override with: make build VERSION=v1.2.3)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

LDFLAGS = -s -w -X github.com/maedoc/mrlib/go/internal/cli.version=$(VERSION)

# Go parameters
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

all: build

build: go/build
	@echo "Building for ${GOOS}/${GOARCH} (version $(VERSION))..."
	cd go && GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags="$(LDFLAGS)" -o ../$(BINARY) ./cmd/mrlib
	@echo "Binary created: $(BINARY)"

go/build:
	cd go && go mod tidy

# Build for specific platforms
build-linux: GOOS = linux
build-linux: GOARCH = amd64
build-linux: build

build-mac: GOOS = darwin
build-mac: GOARCH = amd64
build-mac: build

build-mac-arm64: GOOS = darwin
build-mac-arm64: GOARCH = arm64
build-mac-arm64: build

build-windows: GOOS = windows
build-windows: GOARCH = amd64
build-windows: build

# Build all platforms
build-all: build-linux build-mac build-mac-arm64 build-windows

# Test
test:
	cd go && go test ./...

# Clean
clean:
	rm -f $(BINARY) $(BINARY)-linux $(BINARY)-mac $(BINARY)-mac-arm64 $(BINARY).exe
	cd go && go clean

# Cross-compile all
cross-compile: build-all

# Install locally: install via the Go module proxy into GOBIN
install:
	GOBIN=$(shell go env GOPATH)/bin go install github.com/maedoc/mrlib/go/cmd/mrlib@latest
	@echo "Installed to $(shell go env GOPATH)/bin/mrlib"
