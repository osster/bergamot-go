# bergamot-go

Go library wrapper around Mozilla's Bergamot neural machine translation engine. The upstream
C++ source is pinned as a Git submodule at `third_party/bergamot-translator/`.

## Quick developer setup

From a fresh clone, run this from the repository root:

```sh
./scripts/setup.sh
```

The script checks for Git, CMake, Make, Go, and C/C++ compilers; initializes the pinned recursive
submodules; applies the local compatibility patches if needed; builds the native CPU library in
`build/bergamot`; and verifies the cgo-linked Go packages with `CGO_ENABLED=1 go build ./...`.
It is safe to rerun. Set `JOBS` to change native build parallelism (default: 2), or run `make setup`
as an equivalent command. The script does not install system packages or download model weights.

## Bergamot native dependency

### Pinned revision and fetch

The parent repository pins `third_party/bergamot-translator/` as a gitlink at
`9271618ebbdc5d21ac4dc4df9e72beb7ce644774`. This superproject gitlink is the revision
checked out for the native-build instructions below; it pins a commit, not a moving branch.
The parent `.gitmodules` entry uses `https://github.com/browsermt/bergamot-translator.git`.
Fetch the pinned submodule and its required nested dependencies with:

```sh
git clone --recurse-submodules <this-project-url>
# or, in an existing clone
git submodule update --init --recursive third_party/bergamot-translator/
```

The recursive checkout includes Marian and ssplit; pybind11 and test fixture submodules
are also checked out, although they are not needed for the default native library target.
The SHA in the gitlink, not an upstream branch name, is the pin.

### Prerequisites

The Go bridge requires cgo, Go 1.21 or newer, Git, CMake (3.5.1 or newer), GNU Make, and a
C++17 compiler. The C compiler should come from the same toolchain. A working network connection
is needed for recursive submodule initialization. The setup script checks that these commands
exist; it does not install them.

The only platform validated end-to-end in this repository is macOS on Apple Silicon. Linux
x86_64 and ARM64 have platform linker selections in the bridge but have not been verified here;
other OS/architecture combinations are unsupported until their cgo flags and native build are
validated. Do not interpret a successful CMake build alone as platform support: the cgo-linked Go
build must also succeed.

For a native CPU build on Ubuntu 20.04/22.04, install the C/C++ toolchain, CMake, Git,
and Bergamot's native dependencies (the upstream CI explicitly installs the protobuf,
Boost, unwind, and google-perftools packages):

```sh
sudo apt-get update
sudo apt-get install -y build-essential cmake git libprotobuf-dev protobuf-compiler \
  libboost-all-dev libunwind-dev libgoogle-perftools-dev
```

On macOS, install Xcode Command Line Tools (Clang, SDK, and Make), CMake, and Git. Homebrew can
provide CMake and Bergamot's native dependencies if they are not already installed:

```sh
brew install cmake protobuf boost libunwind google-perftools
```

MKL and CUDA are not required for the CPU build.

`ccache` is used in CI but is only a build speed-up. MKL is an optional CI optimization,
not required by the CPU library build described below. CUDA is disabled by the upstream
superproject for its Bergamot dependency. Upstream's canonical CPU instructions are in
`third_party/bergamot-translator/README.md`; its dependency install and CI build recipe are
in `third_party/bergamot-translator/.github/workflows/native.yml`. The recursive checkout
also fetches Marian and ssplit, which are required source dependencies.

### Build

The upstream native-build instructions in `third_party/bergamot-translator/README.md`
are (run from the Bergamot submodule):

```sh
cd third_party/bergamot-translator
mkdir build-native
cd build-native
cmake ../
make -j2
```

For this repository, an equivalent fresh out-of-tree build from the project root is:

```sh
cmake -S third_party/bergamot-translator -B build/bergamot -DCMAKE_BUILD_TYPE=Release
cmake --build build/bergamot --target bergamot-translator --parallel 2
```

