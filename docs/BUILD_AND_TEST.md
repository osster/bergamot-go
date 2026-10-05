# Building and development

This guide covers native build prerequisites, developer setup, and build/test data. For the Go package API and CLI
usage, see the repository [README](../README.md).

## Quick setup

From the repository root, run:

```sh
./scripts/setup.sh
```

The script initializes recursive submodules, applies local compatibility patches, builds the Bergamot CPU library under
`build/bergamot`, and verifies the cgo-linked Go packages. It checks for Git, CMake, Make, Go, and C/C++ compilers but
does not install system packages or download model weights. It is safe to rerun. Set `JOBS` to change native build
parallelism (default: 2), or run `make setup`.

## Native dependencies and platforms

The bridge requires cgo, Go 1.21 or newer, Git, CMake (3.5.1 or newer), GNU Make, and a C++17 compiler. A network
connection is required to initialize recursive submodules.

The Bergamot submodule is pinned at `9271618ebbdc5d21ac4dc4df9e72beb7ce644774`. Clone recursively or initialize it in an
existing checkout:

```sh
git clone --recurse-submodules <this-project-url>
# or from an existing clone
git submodule update --init --recursive third_party/bergamot-translator/
```

The recursive checkout includes Marian and ssplit, plus pybind11 and test fixture submodules. The gitlink SHA, rather
than an upstream branch name, selects the Bergamot revision.

For a CPU build on Ubuntu 20.04/22.04:

```sh
sudo apt-get update
sudo apt-get install -y build-essential cmake git libprotobuf-dev protobuf-compiler \
  libboost-all-dev libunwind-dev libgoogle-perftools-dev
```

On macOS, install Xcode Command Line Tools, CMake, and Git. Homebrew can provide native dependencies:

```sh
brew install cmake protobuf boost gperftools
```

The macOS SDK provides `libunwind`. MKL and CUDA are not required for the CPU build. The GitHub Actions workflow
validates macOS arm64, Linux x86_64, and Linux ARM64; only macOS Apple Silicon has so far been verified end-to-end in
this repository. A successful CMake build alone does not establish platform support; the cgo-linked Go build must also
succeed.

## Continuous integration

GitHub Actions runs native setup, race-enabled Go tests, and `go vet` on macOS arm64, Linux x86_64, and Linux ARM64 for
pushes, pull requests, and manual runs. Model-backed smoke tests are skipped unless their artifacts are supplied; CI
does not download those artifacts.

## Build

The standard fresh out-of-tree native build from the repository root is:

```sh
cmake -S third_party/bergamot-translator -B build/bergamot -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_POLICY_VERSION_MINIMUM=3.5
cmake --build build/bergamot --target bergamot-translator --parallel 2
CGO_ENABLED=1 go build ./...
```

To also build the upstream optional CLI, omit `--target bergamot-translator`. The target archive is generated at
`build/bergamot/src/translator/libbergamot-translator.a` on standard Unix-like toolchains. The Go bridge requires the
native library and does not support `CGO_ENABLED=0`.

The upstream build instructions and dependency recipe are in
`third_party/bergamot-translator/README.md` and
`third_party/bergamot-translator/.github/workflows/native.yml`.

## Local compatibility patches

Patches preserving local macOS/ARM compatibility changes are stored in `patches/` and are applied by `scripts/setup.sh`;
they are not changes to upstream submodule history:

- `patches/marian-macos-arm64.patch` targets Marian revision
  `2781d735d4a10dca876d61be587afdab2726293c`.
- `patches/marian-arm64-simd.patch` and `patches/marian-arm64-neon.patch` adapt Marian ARM SIMD types for SSE2NEON and
  native NEON math.
- `patches/sentencepiece-macos-arm64.patch` targets SentencePiece revision
  `ae41b7740d7006596bb9257e83340b2620db9d00`.

The recursive checkout also pins ssplit to
`a311f9865ade34db1e8e080e6cc146f55dafb067`.

## Native smoke test

`pkg/bridge_smoke_test.go` tests native initialization, translation, and cleanup using Mozilla's production EN-DE
`base-memory` model. Its config is
`pkg/testdata/bridge-smoke-mozilla-en-de.yml`; model artifacts live in the submodule's ignored
`models/` directory and are downloaded separately. From the repository root:

```sh
MODEL_DIR=third_party/bergamot-translator/models/mozilla-en-de
MODEL_BASE=https://storage.googleapis.com/moz-fx-translations-data--303e-prod-translations-data/models/en-de/retrain_hr_fix_names_SCgGhxUPQ2WAECHLRtzrMg/exported
mkdir -p "$MODEL_DIR"
curl -fL "$MODEL_BASE/model.ende.intgemm.alphas.bin.gz" -o "$MODEL_DIR/model.ende.intgemm.alphas.bin.gz"
curl -fL "$MODEL_BASE/vocab.ende.spm.gz" -o "$MODEL_DIR/vocab.ende.spm.gz"
curl -fL "$MODEL_BASE/lex.50.50.ende.s2t.bin.gz" -o "$MODEL_DIR/lex.50.50.ende.s2t.bin.gz"
gunzip -f "$MODEL_DIR"/*.gz
printf '%s  %s\n' 8df29d9494d19f47fd5d97c6a73474c6f657e9f81c1a607c431d02befdf3810f "$MODEL_DIR/model.ende.intgemm.alphas.bin" | shasum -a 256 -c

CGO_ENABLED=1 go test ./pkg -run '^TestBridgeSmoke$' -v
```

