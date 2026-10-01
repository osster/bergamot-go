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

**Status:** planned  

Implement context window maintenance to preserve accuracy across sentences. Add support for advanced parameters like beam size and timeout. Refine configuration loading logic.

#### Tasks

##### Add parameter validation

**Status:** todo

Validate language pairs, window sizes, and other configuration parameters before use

##### Add support for configurable beam size and timeout

**Status:** todo

Expose Bergamot parameters like beam size and timeout through configuration

##### Implement context window maintenance

**Status:** todo

Track and pass previous sentences to Bergamot engine for improved translation coherence

##### Refine configuration loading logic

**Status:** todo

Implement hot-reload support and default value fallbacks for configuration loading

### Concurrency & Memory Optimization

**Status:** planned  

Ensure thread safety for concurrent translation requests. Optimize memory management for model inference and garbage collection. Handle resource cleanup and connection pooling efficiently.

#### Tasks

##### Add resource cleanup and finalizer hooks

**Status:** todo

Ensure CGO resources are freed on Go GC using finalizers and proper cleanup routines

##### Implement model reference counting and pooling

**Status:** todo

Share loaded models across multiple translator instances safely using reference counting

##### Implement mutex/rwlock mechanisms for thread safety

**Status:** todo

Protect shared model state during concurrent translation requests using mutex/rwlock mechanisms

##### Optimize memory management for model inference

**Status:** todo

Reduce GC pressure by implementing object pooling for frequently allocated translation buffers

### Testing, Documentation & Release

**Status:** planned  

Write comprehensive unit and integration tests. Generate detailed API documentation with usage examples. Finalize packaging and prepare for release.

#### Tasks

##### Finalize packaging and prepare for release

**Status:** todo

Tag versions, write CHANGELOG, and publish package to pkg.go.dev

##### Generate API documentation with usage examples

**Status:** todo

Add comprehensive godoc comments and example programs demonstrating usage patterns

##### Write integration tests for CGO bridge

**Status:** complete

Write end-to-end translation tests using real Bergamot models to validate CGO bridge functionality

`pkg/bridge_smoke_test.go` covers model initialization, real translation, cleanup, and behavior after cleanup when model artifacts are available; it supports an externally supplied model config.

##### Write unit tests for core translation functions

**Status:** todo

Write unit tests covering sync/async APIs, config parsing, error cases, and edge conditions
