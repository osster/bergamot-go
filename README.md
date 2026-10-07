# bergamot-go

[![CI](https://github.com/osster/bergamot-go/actions/workflows/ci.yml/badge.svg)](https://github.com/osster/bergamot-go/actions/workflows/ci.yml)

Go library wrapper around Mozilla's Bergamot neural machine translation engine. The upstream
C++ source is pinned as a Git submodule at `third_party/bergamot-translator/`.

## Usage

Install prerequisites and build the native library as described in [docs/BUILD_AND_TEST.md](docs/BUILD_AND_TEST.md).
That guide also covers developer setup, compatibility patches, and native smoke tests.
For model sources, available language pairs, and the model used by the smoke test, see
[docs/MODELS.md](docs/MODELS.md).

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

	"github.com/osster/bergamot-go/pkg"
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
translator closes. All models in a process share one Bergamot service, so native translations run one
at a time across the process, even for different models. `Bridge` also serializes direct calls against cleanup. Explicitly call `Close`
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

To translate many texts at once, for example the messages of a chat, use `TranslateMultiple`.
It sends all texts to Bergamot in one native call, so their sentences are packed into shared
mini-batches (`mini-batch-words` in the model config) instead of one nearly empty batch per text:

```go
translations, err := translator.TranslateMultiple([]string{"Hello!", "See you tomorrow."}, "en-de")
```

Results are in input order. Every text must be non-empty, or the call fails with
`ErrEmptyInput`. It neither uses nor updates the context history, and the pair's `timeout` bounds
the whole call. A text may translate slightly differently in a batch than alone: with a lexical
shortlist in the model config, the allowed output words are computed per batch, from all of its
sentences. Without a shortlist, batch and single output match except for rare floating-point
differences. One native call holds the process-wide service lock for its whole duration, so split
very large batches if other translations must not wait behind them.

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

Runnable examples for file-based configuration, an in-memory config object, and a bounded
message/worker pool are available in [examples/](examples/README.md). These keep application-level
configuration and worker orchestration outside the wrapper package.

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
override. For Mozilla model sources and current language-pair availability, see
[docs/MODELS.md](docs/MODELS.md).