If the model weights are absent, only `TestBridgeSmoke` is skipped. Set
`BERGAMOT_TEST_MODEL_CONFIG` to use another Bergamot model config or `BERGAMOT_TEST_INPUT` to override the fixture
sentence. CI does not download model artifacts.

## Translation load benchmarks

The opt-in benchmarks in `pkg/load_benchmark_test.go` run real model inference and report average latency (`ns/op`),
request/document throughput (`ops/s`), and input throughput (`MB/s`). They exclude model loading by warming the
configured pair before timing. Model and native-library requirements are the same as for the smoke test.

The repository includes a translator config at `tests/bench-translator.yml` that reuses the Mozilla EN-DE smoke-test
model config. Relative `model_config` paths are resolved from the wrapper file. To use another pair or model, create a
wrapper config with the same structure:

```yaml
language_pairs:
  en-de:
    model_config: pkg/testdata/bridge-smoke-mozilla-en-de.yml
    beam_size: 1
```

`BENCH_CONFIG` must point to a translator config, not directly to the native model config at
`pkg/testdata/bridge-smoke-mozilla-en-de.yml`. The Make target resolves relative `BENCH_CONFIG`
paths from the repository root before passing them to Go's package tests, and checks that the file exists before
starting the benchmarks.

Run with the project Make target, varying `BENCHTIME`, `BENCH_CPU`, and `BENCH_PAIR` as needed:

```sh
make loadtest BENCH_CONFIG=tests/bench-interactive.yml BENCH_PAIR=en-de BENCHTIME=30s BENCH_CPU=1,2,4,8
```

```sh
make loadtest BENCH_CONFIG=tests/bench-general.yml BENCH_PAIR=en-de BENCHTIME=30s BENCH_CPU=1,2,4,8
```

```sh
make loadtest BENCH_CONFIG=tests/bench-quality.yml BENCH_PAIR=en-de BENCHTIME=30s BENCH_CPU=1,2,4,8
```

To run the rolling-context benchmark, use its dedicated config, which enables a three-sentence context window:

```sh
make loadtest BENCH_CONFIG=tests/bench-context.yml BENCH_PAIR=en-de BENCHTIME=30s BENCH_CPU=1,2,4,8
```

The benchmarks cover:

- Short, medium, and long synchronous translation requests.
- Asynchronous channel requests and callback-delivered requests, including goroutine/channel overhead.
- Long-document streaming with 128-, 512-, and 2048-rune chunks.
- Rolling source context when `context_window` is enabled for the selected pair.
- Concurrent requests through one translator, and concurrent requests through multiple translator instances sharing the
  same configured model.

`BENCH_CPU` sets `GOMAXPROCS` values used by Go's parallel benchmarks; it is not a limit on the number of waiting client
requests. Calls through a translator and shared native bridge are serialized, so the parallel cases measure queueing and
wrapper overhead as well as throughput, not parallel native inference. The stream benchmark's `ops/s` is completed
documents per second; chunk size changes the number of native calls per document. Use identical model files, beam size,
input, machine power settings, and benchmark options when comparing runs. Run several times and compare the results
rather than treating one short run as a capacity guarantee.

Further workload scenarios to characterize for a deployment include cold process/model startup (measured separately from
these warm benchmarks), mixed short and long requests, sustained versus burst arrival rates, increasing client
concurrency and queue depth, multiple language pairs and models resident at once, beam-size/model variants,
context-window sizes, real document-size and language distributions, stream chunk sizes matched to the application, and
memory/CPU behavior under long-running load. Timeout and cancellation behavior, model reload/close during traffic, and
error rates under invalid or unsupported requests are also useful resilience tests; these benchmarks intentionally
measure successful steady-state inference and do not report latency percentiles.


## Benchmarks