The target-only command builds the native static library; to also build the upstream
optional CLI, use the default target (`cmake --build build/bergamot --parallel 2`).
Upstream's Ubuntu native CI uses the default target and `make -j2`.

**Validation status:** On macOS arm64, Bergamot commit
`9271618ebbdc5d21ac4dc4df9e72beb7ce644774` was configured in Release mode and built from this
checkout with `cmake --build build/bergamot --target bergamot-translator --parallel 2`; the
resulting archive is `build/bergamot/src/translator/libbergamot-translator.a`. The cgo-linked Go
package build was also verified in this workspace. The pinned submodule sources required small
Clang/macOS compatibility edits for SentencePiece, zlib, and Marian's stack-trace helper.

### Local compatibility patches

The compatibility edits are preserved as patches in this repository rather than as uncommitted
changes to nested submodules:

- `patches/marian-macos-arm64.patch` targets Marian revision
  `2781d735d4a10dca876d61be587afdab2726293c`.
- `patches/sentencepiece-macos-arm64.patch` targets the nested SentencePiece revision
  `ae41b7740d7006596bb9257e83340b2620db9d00`.

After initializing recursive submodules at the pinned revisions above, apply each patch from its
own repository root. Check both patches before applying either:

```sh
MARIAN=third_party/bergamot-translator/3rd_party/marian-dev
SENTENCEPIECE="$MARIAN/src/3rd_party/sentencepiece"
git -C "$MARIAN" apply --check "$PWD/patches/marian-macos-arm64.patch"
git -C "$SENTENCEPIECE" apply --check "$PWD/patches/sentencepiece-macos-arm64.patch"
git -C "$MARIAN" apply "$PWD/patches/marian-macos-arm64.patch"
git -C "$SENTENCEPIECE" apply "$PWD/patches/sentencepiece-macos-arm64.patch"
```

Run those commands from the `bergamot-go` repository root. If either `--check` fails, verify the
submodule revisions and review the patch against the updated source instead of forcing it. These
are local build-compatibility changes; they do not modify Bergamot or Marian upstream history.

### Header and library locations

The `bergamot-translator` CMake target declares these public source include roots
(verified in `src/translator/CMakeLists.txt`):

- `${BERGAMOT_SOURCE_DIR}` (in this checkout: `third_party/bergamot-translator/`)
- `${BERGAMOT_SOURCE_DIR}/src` (in this checkout: `third_party/bergamot-translator/src/`)

The Bergamot target is static and links its Marian and ssplit dependencies. For the root-level
CMake command above, the public source header roots are
`third_party/bergamot-translator/` and `third_party/bergamot-translator/src/`; CMake declares
both on the `bergamot-translator` target. The target archive is generated at
`build/bergamot/src/translator/libbergamot-translator.a` on standard Unix-like toolchains.
In the verified macOS arm64 build, the Marian and ssplit archives are generated at
`build/bergamot/libmarian.a` and `build/bergamot/libssplit.a`; SentencePiece is at
`build/bergamot/3rd_party/marian-dev/src/3rd_party/sentencepiece/src/libsentencepiece.a`, and
other transitive archives are under `build/bergamot/3rd_party/marian-dev/`. CMake-generated
Marian headers and configuration headers are also in the corresponding `build/bergamot/3rd_party/marian-dev/`
build tree (the cgo preamble lists its concrete include roots). These are build-tree paths
(there is no install step in these instructions); confirm actual dependency libraries for the
selected generator/platform in CMake's build output. To verify the main archive and locate the
native static libraries after building, run from the project root:

```sh
test -f build/bergamot/src/translator/libbergamot-translator.a
find build/bergamot -type f \\\( -name 'libbergamot-translator.a' -o -name 'libmarian.a' \\\
  -o -name 'libssplit.a' -o -name 'libsentencepiece.a' \\\) -print
``` The parent gitlink also pins Marian to
`2781d735d4a10dca876d61be587afdab2726293c` and ssplit to
`a311f9865ade34db1e8e080e6cc146f55dafb067` when recursively initialized.

