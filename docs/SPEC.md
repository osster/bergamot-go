# Project purpose

**Type:** general  
**Status:** accepted

## Description

A configurable and robust Go library wrapper for the Bergamot neural machine translation engine, featuring safe concurrency, memory management, and easy integration.

## Overview

A configurable and robust Go library wrapper for the Bergamot neural machine translation engine. Features include support for all language pairs via configuration, synchronous and asynchronous/streaming APIs, context window maintenance, custom model directories, thread safety, and memory-efficient model management.

## Objectives

- Deliver a production-ready Go library wrapping Bergamot's C++ translation engine
- Ensure thread-safe concurrent access to translation models via CGO
- Provide both synchronous and asynchronous/streaming APIs for flexible integration
- Maintain contextual accuracy across sentences using configurable window sizes
- Optimize memory efficiency for model loading and inference in long-running processes
- Support dynamic configuration of language pairs and parameters without recompilation

## Scope

### Included

- Go package library
- Local execution
- Config-driven language pairs
- Sync/async APIs
- Streaming translation
- Context-aware translation

### Excluded

- Remote service deployment
- Model training
- GUI/web interface

## Requirements

### Functional

- Support all Bergamot language pairs via configuration file
- Synchronous translation API for simple use cases
- Asynchronous/streaming API for large documents
- Context window maintenance across sentences
- Custom download directory for models
- Thread safety for concurrent requests
- Memory-efficient model management
- Configurable parameters (beam size, timeout, etc.)

### Technical

- CGO interface with Bergamot C++ library
- Go 1.21+ support
- Comprehensive error handling
- Unit tests
- API documentation with examples

## Constraints

### UX

- None

### Other

- Requires C/C++ compiler toolchain for CGO builds
- Cross-compilation limited by CGO dependencies
- Minimum system RAM depends on loaded translation models
- Network bandwidth required for initial model downloads

## Risks

- Bergamot model availability and licensing for target language pairs
- CGO compatibility across different OS/platforms (Linux, macOS, Windows)
- Memory pressure from large translation models in long-running processes
- Performance degradation with very long documents or context windows
- Potential C++ library version incompatibilities
- Complexity of managing concurrent CGO calls safely

## Timeline

- None

## Milestones and tasks

### Foundation & Model Management

**Status:** complete

Set up project structure and Go module. Implement the CGO bridge with the Bergamot C++ library. Build the core model loading/unloading mechanism with support for custom directories.

#### Tasks

##### Support user-managed local model artifact paths

**Status:** complete

Users configure model, vocabulary, and shortlist artifact paths in the Bergamot YAML file. Paths
may be relative to the config file or absolute; users choose and manage the artifact directory,
and model downloading is out of scope.

##### Build core model loading mechanism

**Status:** complete

The CGO bridge loads a translation model from the user-provided Bergamot YAML configuration and initializes the native service. Missing configurations return errors, and the end-to-end smoke test covers model initialization, translation, and cleanup.

##### Implement CGO bridge with Bergamot C++ library

**Status:** complete

The pinned Bergamot submodule build, C-compatible ownership-safe wrapper, cgo linkage, and Go error propagation are implemented and documented. Tests cover native error paths and an end-to-end init → translate → cleanup smoke path when model artifacts are available.

## Scope
Implement the Go ↔ Bergamot C++ bridge only. Model downloading, higher-level Go APIs, and concurrency are out of scope unless required to compile or validate the bridge.

## Steps
1. **Preparation** — The upstream source is already present as a Git submodule at `third_party/bergamot-translator/`. Pin the current revision (record the commit SHA) and verify it builds per the upstream instructions. Document: fetch command, build commands, required system dependencies, and the resulting include/library paths.
2. **C-compatible wrapper** — Create a minimal C header (`bridge.h`) and C++ implementation (`bridge.cpp`) that exposes only the translation operations needed by Go (e.g., init, translate, cleanup). Define ownership and lifetime rules for every C++ object and buffer that crosses the boundary.
3. **CGO exposure** — Wire the wrapper into the Go package via cgo. Set CGO `CPPFLAGS` and `LDFLAGS` so the bridge compiles and links against the built Bergamot library. Document these flags in a README or comment block.
4. **Error handling** — Translate any C++ exceptions or error codes into idiomatic Go errors.
5. **Smoke test** — Add a focused Go test that exercises the exposed functions end-to-end (init → translate → cleanup) using whatever fixtures are available from the submodule.