### bench-interactive.yml
```text
goos: darwin
goarch: arm64
pkg: github.com/osster/bergamot-go/pkg
cpu: Apple M3 Pro
BenchmarkLoadTranslate/short                        7974           4500372 ns/op           0.00 MB/s           222.2 ops/s            24 B/op          2 allocs/op
BenchmarkLoadTranslate/short-2                      8367           4505159 ns/op           0.00 MB/s           222.0 ops/s            24 B/op          2 allocs/op
BenchmarkLoadTranslate/short-4                      7975           4339669 ns/op           0.00 MB/s           230.4 ops/s            24 B/op          2 allocs/op
BenchmarkLoadTranslate/short-8                      8367           4313041 ns/op           0.00 MB/s           231.9 ops/s            24 B/op          2 allocs/op
BenchmarkLoadTranslate/medium                       1975          18340339 ns/op           0.00 MB/s            54.52 ops/s          104 B/op          2 allocs/op
BenchmarkLoadTranslate/medium-2                     1940          18711074 ns/op           0.00 MB/s            53.44 ops/s          104 B/op          2 allocs/op
BenchmarkLoadTranslate/medium-4                     1956          18824809 ns/op           0.00 MB/s            53.12 ops/s          104 B/op          2 allocs/op
BenchmarkLoadTranslate/medium-8                     1963          18854546 ns/op           0.00 MB/s            53.04 ops/s          104 B/op          2 allocs/op
BenchmarkLoadTranslate/long                          229         157027519 ns/op           0.01 MB/s             6.368 ops/s        2696 B/op          2 allocs/op
BenchmarkLoadTranslate/long-2                        232         154303309 ns/op           0.01 MB/s             6.481 ops/s        2696 B/op          2 allocs/op
BenchmarkLoadTranslate/long-4                        234         153295740 ns/op           0.01 MB/s             6.523 ops/s        2696 B/op          2 allocs/op
BenchmarkLoadTranslate/long-8                        234         154202332 ns/op           0.01 MB/s             6.485 ops/s        2696 B/op          2 allocs/op
BenchmarkLoadTranslateAsync                         2482          14734668 ns/op           0.00 MB/s            67.87 ops/s          312 B/op          5 allocs/op
BenchmarkLoadTranslateAsync-2                       2431          14933381 ns/op           0.00 MB/s            66.96 ops/s          312 B/op          5 allocs/op
BenchmarkLoadTranslateAsync-4                       2403          15140738 ns/op           0.00 MB/s            66.05 ops/s          313 B/op          5 allocs/op
BenchmarkLoadTranslateAsync-8                       2422          15423328 ns/op           0.00 MB/s            64.84 ops/s          313 B/op          5 allocs/op
BenchmarkLoadTranslateCallback                      2424          14975859 ns/op           0.00 MB/s            66.77 ops/s          440 B/op          7 allocs/op
BenchmarkLoadTranslateCallback-2                    2426          14907175 ns/op           0.00 MB/s            67.08 ops/s          440 B/op          7 allocs/op
BenchmarkLoadTranslateCallback-4                    2401          14996676 ns/op           0.00 MB/s            66.68 ops/s          440 B/op          7 allocs/op
BenchmarkLoadTranslateCallback-8                    2414          14941610 ns/op           0.00 MB/s            66.93 ops/s          440 B/op          7 allocs/op
BenchmarkLoadTranslateStream/chunk_128               138         260838764 ns/op           0.01 MB/s             3.834 ops/s        6712 B/op         39 allocs/op
BenchmarkLoadTranslateStream/chunk_128-2             138         262804876 ns/op           0.01 MB/s             3.805 ops/s        6721 B/op         39 allocs/op
BenchmarkLoadTranslateStream/chunk_128-4             135         263667004 ns/op           0.01 MB/s             3.793 ops/s        6731 B/op         39 allocs/op
BenchmarkLoadTranslateStream/chunk_128-8             134         261939532 ns/op           0.01 MB/s             3.818 ops/s        6726 B/op         39 allocs/op
BenchmarkLoadTranslateStream/chunk_512               223         160438792 ns/op           0.01 MB/s             6.233 ops/s        6568 B/op         15 allocs/op
BenchmarkLoadTranslateStream/chunk_512-2             222         159069342 ns/op           0.01 MB/s             6.287 ops/s        6570 B/op         15 allocs/op
BenchmarkLoadTranslateStream/chunk_512-4             229         158019952 ns/op           0.01 MB/s             6.328 ops/s        6595 B/op         15 allocs/op
BenchmarkLoadTranslateStream/chunk_512-8             228         159120072 ns/op           0.01 MB/s             6.285 ops/s        6602 B/op         15 allocs/op
BenchmarkLoadTranslateStream/chunk_2048              268         134032070 ns/op           0.02 MB/s             7.461 ops/s        6480 B/op          9 allocs/op
BenchmarkLoadTranslateStream/chunk_2048-2            266         135915278 ns/op           0.02 MB/s             7.358 ops/s        6482 B/op          9 allocs/op
BenchmarkLoadTranslateStream/chunk_2048-4            259         134113923 ns/op           0.02 MB/s             7.456 ops/s        6505 B/op          9 allocs/op
BenchmarkLoadTranslateStream/chunk_2048-8            268         135197158 ns/op           0.02 MB/s             7.397 ops/s        6520 B/op          9 allocs/op
BenchmarkLoadTranslateParallel/long                  224         157778800 ns/op           0.01 MB/s             6.338 ops/s        2696 B/op          2 allocs/op
BenchmarkLoadTranslateParallel/long-2                228         157778566 ns/op           0.01 MB/s             6.338 ops/s        2703 B/op          2 allocs/op
BenchmarkLoadTranslateParallel/long-4                228         157863225 ns/op           0.01 MB/s             6.335 ops/s        2708 B/op          2 allocs/op
BenchmarkLoadTranslateParallel/long-8                224         158551351 ns/op           0.01 MB/s             6.307 ops/s        2732 B/op          2 allocs/op
BenchmarkLoadTranslateParallel/short                8178           4489506 ns/op           0.00 MB/s           222.7 ops/s            24 B/op          2 allocs/op
BenchmarkLoadTranslateParallel/short-2              8076           4517600 ns/op           0.00 MB/s           221.4 ops/s            25 B/op          2 allocs/op
BenchmarkLoadTranslateParallel/short-4              8090           4427421 ns/op           0.00 MB/s           225.9 ops/s            25 B/op          2 allocs/op
BenchmarkLoadTranslateParallel/short-8              8148           4439962 ns/op           0.00 MB/s           225.2 ops/s            28 B/op          2 allocs/op
BenchmarkLoadTranslateSeparateTranslators           8126           4418078 ns/op           0.00 MB/s           226.3 ops/s            24 B/op          2 allocs/op
BenchmarkLoadTranslateSeparateTranslators-2         8082           4468527 ns/op           0.00 MB/s           223.8 ops/s            24 B/op          2 allocs/op
BenchmarkLoadTranslateSeparateTranslators-4         7884           4572970 ns/op           0.00 MB/s           218.7 ops/s            26 B/op          2 allocs/op
BenchmarkLoadTranslateSeparateTranslators-8         7944           4540426 ns/op           0.00 MB/s           220.2 ops/s            27 B/op          2 allocs/op
PASS
ok      github.com/osster/bergamot-go/pkg       1966.690s
```