### Go cgo bridge

`pkg/bridge.go` exposes `Init`, `(*Bridge).Translate`, and `(*Bridge).Close` by calling the
C-compatible API in `pkg/bridge.h`. The cgo preamble documents and sets the flags for the
root-level `build/bergamot` build above:

- `CPPFLAGS` include `pkg/`, Bergamot root and `src/`, Marian and its vendored headers
  (including SentencePiece, ruy/cpuinfo, and protobuf-lite), ssplit, plus generated
  `build/bergamot/3rd_party/marian-dev` include roots.
- `CXXFLAGS` selects C++17 for the wrapper and supplies Marian's CPU/SIMD definitions on
  macOS arm64 to match the native CMake target.
- `LDFLAGS` link `build/bergamot/src/translator/libbergamot-translator.a` followed by
  Marian and ssplit archives from `build/bergamot/`, SentencePiece from
  `build/bergamot/3rd_party/marian-dev/src/3rd_party/sentencepiece/src/`, and ruy/cpuinfo/clog
  archives from their Marian build-tree directories, then PCRE2, zlib, pthread, and the platform
  C++/Accelerate libraries. Linux selects `-lstdc++`; macOS selects Accelerate, iconv, and libc++.

Build with `cmake -S third_party/bergamot-translator -B build/bergamot -DCMAKE_BUILD_TYPE=Release`
and `cmake --build build/bergamot --target bergamot-translator --parallel 2`, then compile
Go packages with `CGO_ENABLED=1 go build ./...`. Set `CGO_ENABLED=0` is not supported for
this native bridge. The bridge methods are sequential-only per handle; concurrent access is
left to higher layers.

### Command-line translation

After building the native library, translate text by passing a model configuration and input:

```sh
CGO_ENABLED=1 go run ./cmd/bergamot-go -config path/to/model.yml "Hello, world."
```

When no text argument is supplied, the command reads the translation input from standard input:

```sh
printf 'Hello, world.' | CGO_ENABLED=1 go run ./cmd/bergamot-go -config path/to/model.yml
```

The command initializes the configured model for the translation and closes it before exiting.
Run with `-h` for usage; model weights must be available at the paths referenced by the config.

### Go translation API

The library API accepts a separate YAML or JSON configuration that maps language pairs to
Bergamot model config files. A relative `model_config` path is resolved from the directory
containing this translator configuration:

```yaml
language_pairs:
  en-de:
    model_config: models/mozilla-en-de.yml
    beam_size: 4
    timeout: 30s
    context_window: 3
  fr-en:
    model_config: models/mozilla-fr-en.yml
defaults:
  timeout: 1m
  context_window: 2
```

`defaults` supplies fallback values; each pair may override them. `beam_size` is 0 (or omitted) to
keep the model config value, or 1 to 256 to override it; `timeout` is a Go duration such as `30s`
or `1m`, and `context_window` is the number of prior source sentences to retain (0 to 100).
Pair-level zero values can disable inherited timeouts or context windows. `LoadConfig` validates
the mapping and options before a translator is created.

Create a translator and translate synchronously with the requested pair:

```go
import (
	"fmt"

	"bergamot-go/pkg"
)

func translate() error {
	translator, err := bergamot.NewTranslator("translator.yml")
	if err != nil {
		return err
	}
	defer translator.Close()

	translation, err := translator.Translate("Hello, world.", "en-de")
	if err != nil {
		return err
	}
	fmt.Println(translation)
	return nil
}
```

`NewTranslator` validates the pair mapping and loads each native model on its first use, reusing
it for subsequent translations until `Close`. Check errors with `errors.Is` and the exported
sentinels such as `bergamot.ErrUnsupportedLanguagePair` or `bergamot.ErrTranslation`; the
`*bergamot.Error` also unwraps the underlying parse or native error. Calls through a `Translator`
are serialized so synchronous, asynchronous, streaming, and `Close` operations do not use a
native bridge concurrently. Separate translators share a model when its resolved config path and
beam size match; shared native calls are serialized, and the model remains loaded until its last
translator closes. `Bridge` also serializes direct calls against cleanup. Explicitly call `Close`
when finished; finalizers provide best-effort cleanup if a translator or bridge is abandoned. The
command-line interface above continues to accept a direct
Bergamot model config; the language-pair mapping is for the Go library API.

