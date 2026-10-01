# Changelog

Notable changes to `bergamot-go` are documented here.

## Unreleased

- Added a Go library API for configured synchronous, asynchronous, streaming, and context-aware
  translation, with typed errors and configuration reload.
- Added thread-safe shared model ownership, deterministic cleanup, and best-effort finalizers.
- Added CGO bridge error and lifecycle tests, translator and streaming unit tests, and model-backed
  smoke coverage when local model artifacts are available.
- Documented native dependency setup, configuration, API usage, and runnable Go examples.

### Release prerequisites

- The configured Git remote establishes `github.com/osster/bergamot-go` as the module path, now
  declared in `go.mod`.
- The project owner is confirmed as release owner; choose a version, then create a tag and publish.
- Validate the selected macOS arm64, Linux x86_64, and Linux ARM64 targets with the pinned native
  dependency and `CGO_ENABLED=1 go test -race ./...`; real-model smoke coverage requires local
  model artifacts.