### bench-general.yml
```text
goos: darwin
goarch: arm64
pkg: github.com/osster/bergamot-go/pkg
cpu: Apple M3 Pro
BenchmarkLoadTranslate/long                          244         146652355 ns/op           0.02 MB/s             6.819 ops/s        2696 B/op          2 allocs/op
BenchmarkLoadTranslate/long-2                        242         150418151 ns/op           0.02 MB/s             6.648 ops/s        2696 B/op          2 allocs/op
BenchmarkLoadTranslate/long-4                        244         146143983 ns/op           0.02 MB/s             6.843 ops/s        2696 B/op          2 allocs/op
BenchmarkLoadTranslate/long-8                        246         146572899 ns/op           0.02 MB/s             6.823 ops/s        2696 B/op          2 allocs/op
BenchmarkLoadTranslate/short                        7900           4432734 ns/op           0.00 MB/s           225.6 ops/s            24 B/op          2 allocs/op
BenchmarkLoadTranslate/short-2                      8050           4425235 ns/op           0.00 MB/s           226.0 ops/s            24 B/op          2 allocs/op
BenchmarkLoadTranslate/short-4                      8082           4416511 ns/op           0.00 MB/s           226.4 ops/s            24 B/op          2 allocs/op
BenchmarkLoadTranslate/short-8                      8140           4413966 ns/op           0.00 MB/s           226.6 ops/s            24 B/op          2 allocs/op
BenchmarkLoadTranslate/medium                       1915          19008421 ns/op           0.00 MB/s            52.61 ops/s          104 B/op          2 allocs/op
BenchmarkLoadTranslate/medium-2                     1916          19078457 ns/op           0.00 MB/s            52.42 ops/s          104 B/op          2 allocs/op
BenchmarkLoadTranslate/medium-4                     1911          19276305 ns/op           0.00 MB/s            51.88 ops/s          104 B/op          2 allocs/op
BenchmarkLoadTranslate/medium-8                     1891          18944503 ns/op           0.00 MB/s            52.79 ops/s          104 B/op          2 allocs/op
BenchmarkLoadTranslateAsync                         2416          15161437 ns/op           0.00 MB/s            65.96 ops/s          312 B/op          5 allocs/op
BenchmarkLoadTranslateAsync-2                       2353          15334202 ns/op           0.00 MB/s            65.21 ops/s          312 B/op          5 allocs/op
BenchmarkLoadTranslateAsync-4                       2286          15026136 ns/op           0.00 MB/s            66.55 ops/s          312 B/op          5 allocs/op
BenchmarkLoadTranslateAsync-8                       2418          14953124 ns/op           0.00 MB/s            66.88 ops/s          312 B/op          5 allocs/op
BenchmarkLoadTranslateCallback                      2424          14881415 ns/op           0.00 MB/s            67.20 ops/s          440 B/op          7 allocs/op
BenchmarkLoadTranslateCallback-2                    2427          15104734 ns/op           0.00 MB/s            66.20 ops/s          440 B/op          7 allocs/op
BenchmarkLoadTranslateCallback-4                    2425          14994771 ns/op           0.00 MB/s            66.69 ops/s          440 B/op          7 allocs/op
BenchmarkLoadTranslateCallback-8                    2412          15174837 ns/op           0.00 MB/s            65.90 ops/s          442 B/op          7 allocs/op
BenchmarkLoadTranslateStream/chunk_128               133         261250560 ns/op           0.01 MB/s             3.828 ops/s        6713 B/op         39 allocs/op
BenchmarkLoadTranslateStream/chunk_128-2             136         265626624 ns/op           0.01 MB/s             3.765 ops/s        6720 B/op         39 allocs/op
BenchmarkLoadTranslateStream/chunk_128-4             138         259360037 ns/op           0.01 MB/s             3.856 ops/s        6732 B/op         39 allocs/op
BenchmarkLoadTranslateStream/chunk_128-8             138         259732961 ns/op           0.01 MB/s             3.850 ops/s        6733 B/op         39 allocs/op
BenchmarkLoadTranslateStream/chunk_512               226         159201547 ns/op           0.01 MB/s             6.281 ops/s        6568 B/op         15 allocs/op
BenchmarkLoadTranslateStream/chunk_512-2             226         159628032 ns/op           0.01 MB/s             6.265 ops/s        6572 B/op         15 allocs/op
BenchmarkLoadTranslateStream/chunk_512-4             223         159567381 ns/op           0.01 MB/s             6.267 ops/s        6579 B/op         15 allocs/op
BenchmarkLoadTranslateStream/chunk_512-8             226         158354279 ns/op           0.01 MB/s             6.315 ops/s        6597 B/op         15 allocs/op
BenchmarkLoadTranslateStream/chunk_2048              276         130080031 ns/op           0.02 MB/s             7.688 ops/s        6480 B/op          9 allocs/op
BenchmarkLoadTranslateStream/chunk_2048-2            272         131654561 ns/op           0.02 MB/s             7.596 ops/s        6483 B/op          9 allocs/op
BenchmarkLoadTranslateStream/chunk_2048-4            270         133113226 ns/op           0.02 MB/s             7.512 ops/s        6490 B/op          9 allocs/op
BenchmarkLoadTranslateStream/chunk_2048-8            276         127044956 ns/op           0.02 MB/s             7.871 ops/s        6492 B/op          9 allocs/op
BenchmarkLoadTranslateParallel/short                8316           4342172 ns/op           0.00 MB/s           230.3 ops/s            24 B/op          2 allocs/op
BenchmarkLoadTranslateParallel/short-2              8328           4365360 ns/op           0.00 MB/s           229.1 ops/s            25 B/op          2 allocs/op
BenchmarkLoadTranslateParallel/short-4              8265           4347957 ns/op           0.00 MB/s           230.0 ops/s            26 B/op          2 allocs/op
BenchmarkLoadTranslateParallel/short-8              8284           4407681 ns/op           0.00 MB/s           226.9 ops/s            28 B/op          2 allocs/op
BenchmarkLoadTranslateParallel/long                  248         142501024 ns/op           0.02 MB/s             7.017 ops/s        2696 B/op          2 allocs/op
BenchmarkLoadTranslateParallel/long-2                252         142560774 ns/op           0.02 MB/s             7.015 ops/s        2696 B/op          2 allocs/op
BenchmarkLoadTranslateParallel/long-4                246         143929765 ns/op           0.02 MB/s             6.948 ops/s        2703 B/op          2 allocs/op
BenchmarkLoadTranslateParallel/long-8                252         141534888 ns/op           0.02 MB/s             7.065 ops/s        2748 B/op          2 allocs/op
BenchmarkLoadTranslateSeparateTranslators           8232           4340951 ns/op           0.00 MB/s           230.4 ops/s            24 B/op          2 allocs/op
BenchmarkLoadTranslateSeparateTranslators-2         8306           4456265 ns/op           0.00 MB/s           224.4 ops/s            24 B/op          2 allocs/op
BenchmarkLoadTranslateSeparateTranslators-4         8227           4437506 ns/op           0.00 MB/s           225.4 ops/s            26 B/op          2 allocs/op
BenchmarkLoadTranslateSeparateTranslators-8         7900           4448098 ns/op           0.00 MB/s           224.8 ops/s            26 B/op          2 allocs/op
PASS
ok      github.com/osster/bergamot-go/pkg       1949.815s
```

