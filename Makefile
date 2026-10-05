# bergamot-go Build Configuration
MODULE_NAME := bergamot-go
GO_VERSION  := 1.21
BUILD_DIR   := ./build

.PHONY: all setup build clean test loadtest lint fmt vet

BENCHTIME ?= 30s
BENCH_CPU ?= 1,2,4,8

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
	go test -v -race -coverprofile=coverage.out ./pkg/...

## loadtest: Benchmark native translation using BENCH_CONFIG and optional BENCH_PAIR
loadtest:
	@test -n "$(BENCH_CONFIG)" || (printf 'Set BENCH_CONFIG to a translator YAML config\n' >&2; exit 1)
	@test -f "$(BENCH_CONFIG)" || (printf 'Benchmark config not found: %s\n' "$(BENCH_CONFIG)" >&2; exit 1)
	BERGAMOT_BENCH_CONFIG="$(abspath $(BENCH_CONFIG))" BERGAMOT_BENCH_LANGUAGE_PAIR="$(BENCH_PAIR)" CGO_ENABLED=1 go test ./pkg -run '^$$' -bench '^BenchmarkLoad' -benchmem -benchtime=$(BENCHTIME) -cpu=$(BENCH_CPU)

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
