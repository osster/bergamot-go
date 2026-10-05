#!/usr/bin/env bash
set -euo pipefail

REGISTRY_URL="${BERGAMOT_MODEL_REGISTRY_URL:-https://storage.googleapis.com/moz-fx-translations-data--303e-prod-translations-data/db/models.json}"

usage() {
	cat <<'EOF'
Usage: ./scripts/download-models.sh [lang_a [lang_b [path_to_dir]]]

Downloads Mozilla Bergamot model artifacts in both directions between lang_a
and lang_b. If an eligible direct model is unavailable for a direction, uses
an English pivot for that direction. Release models are preferred; Unspecified
models are used only when no Release model exists. Omitted arguments are
prompted for in an interactive terminal. Language codes use Mozilla registry
format (for example: en, de, zh_hant).
Artifacts are unpacked into path_to_dir/<language-pair>/.
EOF
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
	usage
	exit 0
fi

if [[ "$#" -gt 3 ]]; then
	usage >&2
	exit 2
fi

lang_a="${1:-}"
lang_b="${2:-}"
destination="${3:-}"

if [[ ( -z "$lang_a" || -z "$lang_b" || -z "$destination" ) && ( ! -t 0 || ! -t 1 ) ]]; then
	printf 'error: missing arguments require an interactive terminal\n' >&2
	usage >&2
	exit 2
fi

for command in curl python3 gzip; do
	if ! command -v "$command" >/dev/null 2>&1; then
		printf 'error: required command not found: %s\n' "$command" >&2
		exit 1
	fi
done

registry_file="$(mktemp)"
compressed_file=""
unpacked_file=""
cleanup() {
	rm -f "$registry_file"
	if [[ -n "$compressed_file" ]]; then
		rm -f "$compressed_file"
	fi
	if [[ -n "$unpacked_file" ]]; then
		rm -f "$unpacked_file"
	fi
}
trap cleanup EXIT

printf 'Fetching Mozilla model registry...\n'
curl -fLsS --retry 2 "$REGISTRY_URL" -o "$registry_file"

if [[ -z "$lang_a" || -z "$lang_b" ]]; then
	available_languages=()
	language_options=()
	while IFS= read -r language; do
		IFS=$'\t' read -r language language_name <<< "$language"
		available_languages+=("$language")
		language_options+=("$language - $language_name")
	done < <(python3 - "$registry_file" <<'PY'
import json
import sys

language_names = {
	"af": "Afrikaans", "ar": "Arabic", "az": "Azerbaijani",
	"be": "Belarusian", "bg": "Bulgarian", "bn": "Bengali",
	"bs": "Bosnian", "ca": "Catalan", "cs": "Czech",
	"da": "Danish", "de": "German", "el": "Greek", "en": "English",
	"es": "Spanish", "et": "Estonian", "eu": "Basque", "fa": "Persian",
	"fi": "Finnish", "fr": "French", "gl": "Galician", "gu": "Gujarati",
	"hbs": "Bosnian/Croatian/Serbian", "he": "Hebrew", "hi": "Hindi",
	"hr": "Croatian", "hu": "Hungarian", "id": "Indonesian",
	"is": "Icelandic", "it": "Italian", "ja": "Japanese", "kn": "Kannada",
	"ko": "Korean", "lt": "Lithuanian", "lv": "Latvian", "ml": "Malayalam",
	"mr": "Marathi", "ms": "Malay", "nb": "Norwegian Bokmål", "nl": "Dutch",
	"nn": "Norwegian Nynorsk", "no": "Norwegian", "pl": "Polish",
	"pt": "Portuguese", "ro": "Romanian", "ru": "Russian", "sk": "Slovak",
	"sl": "Slovenian", "sq": "Albanian", "sr": "Serbian", "sv": "Swedish",
	"ta": "Tamil", "te": "Telugu", "th": "Thai", "tl": "Tagalog",
	"tr": "Turkish", "ug": "Uyghur", "uk": "Ukrainian", "ur": "Urdu",
	"vi": "Vietnamese", "zh": "Chinese", "zh_hant": "Traditional Chinese",
}

with open(sys.argv[1], encoding="utf-8") as registry_file:
	registry = json.load(registry_file)

languages = {
	language
	for pair in registry.get("models", {})
	for language in pair.split("-")
}
for language in sorted(languages, key=lambda code: (language_names.get(code, code).casefold(), code)):
	print(f"{language}\t{language_names.get(language, language)}")
PY
)
	if [[ "${#available_languages[@]}" -eq 0 ]]; then
		printf 'error: the model registry contains no available languages\n' >&2
		exit 1
	fi
	select_language() {
		local prompt="$1"
		local language
		PS3="$prompt"
		select language in "${language_options[@]}"; do
			if [[ -n "$language" && "$REPLY" -ge 1 && "$REPLY" -le "${#available_languages[@]}" ]]; then
				selected_language="${available_languages[$((REPLY - 1))]}"
				return 0
			fi
			printf 'Please choose a number from the list.\n' >&2
		done
		return 1
	}
	if [[ -z "$lang_a" ]]; then
		if ! select_language 'Select source language: '; then
			printf 'error: no source language selected\n' >&2
			exit 2
		fi
		lang_a="$selected_language"
	fi
	if [[ -z "$lang_b" ]]; then
		if ! select_language 'Select target language: '; then
			printf 'error: no target language selected\n' >&2
			exit 2
		fi
		lang_b="$selected_language"
	fi
fi

if [[ -z "$destination" ]]; then
	printf 'Destination directory [./models]: ' >&2
	IFS= read -r destination || {
		printf 'error: no destination directory provided\n' >&2
		exit 2
	}
	destination="${destination:-./models}"
fi

for language in "$lang_a" "$lang_b"; do
	if [[ ! "$language" =~ ^[a-z][a-z0-9_]*$ ]]; then
		printf 'error: invalid language code: %s\n' "$language" >&2
		exit 2
	fi
done

mkdir -p "$destination"

model_data() {
	python3 - "$registry_file" "$1" "${2:-list}" "${3:-0}" <<'PY'
import json
import sys
from pathlib import PurePosixPath
from urllib.parse import urljoin

with open(sys.argv[1], encoding="utf-8") as registry_file:
	registry = json.load(registry_file)

pair, action, selected_index = sys.argv[2:]
family_order = {"base-memory": 0, "tiny": 1, "base": 2}

def has_weights(model):
	return bool((model.get("files", {}).get("model") or {}).get("path"))

models_for_pair = registry.get("models", {}).get(pair, [])
released = [
	model for model in models_for_pair
	if (model.get("releaseStatus") or "").startswith("Release")
	and has_weights(model)
]
if released:
	eligible = released
else:
	eligible = [
		model for model in models_for_pair
		if (
			not model.get("releaseStatus")
			or model.get("releaseStatus", "").startswith("Unspecified")
		)
		and has_weights(model)
	]
eligible.sort(key=lambda model: family_order.get(model.get("architecture"), 3))

# Show one eligible variant per family; registry order breaks ties within a family.
families = {}
for model in eligible:
	families.setdefault(model.get("architecture") or "unknown", model)
models = list(families.values())
if not models:
	sys.exit(1)

def format_size(size_bytes):
	if size_bytes is None:
		return "size unavailable"
	if size_bytes >= 1_000_000_000:
		return f"{size_bytes / 1_000_000_000:.1f} GB"
	return f"{size_bytes / 1_000_000:.1f} MB"

if action == "list":
	for index, model in enumerate(models, 1):
		weight = model["files"]["model"]
		print("\t".join((
			str(index), model.get("architecture") or "unknown",
			format_size(weight.get("uncompressedSize")),
			model.get("releaseStatus") or "Unspecified",
		)))
	sys.exit(0)

model = models[int(selected_index) - 1]
family = model.get("architecture") or "unknown"
weight = model["files"]["model"]
print(f"MODEL\t{family}\t{format_size(weight.get('uncompressedSize'))}")
base_url = registry["baseUrl"].rstrip("/") + "/"
for key, label in (
	("model", "weights"),
	("vocab", "vocabulary"),
	("lexicalShortlist", "shortlist"),
):
	artifact = model.get("files", {}).get(key)
	if artifact and artifact.get("path"):
		path = artifact["path"]
		filename = PurePosixPath(path).name
		url = urljoin(base_url, path.lstrip("/"))
		print(f"{label}\t{filename}\t{url}")
PY
}

select_model() {
	local pair="$1"
	local candidates line index family size status choice selected_index
	local -a candidate_indexes=()
	local -a candidate_labels=()
	candidates="$(model_data "$pair")" || return 1
	while IFS=$'\t' read -r index family size status; do
		[[ -n "$index" ]] || continue
		candidate_indexes+=("$index")
		candidate_labels+=("$family - $status - $size")
	done <<< "$candidates"
	selected_index="${candidate_indexes[0]}"
	if [[ "${#candidate_indexes[@]}" -gt 1 && -t 0 ]]; then
		printf '%s: choose a model family:\n' "$pair" >&2
		PS3='Select family: '
		select choice in "${candidate_labels[@]}"; do
			if [[ -n "$choice" && "$REPLY" -ge 1 && "$REPLY" -le "${#candidate_indexes[@]}" ]]; then
				selected_index="${candidate_indexes[$((REPLY - 1))]}"
				break
			fi
			printf 'Please choose a number from the list.\n' >&2
		done
	elif [[ "${#candidate_indexes[@]}" -eq 1 ]]; then
		printf '%s: only one model variant available: %s\n' \
			"$pair" "${candidate_labels[0]}" >&2
	fi
	model_data "$pair" get "$selected_index"
}

download_pair() {
	local pair="$1"
	local selection="$2"
	local pair_dir="$destination/$pair"
	local kind filename url output
	mkdir -p "$pair_dir"
	while IFS=$'\t' read -r kind filename url; do
		[[ -n "$kind" ]] || continue
		if [[ "$kind" == "MODEL" ]]; then
			printf '%s: selected %s model (%s)\n' "$pair" "$filename" "$url"
			continue
		fi
		output="$pair_dir/${filename%.gz}"
		if [[ -f "$output" ]]; then
			printf 'Already present: %s\n' "$output"
			continue
		fi
		compressed_file="$(mktemp "$pair_dir/.download.XXXXXX")"
		unpacked_file="$(mktemp "$pair_dir/.unpacked.XXXXXX")"
		printf 'Downloading %s: %s\n' "$pair" "$filename"
		curl -fLsS --retry 2 "$url" -o "$compressed_file"
		gzip -dc "$compressed_file" > "$unpacked_file"
		mv "$unpacked_file" "$output"
		rm -f "$compressed_file"
		compressed_file=""
		unpacked_file=""
	done <<< "$selection"
}

selected_pairs=()
selected_models=()
add_model_pair() {
	local pair="$1"
	local model="$2"
	local existing
	for existing in "${!selected_pairs[@]}"; do
		if [[ "${selected_pairs[$existing]}" == "$pair" ]]; then
			return 0
		fi
	done
	selected_pairs+=("$pair")
	selected_models+=("$model")
}

resolve_direction() {
	local source="$1"
	local target="$2"
	local direct_pair="$source-$target"
	local selection first_pair second_pair
	if selection="$(select_model "$direct_pair")"; then
		add_model_pair "$direct_pair" "$selection"
		return 0
	fi
	if [[ "$source" == "en" || "$target" == "en" ]]; then
		printf 'error: no eligible model for %s (Release or Unspecified), and an English pivot cannot add a distinct leg\n' "$direct_pair" >&2
		return 1
	fi
	first_pair="$source-en"
	second_pair="en-$target"
	if ! selection="$(select_model "$first_pair")"; then
		printf 'error: no eligible direct model for %s and no eligible pivot model for %s\n' "$direct_pair" "$first_pair" >&2
		return 1
	fi
	add_model_pair "$first_pair" "$selection"
	if ! selection="$(select_model "$second_pair")"; then
		printf 'error: no eligible direct model for %s and no eligible pivot model for %s\n' "$direct_pair" "$second_pair" >&2
		return 1
	fi
	add_model_pair "$second_pair" "$selection"
	printf 'No released direct model for %s; using English pivot (%s, %s).\n' "$direct_pair" "$first_pair" "$second_pair"
}

resolve_direction "$lang_a" "$lang_b"
resolve_direction "$lang_b" "$lang_a"
for index in "${!selected_pairs[@]}"; do
	download_pair "${selected_pairs[$index]}" "${selected_models[$index]}"
done

printf 'Model artifacts are in %s\n' "$destination"
printf 'Use the downloaded files in a Bergamot model YAML config; this script does not create that config.\n'