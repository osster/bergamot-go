# Examples

Run these commands from the repository root after following the native build and model setup
instructions in [`../docs/BUILD_AND_TEST.md`](../docs/BUILD_AND_TEST.md). The examples use the
checked-in `en-de` smoke-test configuration where possible; the referenced native model artifacts
must still be available locally.

## Config file

`config-file` loads a translator YAML file and translates a string. Use `-count` to repeat the
translation for a sequential load test:

```sh
CGO_ENABLED=1 go run ./examples/config-file -pair en-de -text "Hello, world."
```

```sh
CGO_ENABLED=1 go run ./examples/config-file -pair en-de -text "Hello, world." -count 1000
```

Pass `-config` to use another translator configuration. The sample YAML resolves the smoke-test
model config relative to its own directory.

## Translation tuning profiles

[`common/model-profiles/`](common/model-profiles/) contains ready-to-run Bergamot model YAMLs for
interactive low latency, general-purpose balance, and quality-focused decoding. Each file annotates
its options inline and points to the same pinned EN-DE artifacts used by the smoke test, so download
and build them first as described in [`../docs/BUILD_AND_TEST.md`](../docs/BUILD_AND_TEST.md).
Run a profile from the repository root with:

```sh
CGO_ENABLED=1 go run ./cmd/bergamot-go \
  -config examples/common/model-profiles/interactive-low-latency.yml \
  "Hello, world."

CGO_ENABLED=1 go run ./cmd/bergamot-go \
  -config examples/common/model-profiles/general-purpose-balance.yml \
  "Hello, world."

CGO_ENABLED=1 go run ./cmd/bergamot-go \
  -config examples/common/model-profiles/quality-focused.yml \
  "Hello, world."
```

The low-latency profile lowers the batch budget and uses beam size 1; the quality profile uses
beam size 4 and keeps cost calculation enabled. These are decoding profiles, not separate model
families: the checked-in artifact paths point to the smoke-test EN-DE model, not a `tiny` or `base`
variant. For those families, replace the `models`, `vocabs`, and `shortlist` paths with the exact
artifacts and required settings supplied for the chosen release. Benchmark representative input
and hardware before treating any profile as a performance or quality guarantee.

## Compare model configurations

`config-comparison` translates input with two model YAML files and compares their outputs. The
profiles in [`common/model-profiles/`](common/model-profiles/) use the pinned EN-DE artifacts in
`third_party/bergamot-translator/models/mozilla-en-de/`. Download those files once from the
repository root (the same artifacts used by the smoke test):

```sh
MODEL_DIR=third_party/bergamot-translator/models/mozilla-en-de
MODEL_BASE=https://storage.googleapis.com/moz-fx-translations-data--303e-prod-translations-data/models/en-de/retrain_hr_fix_names_SCgGhxUPQ2WAECHLRtzrMg/exported
mkdir -p "$MODEL_DIR"
curl -fL "$MODEL_BASE/model.ende.intgemm.alphas.bin.gz" -o "$MODEL_DIR/model.ende.intgemm.alphas.bin.gz"
curl -fL "$MODEL_BASE/vocab.ende.spm.gz" -o "$MODEL_DIR/vocab.ende.spm.gz"
curl -fL "$MODEL_BASE/lex.50.50.ende.s2t.bin.gz" -o "$MODEL_DIR/lex.50.50.ende.s2t.bin.gz"
gunzip -f "$MODEL_DIR"/*.gz
```

Run any of these direct profile comparisons from the repository root:

```sh
CGO_ENABLED=1 go run ./examples/config-comparison \
  -config-a examples/common/model-profiles/interactive-low-latency.yml \
  -config-b examples/common/model-profiles/general-purpose-balance.yml \
  -text "A representative sentence to compare."
```

```sh
CGO_ENABLED=1 go run ./examples/config-comparison \
  -config-a examples/common/model-profiles/interactive-low-latency.yml \
  -config-b examples/common/model-profiles/quality-focused.yml \
  -text "A representative sentence to compare."
```