##### Implement model unloading and lifecycle management

**Status:** complete

`Bridge.Close` releases the native service/model handle and is safe to call repeatedly; `Init` creates a fresh handle for reload. Tests cover idempotent cleanup, rejected use after close, and a model-backed close → re-init → translate lifecycle when artifacts are available.

##### Initialize Go module and project structure

**Status:** complete

Initialize the Git repository and establish the bergamot-go scaffold: configure go.mod, directory layout, build configuration, and initial project structure. Add Bergamot translator under third_party as a pinned Git submodule so the upstream dependency is reproducible; document the submodule initialization/update steps. Do not implement the CGO bridge or model loading in this task.

The Go module, package and CLI layout, build configuration, pinned Bergamot submodule, setup script, and submodule initialization instructions are in place.

### Core Translation Engine

**Status:** complete

The Go package parses language-pair configuration, lazily initializes and reuses the matching native model, and exposes synchronous translation with typed errors. `Translator.Close` releases loaded models; this API is sequential-only pending the concurrency milestone.

#### Tasks

##### Add configuration parsing for supported language pairs

**Status:** complete

`LoadConfig` parses and validates YAML or JSON `language_pairs` entries with `model_config` paths. Relative model paths resolve against the translator configuration file.

##### Create Translator struct and initialization logic

**Status:** complete

`NewTranslator` constructs a translator from the config file and lazily initializes and caches each pair's bridge on first use.

##### Establish comprehensive error handling framework

**Status:** complete

`*Error` adds operation and language-pair context while unwrapping parser and native errors. Exported sentinels distinguish invalid configuration, unsupported pairs, initialization, translation, closed-translator, empty-input, and cleanup failures.

##### Implement synchronous translation API

**Status:** complete

`(*Translator).Translate(text, languagePair)` dispatches to the configured bridge and wraps failures. Tests cover parsing, path resolution, bridge reuse, validation, and error propagation.

##### Implement batch translation

**Status:** complete

`(*Translator).TranslateMultiple(texts, languagePair)` translates all texts in one native call
through `Bridge.TranslateMultiple` and the C function `bergamot_translate_multiple`. Inputs cross
the boundary as one byte buffer plus a length array (Go memory without Go pointers, as cgo
requires); the bridge returns one malloc'd buffer with all translations and fills a caller-owned
array of output lengths, so a call costs one allocation each way regardless of the number of
texts. Bridges without a batch call (test fakes) fall back to one `Translate` per text. Unit tests
cover the batch path, the fallback, empty input, unsupported pairs, native failures, timeout and
use after close; the model-backed smoke test checks that a batch matches single translations and
passes empty inputs through.

### Asynchronous & Streaming APIs

**Status:** complete

`Translator` provides channel-based asynchronous translation and bounded reader-based chunk
streaming with cumulative progress. Its mutex serializes translation and cleanup around the
sequential-only native bridge. Streaming cancellation is checked between chunks and cannot stop an
in-flight native call. The bridge borrows Go input string bytes for the duration of the CGO call,
avoiding an intermediate `C.CString` copy; native output is copied into Go before its C buffer is
released.

#### Tasks

##### Develop asynchronous translation interface

**Status:** complete

`(*Translator).TranslateAsync(ctx, text, languagePair)` dispatches in a goroutine and returns a
single buffered channel result. `TranslateAsyncWithCallback` additionally invokes a callback with
that result. Cancellation before work starts is reported through the result and callback.

##### Implement streaming translation for large documents

**Status:** complete

`(*Translator).TranslateStream(ctx, reader, languagePair, chunkRunes)` translates bounded,
Unicode-safe chunks from an `io.Reader` and reports chunk index, completed count, and cumulative
input bytes with each result. Errors terminate the stream after an error result.

##### Optimize data flow between Go and C++ layers

**Status:** complete

The Go string input is borrowed directly for the synchronous native call instead of first copying
through `C.CString`. Streaming bounds document-side memory to a chunk; the native response still
requires an owned buffer and a Go string copy to safely release C memory.

### Context Awareness & Configuration

**Status:** complete

The translator validates per-pair settings and inherited defaults, supports model-time beam-size overrides, caller-side timeouts, bounded source-sentence history, and explicit configuration reload. Context history is prepended to requests and prior-context output is removed; Bergamot's blocking API cannot interrupt an active inference, and its sentence-level translation does not provide model-level cross-sentence attention.