### bench-context.yml
```text
goos: darwin
goarch: arm64
pkg: github.com/osster/bergamot-go/pkg
cpu: Apple M3 Pro
BenchmarkLoadTranslate/short                        4977           6677334 ns/op           0.00 MB/s           149.8 ops/s           136 B/op          5 allocs/op
BenchmarkLoadTranslate/short-2                      5424           6816367 ns/op           0.00 MB/s           146.7 ops/s           136 B/op          5 allocs/op
BenchmarkLoadTranslate/short-4                      5370           6850732 ns/op           0.00 MB/s           146.0 ops/s           136 B/op          5 allocs/op
BenchmarkLoadTranslate/short-8                      5410           6735430 ns/op           0.00 MB/s           148.5 ops/s           135 B/op          5 allocs/op
BenchmarkLoadTranslate/medium                       1298          28048424 ns/op           0.00 MB/s            35.65 ops/s          456 B/op          6 allocs/op
BenchmarkLoadTranslate/medium-2                     1302          28239325 ns/op           0.00 MB/s            35.41 ops/s          456 B/op          6 allocs/op
BenchmarkLoadTranslate/medium-4                     1282          27767218 ns/op           0.00 MB/s            36.01 ops/s          456 B/op          6 allocs/op
BenchmarkLoadTranslate/medium-8                     1299          27837116 ns/op           0.00 MB/s            35.92 ops/s          455 B/op          6 allocs/op
BenchmarkLoadTranslate/long                          231         156462160 ns/op           0.01 MB/s             6.391 ops/s        8248 B/op         49 allocs/op
BenchmarkLoadTranslate/long-2                        217         161798972 ns/op           0.01 MB/s             6.181 ops/s        8248 B/op         49 allocs/op
BenchmarkLoadTranslate/long-4                        224         161764067 ns/op           0.01 MB/s             6.182 ops/s        8248 B/op         49 allocs/op
BenchmarkLoadTranslate/long-8                        223         161196704 ns/op           0.01 MB/s             6.204 ops/s        8248 B/op         49 allocs/op
BenchmarkLoadTranslateAsync                         1518          24036518 ns/op           0.00 MB/s            41.60 ops/s          615 B/op          9 allocs/op
BenchmarkLoadTranslateAsync-2                       1395          23450385 ns/op           0.00 MB/s            42.64 ops/s          617 B/op          9 allocs/op
BenchmarkLoadTranslateAsync-4                       1552          23890125 ns/op           0.00 MB/s            41.86 ops/s          619 B/op          9 allocs/op
BenchmarkLoadTranslateAsync-8                       1491          24049463 ns/op           0.00 MB/s            41.58 ops/s          617 B/op          9 allocs/op
BenchmarkLoadTranslateCallback                      1539          23562603 ns/op           0.00 MB/s            42.44 ops/s          743 B/op         11 allocs/op
BenchmarkLoadTranslateCallback-2                    1506          23610954 ns/op           0.00 MB/s            42.35 ops/s          747 B/op         11 allocs/op
BenchmarkLoadTranslateCallback-4                    1562          23097208 ns/op           0.00 MB/s            43.30 ops/s          746 B/op         11 allocs/op
BenchmarkLoadTranslateCallback-8                    1520          23598412 ns/op           0.00 MB/s            42.38 ops/s          745 B/op         11 allocs/op
BenchmarkLoadTranslateStream/chunk_128                80         453905415 ns/op           0.00 MB/s             2.203 ops/s       14044 B/op        143 allocs/op
BenchmarkLoadTranslateStream/chunk_128-2              79         446935226 ns/op           0.00 MB/s             2.237 ops/s       14048 B/op        143 allocs/op
BenchmarkLoadTranslateStream/chunk_128-4              80         448876717 ns/op           0.00 MB/s             2.228 ops/s       14050 B/op        143 allocs/op
BenchmarkLoadTranslateStream/chunk_128-8              80         441662606 ns/op           0.00 MB/s             2.264 ops/s       14067 B/op        143 allocs/op
BenchmarkLoadTranslateStream/chunk_512               177         203428454 ns/op           0.01 MB/s             4.916 ops/s       12576 B/op         82 allocs/op
BenchmarkLoadTranslateStream/chunk_512-2             175         203301910 ns/op           0.01 MB/s             4.919 ops/s       12581 B/op         82 allocs/op
BenchmarkLoadTranslateStream/chunk_512-4             176         204393681 ns/op           0.01 MB/s             4.893 ops/s       12584 B/op         82 allocs/op
BenchmarkLoadTranslateStream/chunk_512-8             174         206079660 ns/op           0.01 MB/s             4.852 ops/s       12596 B/op         82 allocs/op
BenchmarkLoadTranslateStream/chunk_2048              244         149641372 ns/op           0.01 MB/s             6.683 ops/s       12200 B/op         60 allocs/op
BenchmarkLoadTranslateStream/chunk_2048-2            241         149907911 ns/op           0.01 MB/s             6.671 ops/s       12221 B/op         60 allocs/op
BenchmarkLoadTranslateStream/chunk_2048-4            204         147910303 ns/op           0.01 MB/s             6.761 ops/s       12205 B/op         60 allocs/op
BenchmarkLoadTranslateStream/chunk_2048-8            249         146564291 ns/op           0.01 MB/s             6.823 ops/s       12207 B/op         60 allocs/op
BenchmarkLoadTranslateContext                       2108          17649589 ns/op           0.00 MB/s            56.66 ops/s          343 B/op          5 allocs/op
BenchmarkLoadTranslateContext-2                     2062          17796662 ns/op           0.00 MB/s            56.19 ops/s          343 B/op          5 allocs/op
BenchmarkLoadTranslateContext-4                     2091          17633499 ns/op           0.00 MB/s            56.71 ops/s          343 B/op          5 allocs/op
BenchmarkLoadTranslateContext-8                     1957          17361739 ns/op           0.00 MB/s            57.60 ops/s          343 B/op          5 allocs/op
BenchmarkLoadTranslateParallel/short                5221           7336959 ns/op           0.00 MB/s           136.3 ops/s           136 B/op          5 allocs/op
BenchmarkLoadTranslateParallel/short-2              5174           6810003 ns/op           0.00 MB/s           146.8 ops/s           136 B/op          5 allocs/op
BenchmarkLoadTranslateParallel/short-4              5139           6764490 ns/op           0.00 MB/s           147.8 ops/s           138 B/op          5 allocs/op
BenchmarkLoadTranslateParallel/short-8              5386           6938779 ns/op           0.00 MB/s           144.1 ops/s           141 B/op          5 allocs/op
BenchmarkLoadTranslateParallel/long                  224         160480883 ns/op           0.01 MB/s             6.231 ops/s        8248 B/op         49 allocs/op
BenchmarkLoadTranslateParallel/long-2                224         163493970 ns/op           0.01 MB/s             6.116 ops/s        8277 B/op         49 allocs/op
BenchmarkLoadTranslateParallel/long-4                220         160673520 ns/op           0.01 MB/s             6.224 ops/s        8256 B/op         49 allocs/op
BenchmarkLoadTranslateParallel/long-8                230         155453973 ns/op           0.01 MB/s             6.433 ops/s        8282 B/op         49 allocs/op
BenchmarkLoadTranslateSeparateTranslators           5212           6719835 ns/op           0.00 MB/s           148.8 ops/s           136 B/op          5 allocs/op
BenchmarkLoadTranslateSeparateTranslators-2         5102           6928629 ns/op           0.00 MB/s           144.3 ops/s           136 B/op          5 allocs/op
BenchmarkLoadTranslateSeparateTranslators-4         4732           6968582 ns/op           0.00 MB/s           143.5 ops/s           139 B/op          5 allocs/op
BenchmarkLoadTranslateSeparateTranslators-8         5347           6747032 ns/op           0.00 MB/s           148.2 ops/s           136 B/op          5 allocs/op
PASS
ok      github.com/osster/bergamot-go/pkg       2121.246s
```

