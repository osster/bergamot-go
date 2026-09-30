# bergamot-go Build Configuration
MODULE_NAME := bergamot-go
GO_VERSION  := 1.21
BUILD_DIR   := ./build

.PHONY: all setup build clean test lint fmt vet

all: build

## setup: Prepare native dependencies and verify the cgo-linked Go build
setup:
	./scripts/setup.sh

## build: Compile the binary
build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(MODULE_NAME) ./cmd/...

## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)

## test: Run unit tests
test:
	go test -v -race -coverprofile=coverage.out ./pkg/... ./internal/...

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## fmt: Format Go code
fmt:
	go fmt ./...

## vet: Run go vet
vet:
	go vet ./...

## deps: Download and tidy dependencies
deps:
	go mod download
	go mod tidy

## install: Install the binary globally
install:
	go install ./cmd/...