```sh
CGO_ENABLED=1 go run ./examples/config-comparison \
  -config-a examples/common/model-profiles/general-purpose-balance.yml \
  -config-b examples/common/model-profiles/quality-focused.yml \
  -text "A representative sentence to compare."
```

For a 1,000-message timing comparison, generate TSV input and pipe it directly into
`config-comparison` with `-stdin`:

**Important:** Keep **`-stdin`** in this command so `config-comparison` reads the generated
dataset from the pipe and compares all 1,000 messages. Without it, the example translates only
the default text.

```sh
go run ./examples/common/dataset -count 1000 -pair en-de | \
  CGO_ENABLED=1 go run ./examples/config-comparison -stdin \
    -config-a examples/common/model-profiles/interactive-low-latency.yml \
    -config-b examples/common/model-profiles/quality-focused.yml
```

Batch mode reports the number of matching translations and, for each profile, model-load time,
total translation time, average time per message, and translations per second. Translation metrics
exclude model initialization and dataset generation; runs are sequential, with each profile
translating the full dataset. This is a decoding-profile performance comparison, not a quality
evaluation or a concurrent request-load test. Artifact paths inside each YAML are resolved relative
to that YAML file, and both configurations must support the same input language. Build the native
library as described in [`../docs/BUILD_AND_TEST.md`](../docs/BUILD_AND_TEST.md).

## Config object

`config-object` builds an example-owned Go config value, serializes it to a temporary YAML file,
and passes that file to the wrapper's existing `NewTranslator` API. Use `-count` for a sequential
load test:

```sh
CGO_ENABLED=1 go run ./examples/config-object -model-config pkg/testdata/bridge-smoke-mozilla-en-de.yml
```

```sh
CGO_ENABLED=1 go run ./examples/config-object -model-config pkg/testdata/bridge-smoke-mozilla-en-de.yml -count 1000
```

This demonstrates constructing configuration data in Go without adding a config-object API or
business logic to the wrapper package.

## Document context

`context` enables a rolling source-sentence context window and translates each document in order.
It resets the history before each unrelated document so earlier text does not carry across:

```sh
CGO_ENABLED=1 go run ./examples/context -pair en-de
```

The example config sets `context_window: 2`. Context is maintained per translator and language pair;
use `ResetContext` when starting a separate document or conversation.

## Asynchronous translations

`async` submits multiple requests with `TranslateAsync` and collects each request's result channel:

```sh
CGO_ENABLED=1 go run ./examples/async -pair en-de
```

The requests share one translator and demonstrate asynchronous submission, not parallel native
inference: calls through a translator are serialized. The example also shows a pre-canceled
`context.Context`; cancellation prevents work that has not started, but cannot interrupt native
inference already in progress.

## Model, message, and worker pools

`worker-pool` creates one translator per worker, reads `language-pair<TAB>text` lines from standard
input, and sends them through a bounded message queue. Feed it the generated dataset for a pooled
load test:

```sh
printf 'en-de\tHello, world.\nen-de\tGood morning.\n' | \
  CGO_ENABLED=1 go run ./examples/worker-pool -workers 4 -queue 32
```

Generate a deterministic 1,000-message `en-de` dataset and send it through the model, message, and
worker pools:

```sh
go run ./examples/common/dataset -count 1000 | \
  CGO_ENABLED=1 go run ./examples/worker-pool -workers 4 -queue 32
```

The dataset generator cycles through 50 varied sample messages to reach the requested count; use
`-count` and `-pair` to change the dataset size or language pair.

The sample translator YAML includes `en-de` and a placeholder `fr-en` model path. Replace that
placeholder with a real local model config before sending `fr-en` requests. Models are loaded
lazily when first requested and shared by the library's model pool when config path and beam size
match. Shared native inference is serialized per model, so extra workers mainly help with queuing
and can run independent models concurrently; they do not multiply throughput for one shared model.