#### Tasks

##### Add parameter validation

**Status:** complete

`LoadConfig` validates language-pair identifiers, required model paths, beam sizes (0 to keep the model value, or 1–256), positive timeouts up to 24 hours, and context windows (0–100 sentences), with pair overrides applied over defaults.

##### Add support for configurable beam size and timeout

**Status:** complete

Per-pair and default `beam_size` overrides are applied before native model construction. Configured `timeout` values bound caller wait time; an active native inference is not cancellable and retains the translator lock until it completes.

##### Implement context window maintenance

**Status:** complete

The translator retains a bounded source-sentence window for each language pair, prepends it to the next request, returns only the translation suffix for current input, and exposes `ResetContext` for unrelated documents. Sentence boundaries use a Go punctuation heuristic; Bergamot itself processes sentences independently and does not provide cross-sentence attention.

##### Refine configuration loading logic

**Status:** complete

`ReloadConfig` validates the updated file before applying it, retains unchanged loaded models, and closes models removed or changed by model path/beam size. Defaults inherit into each pair and explicit pair overrides are supported, including zero to disable an inherited timeout or context window.

### Concurrency & Memory Optimization

**Status:** complete

Concurrent translator and bridge access is serialized around each shared native model. Translators with the same resolved model-config path and beam size share a reference-counted model, and explicit close plus best-effort finalizers release native resources. Streaming reuses bounded chunk buffers, timed-out input buffers are not recycled while native calls may still borrow them, and native output copying avoids an intermediate substring allocation.

#### Tasks

##### Add resource cleanup and finalizer hooks

**Status:** complete

`Translator` and `Bridge` finalizers provide best-effort cleanup when callers omit `Close`; explicit idempotent cleanup remains the deterministic lifecycle path. Direct bridge translation and cleanup are mutex-protected.

##### Implement model reference counting and pooling

**Status:** complete

`Translator` instances share a model keyed by resolved model-config path and beam size. Reference counts retain it until the last translator closes; tests verify initialization reuse and final-owner cleanup.

##### Implement mutex/rwlock mechanisms for thread safety

**Status:** complete

Translator state, each bridge handle, and shared model calls are protected by mutexes. Concurrent calls into a shared sequential native service are serialized, and tests verify cross-translator safety.

##### Optimize memory management for model inference

**Status:** complete

`TranslateStream` reuses bounded byte buffers from a `sync.Pool`, discarding buffers after timeout while native inference may still borrow their contents. Context history clones retained sentences, and the C++ bridge copies output directly into its owned result buffer without allocating an intermediate substring.

### Testing, Documentation & Release

**Status:** in progress

Core unit and CGO integration coverage, package/API documentation, runnable examples, an unreleased changelog, and a GitHub Actions validation workflow are in place. The configured GitHub remote establishes `github.com/osster/bergamot-go` as the canonical module path, and the project owner is confirmed as release owner. The intended release matrix is macOS arm64, Linux x86_64, and Linux ARM64; Linux validation and public release publication remain pending until the workflow passes on those targets.

#### Tasks

##### Finalize packaging and prepare for release

**Status:** in progress

`go.mod` and Go import examples use the canonical module path `github.com/osster/bergamot-go`, matching the configured Git remote. The project owner is the confirmed release owner. `CHANGELOG.md` records the unreleased work; version tagging and pkg.go.dev publication remain pending until the selected macOS arm64, Linux x86_64, and Linux ARM64 targets are validated and a version is chosen.

##### Generate API documentation with usage examples

**Status:** complete

Exported package APIs and their configuration/result fields have Go documentation comments. `pkg/example_test.go` contains executable examples for config validation and lazy translator construction; the README documents translation and streaming usage.

##### Write integration tests for CGO bridge

**Status:** complete

Write end-to-end translation tests using real Bergamot models to validate CGO bridge functionality

`pkg/bridge_smoke_test.go` covers model initialization, real translation, cleanup, and behavior after cleanup when model artifacts are available; it supports an externally supplied model config.

##### Write unit tests for core translation functions

**Status:** complete

Unit tests cover YAML/JSON parsing and validation, synchronous dispatch and lifecycle, error wrapping, context/reload, asynchronous cancellation and callbacks, streaming progress/errors/cancellation, and pooled-model concurrency and cleanup.
