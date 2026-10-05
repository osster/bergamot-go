# Translation models

This project provides a Go wrapper and the Bergamot translation engine; it does not include
translation model weights or download them automatically. You choose the model files, download
them separately, and point a Bergamot YAML config at them. The Go API's language-pair config then
maps a pair such as `en-de` to that model config. See the [README](../README.md) for configuration
examples and [BUILD_AND_TEST.md](BUILD_AND_TEST.md) for the model used by the native smoke test.

## Where to find models

Mozilla publishes CPU-optimized Firefox Translations models. For current language pairs and
releases, use the [model registry dashboard](https://mozilla.github.io/translations/model-registry/)
or its [machine-readable JSON registry](https://storage.googleapis.com/moz-fx-translations-data--303e-prod-translations-data/db/models.json).
The [Firefox models dashboard](https://mozilla.github.io/translations/firefox-models/) lists
released Firefox models. Model files are hosted on Google Cloud Storage; the registry contains
the artifact paths and release metadata. Mozilla's [translations repository](https://github.com/mozilla/translations)
contains the model training pipeline.

The former [firefox-translations-models repository](https://github.com/mozilla/firefox-translations-models)
is archived and directs users to these newer sources. Its documentation describes three model
families:

- `tiny`: smallest and fastest, with lower translation quality.
- `base`: larger and slower, intended for higher translation quality.
- `base-memory`: lower memory use than `base`, with slightly lower quality.

Availability varies by language direction and family; for example, `en-de` and `de-en` are
separate entries, and not every pair has every family. The live registry generated on
2026-10-01 listed 118 directed language-pair entries. Treat the registry as authoritative for
current availability rather than relying on this dated count.

## Memory estimate

As a rough per-model baseline, the registry's **uncompressed model-weight files** for the
2026-10-01 snapshot occupy:

| Family | Variants | Weight-file size |
| --- | ---: | ---: |
| `tiny` | 67 | 12.4–24.3 MiB |
| `base-memory` | 73 | 30.1–41.9 MiB |
| `base` | 11 | 41.0–56.7 MiB |

These figures are not total RAM requirements: they exclude vocabulary and lexical-shortlist
data, runtime buffers, translation context, and other loaded models. Plan for additional memory
and measure the process under your actual model, context-window, and concurrency settings; the
registry does not publish a reliable whole-process RAM estimate.

## Direct downloads for available models

The [download catalog](MODEL_DOWNLOADS.md) lists every model variant in the registry snapshot,
grouped by the non-English language's primary geographic association. English is treated as the
shared bridge language. The catalog includes language names, uncompressed weight-file sizes, and
direct links to compressed weights, vocabulary, and lexical shortlist when published. The
registry does not provide sizes for vocabulary or shortlist artifacts, and the displayed weight
size is not the total download size or total RAM requirement. It includes nightly and
platform-specific entries as well as general releases; check the status column and the model's
terms before choosing or redistributing a model.
Seven variants in this snapshot do not list a vocabulary artifact.

To refresh the checked-in catalog from Mozilla's live registry, run from the repository root:

```sh
python3 scripts/generate-model-catalog.py > doc/MODEL_DOWNLOADS.md
```

The catalog links are direct Google Cloud Storage downloads and end in `.gz`; decompress the
artifacts before configuring Bergamot.

To download released models for both directions of a pair, using English pivot legs only when a
direct direction is unavailable, run:

```sh
./scripts/download-models.sh lang_a lang_b path/to/models
```

For example, `./scripts/download-models.sh en de ./models` downloads the available `en-de` and
`de-en` artifacts into separate subdirectories. Language codes use the registry spelling,
including underscores such as `zh_hant`. Entries whose registry status begins with `Release` are
preferred; if none exist for a direction, `Unspecified` entries with weights are eligible. Other
statuses, such as `Nightly`, are not selected. When a direct direction has no eligible model, that
direction is routed through English; for example, `fr-ja` may use `fr-en` and `en-ja`, while the
reverse uses `ja-en` and `en-fr`. Published weights, vocabulary, and lexical-shortlist artifacts
are downloaded and decompressed; absent optional artifacts are skipped, and already-present
uncompressed files are kept. The script requires `curl`, `python3`, and `gzip`.

In an interactive terminal, language choices show both code and name (for example, `ru - Russian`).
If a direction has multiple eligible families, the script asks which to use and labels each choice
as `family - status - size`, such as `base-memory - Unspecified - 31.6 MB`. If only one eligible
variant is available, it prints that variant instead of prompting. When multiple families are
available non-interactively, it selects `base-memory`, then `tiny`, then `base`. Omitted language or
destination arguments are prompted for interactively; a blank destination uses `./models`. All
three arguments are required when running non-interactively. The script does not generate Bergamot
YAML configuration, and model usage terms still apply; check each model's terms before
redistribution.

## Bergamot model YAML settings

A Bergamot model YAML is passed directly to the native engine. The checked-in smoke-test config
uses the keys below; model releases may require other options or different values, so prefer the
config supplied with a model and treat these descriptions as guidance for the pinned Bergamot
version in this repository.

| Key | Smoke-test value | Purpose and tuning notes |
| --- | --- | --- |
| `models` | `model.ende.intgemm.alphas.bin` | Ordered path(s) to the translation weight file(s). Keep the paths and order supplied for the model. |
| `vocabs` | `vocab.ende.spm` for source and target | Vocabulary paths, normally source then target. Some models use the same vocabulary file for both entries. |
| `shortlist` | `lex.50.50.ende.s2t.bin`, `false` | Optional lexical-shortlist paths for source-to-target and target-to-source directions. `false` disables a direction with no shortlist. |
| `beam-size` | `1` | Number of candidate translations explored during decoding. Larger values can improve output for some inputs but increase latency; compare on your own text. |
| `normalize` | `1.0` | Length-normalization factor used when scoring candidate translations. It affects decoding scores; keep the model's supplied value unless evaluating alternatives. |
| `word-penalty` | `0` | Per-token adjustment subtracted from a candidate's translation score. `0` applies no additional word penalty. |
| `max-length-break` | `128` | Maximum input-token length used by the engine's sentence processing and batching. Longer inputs may be split/wrapped; tune only if your workload contains unusually long sentences. |
| `mini-batch-words` | `1024` | Approximate token budget for a batch of sentences. Larger batches can improve throughput for concurrent requests, at the cost of more working memory. |
| `workspace` | `128` | Amount of inference workspace reserved, in MiB. Increasing it can help when the backend needs more workspace, but consumes more memory. |
| `max-length-factor` | `2.0` | Batching-pool factor that allows internal length buckets beyond `max-length-break`; keep the upstream/model value unless tuning long-input batching. |
| `skip-cost` | `true` | Skips model-cost calculation. The upstream engine advises against enabling it when `beam-size` is greater than 1; retain `true` only for beam size 1 configurations that use it. |
| `cpu-threads` | `0` | CPU thread setting for inference. Benchmark thread counts on the target machine rather than assuming more threads are faster. |
| `quiet` | `true` | Reduces general engine logging. Turn it off while diagnosing model-loading or runtime issues. |
| `quiet-translation` | `true` | Reduces per-translation logging. Turn it off when investigating individual translation behavior. |
| `gemm-precision` | `int8Alpha` | Selects the matrix-multiplication precision mode supported by the build. Keep the model's supplied mode unless the target build supports and has been tested with another. |
| `alignment` | `soft` | Requests/configures token-alignment behavior. The Go API currently returns translated text only, so this setting does not expose alignment data through that API. |
| `ssplit-mode` | `paragraph` | Input segmentation mode: `paragraph` (default) groups by paragraphs, `sentence` treats each line as a sentence, and `wrapped_text` handles visually wrapped text. Choose according to the input format. |

### Ready-to-use model YAML examples

Each example below is a complete Bergamot model config for the repository's pinned Mozilla EN-DE
smoke-test artifacts. First download those artifacts as described in [BUILD_AND_TEST.md](BUILD_AND_TEST.md),
then save a chosen block as `low-latency.yml`, `balanced.yml`, `quality.yml`, or `throughput.yml`
inside `third_party/bergamot-translator/models/mozilla-en-de/`. All paths are relative to that
directory, so the listed artifact files must be present there. The runnable interactive low-latency,
general-purpose balance, and quality-focused configs are also available in
[`examples/common/model-profiles/`](../examples/common/model-profiles/), with commands in
[`examples/README.md`](../examples/README.md#translation-tuning-profiles). Run a config saved beside
the artifacts with:

```sh
CGO_ENABLED=1 go run ./cmd/bergamot-go \
  -config third_party/bergamot-translator/models/mozilla-en-de/balanced.yml \
  "Hello, world."
```

These are starting points rather than universal performance guarantees. All four use the same
EN-DE weights; changing decoding options does not change the model family or language pair. For
another release or language direction, keep that release's `models`, `vocabs`, `shortlist`, and
other required vendor settings, and apply only the relevant decoding changes after testing.

#### Interactive, low-latency translation

Use this when response time matters more than exploring multiple candidate translations. For an
available `tiny` model, replace the three artifact entries with paths from that model's supplied
config; the example as written uses the repository's EN-DE smoke-test model.

```yaml
models:
  - model.ende.intgemm.alphas.bin # Translation weights; use the file and order supplied with this model.
vocabs:
  - vocab.ende.spm # Source vocabulary used to tokenize input.
  - vocab.ende.spm # Target vocabulary used to decode output; this model shares one vocabulary.
shortlist:
  - lex.50.50.ende.s2t.bin # Source-to-target lexical shortlist for this direction.
  - false # No reverse-direction shortlist is used by this one-way model config.
beam-size: 1 # Decode one candidate to minimize search work and latency.
normalize: 1.0 # Length-normalization factor; retain the model's supplied scoring value.
word-penalty: 0 # No additional per-token score penalty.
max-length-break: 128 # Sentence-token length threshold used for processing and batching.
mini-batch-words: 1024 # Maximum approximate token budget per inference batch.
workspace: 128 # Inference workspace reservation in MiB.
max-length-factor: 2.0 # Allows longer internal batching buckets beyond max-length-break.
skip-cost: true # Skips model-cost calculation; keep enabled only with beam-size 1.
cpu-threads: 0 # Use the engine's default CPU thread setting.
quiet: true # Suppress general engine logs during normal operation.
quiet-translation: true # Suppress per-translation logs during normal operation.
gemm-precision: int8Alpha # Matrix multiplication precision; must be supported by this build.
alignment: soft # Alignment behavior; the Go API currently returns translated text only.
ssplit-mode: paragraph # Segment input by paragraphs; choose sentence/wrapped_text for matching input.
```

#### General-purpose balance

This mirrors the repository smoke-test decoding values: beam size 1, moderate batching, and the
model's supplied scoring values. It is a conservative baseline to benchmark before tuning.

```yaml
models:
  - model.ende.intgemm.alphas.bin # Translation weights; use the file and order supplied with this model.
vocabs:
  - vocab.ende.spm # Source vocabulary used to tokenize input.
  - vocab.ende.spm # Target vocabulary used to decode output; this model shares one vocabulary.
shortlist:
  - lex.50.50.ende.s2t.bin # Source-to-target lexical shortlist for this direction.
  - false # No reverse-direction shortlist is used by this one-way model config.
beam-size: 1 # Decode one candidate for a balance of speed and quality.
normalize: 1.0 # Length-normalization factor; retain the model's supplied scoring value.
word-penalty: 0 # No additional per-token score penalty.
max-length-break: 128 # Sentence-token length threshold used for processing and batching.
mini-batch-words: 1024 # Maximum approximate token budget per inference batch.
workspace: 128 # Inference workspace reservation in MiB.
max-length-factor: 2.0 # Allows longer internal batching buckets beyond max-length-break.
skip-cost: true # Skips model-cost calculation; valid for this beam-size 1 setup.
cpu-threads: 0 # Use the engine's default CPU thread setting.
quiet: true # Suppress general engine logs during normal operation.
quiet-translation: true # Suppress per-translation logs during normal operation.
gemm-precision: int8Alpha # Matrix multiplication precision; must be supported by this build.
alignment: soft # Alignment behavior; the Go API currently returns translated text only.
ssplit-mode: paragraph # Segment input by paragraphs; choose sentence/wrapped_text for matching input.
```

#### Quality-focused decoding

This explores more candidates and may improve some outputs at the cost of latency. Use `base`
weights when available and evaluate on representative text; the example's artifact entries still
refer to the repository's EN-DE smoke-test model.

```yaml
models:
  - model.ende.intgemm.alphas.bin # Translation weights; use the file and order supplied with this model.
vocabs:
  - vocab.ende.spm # Source vocabulary used to tokenize input.
  - vocab.ende.spm # Target vocabulary used to decode output; this model shares one vocabulary.
shortlist:
  - lex.50.50.ende.s2t.bin # Source-to-target lexical shortlist for this direction.
  - false # No reverse-direction shortlist is used by this one-way model config.
beam-size: 4 # Explore more candidate translations; higher values increase inference time.
normalize: 1.0 # Length-normalization factor; start with the model's supplied scoring value.
word-penalty: 0 # No additional per-token score penalty; tune only through quality evaluation.
max-length-break: 128 # Sentence-token length threshold used for processing and batching.
mini-batch-words: 1024 # Maximum approximate token budget per inference batch.
workspace: 128 # Inference workspace reservation in MiB.
max-length-factor: 2.0 # Allows longer internal batching buckets beyond max-length-break.
skip-cost: false # Keep cost calculation enabled when beam-size is greater than 1.
cpu-threads: 0 # Use the engine's default CPU thread setting.
quiet: true # Suppress general engine logs during normal operation.
quiet-translation: true # Suppress per-translation logs during normal operation.
gemm-precision: int8Alpha # Matrix multiplication precision; must be supported by this build.
alignment: soft # Alignment behavior; the Go API currently returns translated text only.
ssplit-mode: paragraph # Segment input by paragraphs; choose sentence/wrapped_text for matching input.
```

#### Concurrent service throughput

This raises the token budget so a batch can include more work, which may improve throughput under
load but increases working-memory use. Benchmark it with the target hardware and concurrency; if
memory pressure rises, return to the vendor's batch setting.

```yaml
models:
  - model.ende.intgemm.alphas.bin # Translation weights; use the file and order supplied with this model.
vocabs:
  - vocab.ende.spm # Source vocabulary used to tokenize input.
  - vocab.ende.spm # Target vocabulary used to decode output; this model shares one vocabulary.
shortlist:
  - lex.50.50.ende.s2t.bin # Source-to-target lexical shortlist for this direction.
  - false # No reverse-direction shortlist is used by this one-way model config.
beam-size: 1 # Keep decoding work low so capacity is available for concurrent requests.
normalize: 1.0 # Length-normalization factor; retain the model's supplied scoring value.
word-penalty: 0 # No additional per-token score penalty.
max-length-break: 128 # Sentence-token length threshold used for processing and batching.
mini-batch-words: 2048 # Larger approximate token budget; may improve throughput and use more memory.
workspace: 128 # Inference workspace reservation in MiB.
max-length-factor: 2.0 # Allows longer internal batching buckets beyond max-length-break.
skip-cost: true # Skips model-cost calculation; keep enabled only with beam-size 1.
cpu-threads: 0 # Use the engine's default CPU thread setting; benchmark explicit counts if needed.
quiet: true # Suppress general engine logs during normal operation.
quiet-translation: true # Suppress per-translation logs during normal operation.
gemm-precision: int8Alpha # Matrix multiplication precision; must be supported by this build.
alignment: soft # Alignment behavior; the Go API currently returns translated text only.
ssplit-mode: paragraph # Segment input by paragraphs; choose sentence/wrapped_text for matching input.
```

The profiles above describe options in the Bergamot model YAML. The Go API also has a separate
translator YAML: `beam_size` can override the model's `beam-size` per language pair, while
`timeout` and `context_window` belong only to the Go wrapper. See the [README configuration
example](../README.md#go-translation-api) for those wrapper options and their behavior.

## Models used by this repository

The only specific production model release currently documented and exercised here is Mozilla's
English-to-German (`en-de`) `base-memory` model, used by `TestBridgeSmoke`. Its artifacts are:

| Artifact | Purpose |
| --- | --- |
| `model.ende.intgemm.alphas.bin` | Translation model weights |
| `vocab.ende.spm` | SentencePiece vocabulary, referenced for source and target |
| `lex.50.50.ende.s2t.bin` | Lexical shortlist |

The smoke-test config is [`pkg/testdata/bridge-smoke-mozilla-en-de.yml`](../pkg/testdata/bridge-smoke-mozilla-en-de.yml).
The pinned release artifacts are hosted under Mozilla's EN-DE export in Google Cloud Storage;
the exact download commands and a model-weight checksum are in [BUILD_AND_TEST.md](BUILD_AND_TEST.md).
The files are downloaded into the ignored
`third_party/bergamot-translator/models/mozilla-en-de/` directory and are not committed to this
repository.

`TestBridgeSmoke` can also be pointed at another compatible model config with
`BERGAMOT_TEST_MODEL_CONFIG`. Bergamot's upstream test suite has separate regression fixtures;
those are test data, not a general model catalog or an automatic model downloader.

## Using another model

Download the model artifacts from a source you trust, read its usage and license terms, and use
the model's supplied Bergamot YAML config or create one with the correct artifact paths and
settings. Model files are distinct from the Go translator config: the latter maps language-pair
names to model-config file paths. Paths to artifacts in a Bergamot config can be relative to that
config file or absolute. See [Local model artifacts in the README](../README.md#local-model-artifacts).

Model availability, quality, memory requirements, and license terms depend on the particular
release. The license for this Go wrapper or for Bergamot itself does not determine the license of
downloaded model weights; check the terms published with each model before distributing it.