### bench-quality.yml
```text
goos: darwin
goarch: arm64
pkg: github.com/osster/bergamot-go/pkg
cpu: Apple M3 Pro
BenchmarkLoadTranslate/short                        6705           5315667 ns/op           0.00 MB/s           188.1 ops/s            24 B/op          2 allocs/op
BenchmarkLoadTranslate/short-2                      6756           5347804 ns/op           0.00 MB/s           187.0 ops/s            24 B/op          2 allocs/op
BenchmarkLoadTranslate/short-4                      6762           5392830 ns/op           0.00 MB/s           185.4 ops/s            24 B/op          2 allocs/op
BenchmarkLoadTranslate/short-8                      6840           5348006 ns/op           0.00 MB/s           187.0 ops/s            24 B/op          2 allocs/op
BenchmarkLoadTranslate/medium                       1635          22651819 ns/op           0.00 MB/s            44.15 ops/s          104 B/op          2 allocs/op
BenchmarkLoadTranslate/medium-2                     1586          22624819 ns/op           0.00 MB/s            44.20 ops/s          104 B/op          2 allocs/op
BenchmarkLoadTranslate/medium-4                     1633          22093414 ns/op           0.00 MB/s            45.26 ops/s          104 B/op          2 allocs/op
BenchmarkLoadTranslate/medium-8                     1532          22084663 ns/op           0.00 MB/s            45.28 ops/s          104 B/op          2 allocs/op
BenchmarkLoadTranslate/long                          100         317398275 ns/op           0.01 MB/s             3.151 ops/s        2696 B/op          2 allocs/op
BenchmarkLoadTranslate/long-2                        100         319713805 ns/op           0.01 MB/s             3.128 ops/s        2696 B/op          2 allocs/op
BenchmarkLoadTranslate/long-4                        100         318332812 ns/op           0.01 MB/s             3.141 ops/s        2696 B/op          2 allocs/op
BenchmarkLoadTranslate/long-8                        100         318489996 ns/op           0.01 MB/s             3.140 ops/s        2696 B/op          2 allocs/op
BenchmarkLoadTranslateAsync                         2019          17462534 ns/op           0.00 MB/s            57.27 ops/s          312 B/op          5 allocs/op
BenchmarkLoadTranslateAsync-2                       2052          17633289 ns/op           0.00 MB/s            56.71 ops/s          312 B/op          5 allocs/op
BenchmarkLoadTranslateAsync-4                       2038          17380248 ns/op           0.00 MB/s            57.54 ops/s          313 B/op          5 allocs/op
BenchmarkLoadTranslateAsync-8                       2077          17663459 ns/op           0.00 MB/s            56.61 ops/s          313 B/op          5 allocs/op
BenchmarkLoadTranslateCallback                      2059          17593901 ns/op           0.00 MB/s            56.84 ops/s          440 B/op          7 allocs/op
BenchmarkLoadTranslateCallback-2                    2083          17467230 ns/op           0.00 MB/s            57.25 ops/s          440 B/op          7 allocs/op
BenchmarkLoadTranslateCallback-4                    2100          17495051 ns/op           0.00 MB/s            57.16 ops/s          440 B/op          7 allocs/op
BenchmarkLoadTranslateCallback-8                    2056          17680872 ns/op           0.00 MB/s            56.56 ops/s          441 B/op          7 allocs/op
BenchmarkLoadTranslateStream/chunk_128                84         462501485 ns/op           0.00 MB/s             2.162 ops/s        6716 B/op         39 allocs/op
BenchmarkLoadTranslateStream/chunk_128-2              84         435494879 ns/op           0.00 MB/s             2.296 ops/s        6724 B/op         39 allocs/op
BenchmarkLoadTranslateStream/chunk_128-4              84         434391828 ns/op           0.00 MB/s             2.302 ops/s        6733 B/op         39 allocs/op
BenchmarkLoadTranslateStream/chunk_128-8              84         438497903 ns/op           0.00 MB/s             2.281 ops/s        6734 B/op         39 allocs/op
BenchmarkLoadTranslateStream/chunk_512               124         284853214 ns/op           0.01 MB/s             3.511 ops/s        6569 B/op         15 allocs/op
BenchmarkLoadTranslateStream/chunk_512-2             124         285464069 ns/op           0.01 MB/s             3.503 ops/s        6571 B/op         15 allocs/op
BenchmarkLoadTranslateStream/chunk_512-4             127         283021503 ns/op           0.01 MB/s             3.533 ops/s        6572 B/op         15 allocs/op
BenchmarkLoadTranslateStream/chunk_512-8             126         283308326 ns/op           0.01 MB/s             3.530 ops/s        6602 B/op         15 allocs/op
BenchmarkLoadTranslateStream/chunk_2048              148         242952854 ns/op           0.01 MB/s             4.116 ops/s        6480 B/op          9 allocs/op
BenchmarkLoadTranslateStream/chunk_2048-2            148         244126629 ns/op           0.01 MB/s             4.096 ops/s        6539 B/op          9 allocs/op
BenchmarkLoadTranslateStream/chunk_2048-4            138         265848561 ns/op           0.01 MB/s             3.762 ops/s        6483 B/op          9 allocs/op
BenchmarkLoadTranslateStream/chunk_2048-8            144         246757090 ns/op           0.01 MB/s             4.053 ops/s        6519 B/op          9 allocs/op
BenchmarkLoadTranslateParallel/short                6609           5463762 ns/op           0.00 MB/s           183.0 ops/s            24 B/op          2 allocs/op
BenchmarkLoadTranslateParallel/short-2              6688           5444718 ns/op           0.00 MB/s           183.7 ops/s            25 B/op          2 allocs/op
BenchmarkLoadTranslateParallel/short-4              6685           5416028 ns/op           0.00 MB/s           184.6 ops/s            27 B/op          2 allocs/op
BenchmarkLoadTranslateParallel/short-8              6655           5736218 ns/op           0.00 MB/s           174.3 ops/s            29 B/op          2 allocs/op
BenchmarkLoadTranslateParallel/long                  100         346600144 ns/op           0.01 MB/s             2.885 ops/s        2697 B/op          2 allocs/op
BenchmarkLoadTranslateParallel/long-2                100         336691044 ns/op           0.01 MB/s             2.970 ops/s        2729 B/op          2 allocs/op
BenchmarkLoadTranslateParallel/long-4                100         335798812 ns/op           0.01 MB/s             2.978 ops/s        2757 B/op          2 allocs/op
BenchmarkLoadTranslateParallel/long-8                100         334936555 ns/op           0.01 MB/s             2.986 ops/s        2782 B/op          2 allocs/op
BenchmarkLoadTranslateSeparateTranslators           6818           5344877 ns/op           0.00 MB/s           187.1 ops/s            24 B/op          2 allocs/op
BenchmarkLoadTranslateSeparateTranslators-2         6811           5358174 ns/op           0.00 MB/s           186.6 ops/s            24 B/op          2 allocs/op
BenchmarkLoadTranslateSeparateTranslators-4         6676           5376697 ns/op           0.00 MB/s           186.0 ops/s            27 B/op          2 allocs/op
BenchmarkLoadTranslateSeparateTranslators-8         6640           5483056 ns/op           0.00 MB/s           182.4 ops/s            27 B/op          2 allocs/op
PASS
ok      github.com/osster/bergamot-go/pkg       1819.675s
```