When `context_window` is enabled, the translator retains recent source sentences per language pair,
prepends them to the next Bergamot request, and returns only the current input's translated suffix.
Call `translator.ResetContext("en-de")` between unrelated documents. Sentence boundaries in the
Go history are detected from common punctuation followed by whitespace; this is a bounded source
history, not model-level cross-sentence attention. `ReloadConfig` validates and applies edits to the
same configuration file, retaining loaded models unless a pair's model path or beam size changed.

The configured timeout bounds how long the caller waits for each synchronous translation. Bergamot's
blocking native API cannot cancel an inference already in progress: after a timeout, that native
call continues in the background under the translator lock, and later calls or `Close` wait for it
to finish. Streaming checks cancellation between chunks, but cannot interrupt its current native
call either.

For non-blocking translation, receive the single result from `TranslateAsync`. Use
`TranslateAsyncWithCallback` when callback delivery is more convenient; it also returns the
result channel:

```go
result := <-translator.TranslateAsync(ctx, "Hello, world.", "en-de")
if result.Err != nil {
	return result.Err
}
fmt.Println(result.Text)
```

For large documents, pass an `io.Reader` to `TranslateStream`. It emits each translated chunk
with cumulative progress and closes the results channel at completion. `chunkRunes` bounds each
chunk by Unicode code points; choose a size appropriate for the model and document. A canceled
context stops work between chunks, but cannot interrupt a native translation already in progress.
Chunk input buffers are pooled up to a bounded capacity and are discarded if a translation times
out while native inference may still be borrowing the input.

```go
for result := range translator.TranslateStream(ctx, reader, "en-de", 2000) {
	if result.Err != nil {
		return result.Err
	}
	fmt.Printf("%d chunks, %d bytes: %s", result.Progress.ChunksCompleted,
		result.Progress.BytesProcessed, result.Text)
}
```

When a context window is configured, the same per-pair source history is maintained across stream
chunks. Each chunk still incurs its own native translation call.

### Local model artifacts

Choose and manage the directory where you keep model artifacts, then point the artifact entries
in a Bergamot YAML config at those files. Relative paths are resolved from the directory containing
the YAML file; absolute paths are also supported. For example, with `model.yml` next to a
`models/` directory:

```yaml
models:
  - models/model.ende.intgemm.alphas.bin
vocabs:
  - models/vocab.ende.spm
  - models/vocab.ende.spm
shortlist:
  - models/lex.50.50.ende.s2t.bin
  - false
```

You can instead use absolute paths such as `/opt/bergamot/models/model.ende.intgemm.alphas.bin`.
Supply your config with `-config path/to/model.yml` as shown above; model downloading and choosing
the artifact directory are user-managed, and the CLI does not provide a separate model-directory
override.

### End-to-end bridge smoke test

`pkg/bridge_smoke_test.go` exercises native initialization, translation using the checked-in
Bergamot input fixture, and cleanup. It uses Mozilla's production EN-DE `base-memory` model;
the test config is in `pkg/testdata/bridge-smoke-mozilla-en-de.yml`. Model artifacts are stored
in the submodule's ignored `models/` directory and downloaded separately. From the repository
root, fetch the release listed in Mozilla's model registry:

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

If the model weights are not present, only `TestBridgeSmoke` is skipped. Set
`BERGAMOT_TEST_MODEL_CONFIG` to use a different Bergamot model config, or
`BERGAMOT_TEST_INPUT` to override the fixture sentence. To validate the linked package build,
run `CGO_ENABLED=1 go build ./...` after building the native library.
