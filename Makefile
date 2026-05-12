# Go Build Script

.PHONY: all build test clean

# Binary name
BINARY = mrlib

# Go parameters
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

all: build

build: go/build
	@echo "Building for ${GOOS}/${GOARCH}..."
	cd go && go build -ldflags="-s -w" -o ../$(BINARY) ./cmd/main.go
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
cross-compile: build-linux build-mac build-mac-arm64 build-windows

# Install
install: build
	mv $(BINARY) /usr/local/bin/$(BINARY)
