# Changelog

Notable changes to `bergamot-go` are documented here.

## Unreleased

## [v0.1.1] - 2026-10-06

- Fixed loading a second model while another is loaded, which failed with
  "logger with name 'general' already exists". All bridges now share one Bergamot
  service, so native translations are serialized across the process.

## [v0.1.0] - 2026-10-05

- Added a Go library API for configured synchronous, asynchronous, streaming, and context-aware
  translation, with typed errors and configuration reload.
- Added thread-safe shared model ownership, deterministic cleanup, and best-effort finalizers.
- Added CGO bridge error and lifecycle tests, translator and streaming unit tests, and model-backed
  smoke coverage when local model artifacts are available.
- Documented native dependency setup, configuration, API usage, and runnable Go examples.